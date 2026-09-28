package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ── Narasi AI: seluruh kalimat halaman ditulis ulang dari angka ──────────────
//
// Kalimat templat selalu benar tetapi berbunyi sama dari outlet ke outlet.
// Lapisan ini meminta sebuah model bahasa menulis ulang SEMUA teks halaman —
// judul, vonis, temuan, judul bagian, catatan & saran tiap outlet, pengamatan
// model, pembacaan medsos — dari data angka yang sama, dengan aturan keras:
// tidak boleh ada angka, tanggal, atau nama yang tidak ada di data, dan vonis
// tidak boleh berubah. Keluarannya dipaksa JSON berskema.
//
// Penyedia model (NARRATIVE_PROVIDER):
//   - ollama    : model lokal di server ini, gratis, tanpa kunci, data tidak
//                 keluar (bawaan; model bawaan gemma3:4b, ±3,3 GB RAM saat
//                 dipakai, dilepas lagi sesudahnya).
//   - openai    : titik akhir kompatibel OpenAI — Groq, Gemini, OpenRouter,
//                 LM Studio — untuk tingkat gratis berbatas atau model lebih besar.
//   - anthropic : Claude lewat SDK resmi (berbayar).
//
// Karena model lokal lambat dan jendela konteksnya kecil, narasi dibuat per
// BAGIAN: satu panggilan untuk pasar (judul, vonis, temuan, judul bagian),
// satu panggilan per outlet, satu untuk medsos. Prompt tiap panggilan pendek,
// dan kegagalan satu outlet tidak menggagalkan yang lain.
//
// Cara jalannya:
//   - kunci hash = sha256(angka laporan + penyedia/model + versi prompt). Data
//     yang sama → narasi yang sama, disimpan di business_narratives.
//   - halaman TIDAK menunggu: teks templat dikirim dengan status "generating",
//     pembuatan berjalan di latar, UI memuat ulang berkala.
//   - penjadwal memanaskan rentang bawaan tiap pagi & sesaat setelah boot.

const (
	narrativePromptVersion = "v7-bagian"
	// Batas keluaran per panggilan. Model penalar menghitung token pikirannya
	// di sini juga, jadi batasnya diberi ruang di atas panjang teks jadinya.
	narrativeTokSummary  = 4000
	narrativeTokSections = 4000
	narrativeTokOutlet   = 3500
	narrativeTokSocial   = 3000
	narrativeSectionsPer = 5
)

var (
	narrativeProv    narrativeProvider
	narrativeLabel   string // "ollama/gemma3:4b"
	narrativeMu      sync.Mutex
	narrativeRunning = map[string]bool{}
	narrativeHTTP    = &http.Client{Timeout: 15 * time.Minute}
)

// narrativeProvider = satu cara meminta JSON berskema dari sebuah model.
type narrativeProvider interface {
	Generate(ctx context.Context, system, user string, schema map[string]any, maxTokens int, effort string) (string, error)
	Name() string
}

func InitNarrativeAI(cfg *config.Config) {
	prov := strings.ToLower(strings.TrimSpace(cfg.NarrativeProvider))
	model := strings.TrimSpace(cfg.NarrativeModel)
	base := strings.TrimRight(strings.TrimSpace(cfg.NarrativeBaseURL), "/")
	key := strings.TrimSpace(cfg.NarrativeAPIKey)

	switch prov {
	case "ollama":
		if model == "" {
			model = "gemma3:4b"
		}
		if base == "" {
			base = "http://ollama:11434"
		}
		p := &ollamaProvider{base: base, model: model}
		narrativeProv = p
		go p.ensureModel()
	case "openai", "openai-compatible", "groq", "gemini", "openrouter":
		if base == "" || model == "" {
			log.Printf("[Narasi AI] NARRATIVE_BASE_URL/NARRATIVE_MODEL kosong — narasi AI nonaktif")
			return
		}
		narrativeProv = &openaiProvider{base: base, model: model, key: key, remaining: -1}
	case "anthropic", "claude":
		if key == "" {
			log.Printf("[Narasi AI] NARRATIVE_API_KEY kosong untuk anthropic — narasi AI nonaktif")
			return
		}
		if model == "" {
			model = "claude-opus-5"
		}
		c := anthropic.NewClient(option.WithAPIKey(key), option.WithRequestTimeout(4*time.Minute), option.WithMaxRetries(2))
		narrativeProv = &anthropicProvider{client: &c, model: model}
	default:
		log.Printf("[Narasi AI] NARRATIVE_PROVIDER kosong/tidak dikenal (%q) — halaman memakai kalimat templat", prov)
		return
	}
	narrativeLabel = narrativeProv.Name()
	log.Printf("[Narasi AI] aktif: %s", narrativeLabel)
}

// NarrativeAIEnabled dipakai penjadwal & status halaman.
func NarrativeAIEnabled() bool { return narrativeProv != nil }

// ── Penyedia: Ollama (lokal) ─────────────────────────────────────────────────

type ollamaProvider struct {
	base, model string
	ready       bool
}

func (p *ollamaProvider) Name() string { return "ollama/" + p.model }

// ensureModel mengunduh model bila belum ada, supaya tidak ada langkah manual.
func (p *ollamaProvider) ensureModel() {
	for attempt := 0; attempt < 30; attempt++ {
		resp, err := http.Get(p.base + "/api/tags")
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		json.NewDecoder(resp.Body).Decode(&tags)
		resp.Body.Close()
		for _, m := range tags.Models {
			if m.Name == p.model || strings.HasPrefix(m.Name, p.model+":") {
				p.ready = true
				log.Printf("[Narasi AI] model %s siap di %s", p.model, p.base)
				return
			}
		}
		log.Printf("[Narasi AI] mengunduh model %s (sekali saja, bisa beberapa menit)…", p.model)
		body, _ := json.Marshal(map[string]any{"model": p.model, "stream": false})
		req, _ := http.NewRequest(http.MethodPost, p.base+"/api/pull", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		pr, err := (&http.Client{Timeout: 2 * time.Hour}).Do(req)
		if err != nil {
			log.Printf("[Narasi AI] unduh model gagal: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}
		io.Copy(io.Discard, pr.Body)
		pr.Body.Close()
		if pr.StatusCode == http.StatusOK {
			p.ready = true
			log.Printf("[Narasi AI] model %s selesai diunduh", p.model)
			return
		}
		time.Sleep(30 * time.Second)
	}
}

func (p *ollamaProvider) Generate(ctx context.Context, system, user string, schema map[string]any, maxTokens int, _ string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":  p.model,
		"stream": false,
		"format": schema,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"options": map[string]any{
			"temperature": 0.4,
			"num_ctx":     8192,
			"num_predict": maxTokens,
		},
		"keep_alive": "3m",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := narrativeHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		PromptEval int `json:"prompt_eval_count"`
		Eval       int `json:"eval_count"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.Message.Content, nil
}

// ── Penyedia: titik akhir kompatibel OpenAI (Groq, Gemini, OpenRouter, …) ───
//
// Tingkat gratis dibatasi per menit (Groq: 8.000 token/menit, 1.000 permintaan
// per hari untuk gpt-oss-120b). Semua panggilan karena itu dijalankan satu per
// satu lewat satu kunci, dan sebelum tiap panggilan sisa jatah token dari
// header jawaban sebelumnya dibandingkan dengan perkiraan kebutuhan; kalau
// kurang, ditunggu sampai jatahnya pulih. Jawaban 429 ditunggu sesuai
// retry-after; jeda lebih dari sepuluh menit berarti jatah harian habis dan
// pembuatan dihentikan.

type openaiProvider struct {
	base, model, key string

	mu        sync.Mutex
	remaining int       // sisa token menurut header terakhir (-1 = belum tahu)
	resetAt   time.Time // kapan jatah token pulih
}

func (p *openaiProvider) Name() string { return "openai-compatible/" + p.model }

// errQuotaExhausted = jatah harian habis; tidak ada gunanya mencoba lagi hari ini.
type errQuotaExhausted struct{ wait time.Duration }

func (e errQuotaExhausted) Error() string {
	return fmt.Sprintf("jatah penyedia habis, pulih dalam %s", e.wait.Round(time.Second))
}

func aiParseWait(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	var secs float64
	if _, err := fmt.Sscanf(v, "%g", &secs); err == nil {
		return time.Duration(secs * float64(time.Second))
	}
	return 0
}

func (p *openaiProvider) Generate(ctx context.Context, system, user string, schema map[string]any, maxTokens int, effort string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	need := (len(system)+len(user))/3 + maxTokens
	if p.remaining >= 0 && p.remaining < need && time.Now().Before(p.resetAt) {
		wait := time.Until(p.resetAt) + 500*time.Millisecond
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	call := func(format map[string]any, userText string) (string, int, http.Header, error) {
		payload := map[string]any{
			"model":       p.model,
			"temperature": 0.4,
			"max_tokens":  maxTokens,
			"messages": []map[string]string{
				{"role": "system", "content": system},
				{"role": "user", "content": userText},
			},
		}
		// Model penalar (gpt-oss) menghitung token pikirannya ke jatah per
		// menit. Penalaran "low" terbukti ceroboh dengan angka (28 Sep 2026:
		// arah minggu aneh terbalik, pertumbuhan dibandingkan dengan batas
		// wajar), jadi dipakai "medium".
		if strings.Contains(p.model, "gpt-oss") {
			if effort == "" {
				effort = "medium"
			}
			payload["reasoning_effort"] = effort
		}
		if format != nil {
			payload["response_format"] = format
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", 0, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if p.key != "" {
			req.Header.Set("Authorization", "Bearer "+p.key)
		}
		resp, err := narrativeHTTP.Do(req)
		if err != nil {
			return "", 0, nil, err
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return "", resp.StatusCode, resp.Header, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			return "", resp.StatusCode, resp.Header, err
		}
		if len(out.Choices) == 0 {
			return "", resp.StatusCode, resp.Header, fmt.Errorf("jawaban tanpa pilihan")
		}
		if out.Choices[0].FinishReason == "length" {
			return "", resp.StatusCode, resp.Header, fmt.Errorf("jawaban terpotong (max_tokens %d)", maxTokens)
		}
		return out.Choices[0].Message.Content, resp.StatusCode, resp.Header, nil
	}

	strict := map[string]any{"type": "json_schema",
		"json_schema": map[string]any{"name": "narasi", "schema": schema, "strict": true}}
	format, userText := strict, user
	for attempt := 0; attempt < 6; attempt++ {
		text, code, hdr, err := call(format, userText)
		if hdr != nil {
			if v := hdr.Get("x-ratelimit-remaining-tokens"); v != "" {
				fmt.Sscanf(v, "%d", &p.remaining)
				p.resetAt = time.Now().Add(aiParseWait(hdr.Get("x-ratelimit-reset-tokens")))
			}
		}
		switch {
		case err == nil:
			return text, nil
		case code == http.StatusTooManyRequests:
			wait := aiParseWait(hdr.Get("retry-after"))
			if wait <= 0 {
				wait = 20 * time.Second
			}
			if wait > 10*time.Minute {
				return "", errQuotaExhausted{wait: wait}
			}
			select {
			case <-time.After(wait + time.Second):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		case code == http.StatusBadRequest && format["type"] == "json_schema":
			// Penyedia tanpa skema ketat: mode JSON biasa, skemanya di prompt.
			sch, _ := json.Marshal(schema)
			format = map[string]any{"type": "json_object"}
			userText = user + "\n\nBalas dengan satu objek JSON yang mematuhi JSON Schema ini:\n" + string(sch)
		default:
			return "", err
		}
	}
	return "", fmt.Errorf("gagal setelah beberapa percobaan (batas laju penyedia)")
}

// ── Penyedia: Anthropic (Claude) ─────────────────────────────────────────────

type anthropicProvider struct {
	client *anthropic.Client
	model  string
}

func (p *anthropicProvider) Name() string { return "anthropic/" + p.model }

func (p *anthropicProvider) Generate(ctx context.Context, system, user string, schema map[string]any, maxTokens int, _ string) (string, error) {
	resp, err := p.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: int64(maxTokens) * 4,
		System:    []anthropic.BetaTextBlockParam{{Text: system}},
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(user))},
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortMedium,
			Format: anthropic.BetaJSONOutputFormatParam{Schema: schema},
		},
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
	})
	if err != nil {
		return "", err
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return "", fmt.Errorf("ditolak model: %s", resp.StopDetails.Explanation)
	}
	var text string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.BetaTextBlock); ok {
			text += tb.Text
		}
	}
	return text, nil
}

// ── Bentuk keluaran ──────────────────────────────────────────────────────────

type aiNarrative struct {
	Headline    string `json:"headline"`
	VerdictText string `json:"verdict_text"`
	Insights    []struct {
		Index int    `json:"index"`
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"insights"`
	Sections []struct {
		Key  string `json:"key"`
		Lead string `json:"lead"`
		Hint string `json:"hint"`
	} `json:"sections"`
	Outlets []struct {
		Code       string   `json:"code"`
		Note       string   `json:"note"`
		Advice     string   `json:"advice"`
		Breakdown  string   `json:"breakdown"`
		ModelNotes []string `json:"model_notes"`
	} `json:"outlets"`
	SocialOutlets []struct {
		Code    string `json:"code"`
		Reading string `json:"reading"`
	} `json:"social_outlets"`
	// Rejected = kolom yang dikembalikan ke templat oleh pemeriksa fakta.
	Rejected []string `json:"rejected,omitempty"`
}

func aiStr() map[string]any { return map[string]any{"type": "string"} }
func aiObj(props map[string]any, req ...string) map[string]any {
	return map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": req}
}
func aiArr(item map[string]any) map[string]any { return map[string]any{"type": "array", "items": item} }

func aiSchemaMarket() map[string]any {
	return aiObj(map[string]any{
		"headline":     aiStr(),
		"verdict_text": aiStr(),
		"insights":     aiArr(aiObj(map[string]any{"index": map[string]any{"type": "integer"}, "title": aiStr(), "body": aiStr()}, "index", "title", "body")),
		"sections":     aiArr(aiObj(map[string]any{"key": aiStr(), "lead": aiStr(), "hint": aiStr()}, "key", "lead", "hint")),
	}, "headline", "verdict_text", "insights", "sections")
}
func aiSchemaOutlet() map[string]any {
	return aiObj(map[string]any{"code": aiStr(), "note": aiStr(), "advice": aiStr(), "breakdown": aiStr(), "model_notes": aiArr(aiStr())},
		"code", "note", "advice", "breakdown", "model_notes")
}
func aiSchemaSocial() map[string]any {
	return aiObj(map[string]any{"readings": aiArr(aiObj(map[string]any{"code": aiStr(), "reading": aiStr()}, "code", "reading"))}, "readings")
}

// aiNarrativeSchema = gabungan (dipakai tes & dokumentasi).
func aiNarrativeSchema() map[string]any {
	return aiObj(map[string]any{
		"headline": aiStr(), "verdict_text": aiStr(),
		"insights":       aiSchemaMarket()["properties"].(map[string]any)["insights"],
		"sections":       aiSchemaMarket()["properties"].(map[string]any)["sections"],
		"outlets":        aiArr(aiSchemaOutlet()),
		"social_outlets": aiArr(aiObj(map[string]any{"code": aiStr(), "reading": aiStr()}, "code", "reading")),
	}, "headline", "verdict_text", "insights", "sections", "outlets", "social_outlets")
}

const aiSystemPrompt = `Anda penulis laporan bisnis untuk grup kafe dan tempat wisata keluarga di Sumatera Utara. Anda menulis ulang teks halaman "Analisa Bisnis" dalam bahasa Indonesia yang jernih untuk manajer outlet dan tim Markom yang bukan analis.

Bahan Anda hanya dua: FAKTA (pernyataan yang sudah dihitung dan ditafsirkan sistem) dan DRAF (kalimat templat yang benar tetapi kaku).

Aturan keras:
1. Setiap klaim harus berasal dari FAKTA atau DRAF. Jangan menghitung sendiri, jangan mengurangkan atau membandingkan dua angka yang tidak dibandingkan di FAKTA, jangan menyimpulkan lamanya penurunan atau urutan minggu yang tidak tertulis.
2. Setiap angka yang Anda tulis harus ada di FAKTA atau DRAF. Boleh dibulatkan (Rp 314.915.646 menjadi "sekitar Rp 315 juta", 43,3% menjadi 43%). Jangan menulis nomor minggu seperti W34 kecuali tertulis.
3. Kata arah harus sama dengan FAKTA: naik/turun, di atas/di bawah, di dalam/melewati batas, sudah/belum konsisten, nyata/belum nyata.
4. Label kesimpulan (Tertinggal, Lebih Baik, Perlu Dipantau, Sama Saja, Belum Bisa Dinilai) hanya untuk outlet yang memang berlabel itu.
5. "Batas wajar" adalah ambang untuk SELISIH, bukan angka pertumbuhan. Jangan membandingkan pertumbuhan dengan batas wajar.
6. Sebab yang tidak ada di FAKTA ditulis sebagai dugaan yang perlu dicek ("pantas dicek apakah ..."), bukan kesimpulan.
7. Istilah: patokan adalah NILAI TENGAH outlet pembanding (jangan sebut rata-rata). Konsistensi berarti searah pada sekian dari minggu-minggu blok, BUKAN berturut-turut. Perkiraan penjualan dan batas wajar tidak berhubungan; jangan dikaitkan. Pernyataan "tidak ada libur pada minggu-minggu itu" adalah fakta, bukan syarat — jangan diubah menjadi "asalkan tidak ada libur". Jangan menyebut "semua outlet turun" bila FAKTA menyebut ada outlet yang naik.
8. Tulis lebih hidup, lebih spesifik, dan lebih membantu memutuskan daripada DRAF, tanpa menambah fakta. Kalimat pendek. Tanpa markdown, tanpa daftar bernomor, tanpa emoji. Nama bulan dalam bahasa Indonesia: Jan, Feb, Mar, Apr, Mei, Jun, Jul, Agu, Sep, Okt, Nov, Des.
9. Balas HANYA dengan satu objek JSON sesuai skema.`

// ── Lembar fakta ─────────────────────────────────────────────────────────────
//
// Model bahasa kecil-menengah pandai menulis tetapi ceroboh menafsirkan angka
// mentah: pada percobaan pertama ia membandingkan pertumbuhan dengan batas
// wajar, mengurangkan dua angka yang tidak sebanding, dan menebak arah minggu
// aneh. Karena itu semua tafsiran dikerjakan di sini, oleh kode, dan model
// hanya menerima kalimat fakta yang sudah benar. Tugasnya menulis, bukan
// menghitung.

func aiPctWord(v *float64) string {
	if v == nil {
		return "belum ada"
	}
	return bizNaikTurun(*v)
}

func aiFactsOutlet(rep *models.BusinessAnalysis, o *models.BizOutlet) []string {
	k := rep.BlockWeeks
	f := []string{fmt.Sprintf("Outlet %s (%s). Kesimpulan halaman: %s. Yang bertindak: %s.", o.Name, o.Code, o.DiagnosisLabel, o.Owner),
		"ALASAN KESIMPULAN (dari sistem, wajib diikuti): " + bizAlasanVonis(o, k)}
	if o.Growth4 != nil {
		f = append(f, fmt.Sprintf("Penjualan %d minggu terakhir lawan %d minggu sebelumnya, setara kalender: %s (angka mentah: %s).",
			k, k, aiPctWord(o.Growth4), aiPctWord(o.GrowthRaw4)))
		f = append(f, fmt.Sprintf("Penjualan sebenarnya %d minggu sebelumnya %s, %d minggu terakhir %s.", k, bizRupiah(o.PrevNet), k, bizRupiah(o.RecentNet)))
	}
	if o.PeerGrowth4 != nil {
		f = append(f, fmt.Sprintf("Patokan (nilai tengah %d outlet pembanding) pada periode yang sama: %s.", o.PeerCount, aiPctWord(o.PeerGrowth4)))
	}
	if o.RGI4 != nil && o.Threshold != nil {
		posisi := "MASIH DI DALAM batas wajar"
		if math.Abs(*o.RGI4) >= *o.Threshold {
			posisi = "MELEWATI batas wajar"
		}
		arah := "lebih cepat"
		if *o.RGI4 < 0 {
			arah = "lebih lambat"
		}
		f = append(f, fmt.Sprintf("Selisih outlet ini dengan patokan: %s poin (%s daripada patokan). Batas wajar selisih outlet ini: ±%s poin. Selisih itu %s.",
			bizNum(*o.RGI4), arah, bizNum(*o.Threshold), posisi))
	}
	if o.ConsistencyNeed > 0 {
		st := "BELUM konsisten"
		if o.Consistency >= o.ConsistencyNeed {
			st = "SUDAH konsisten"
		}
		f = append(f, fmt.Sprintf("Arah selisih mingguan searah pada %d dari %d minggu terakhir; syarat vonis %d minggu; jadi %s.", o.Consistency, k, o.ConsistencyNeed, st))
	}
	if o.ExpectedNet != nil && o.GapNet != nil {
		kata := "kurang"
		if *o.GapNet >= 0 {
			kata = "lebih"
		}
		f = append(f, fmt.Sprintf("Seandainya bergerak seperti patokan, penjualan %d minggu terakhir sekitar %s; kenyataannya %s, jadi %s %s.",
			k, bizRupiah(*o.ExpectedNet), bizRupiah(o.RecentNet), kata, bizRupiah(math.Abs(*o.GapNet))))
	}
	if o.PrevTrx > 0 && o.RecentTrx > 0 {
		trxPct := round1(float64(o.RecentTrx-o.PrevTrx) / float64(o.PrevTrx) * 100)
		prevATV, recATV := o.PrevNet/float64(o.PrevTrx), o.RecentNet/float64(o.RecentTrx)
		atvPct := round1((recATV - prevATV) / prevATV * 100)
		f = append(f, fmt.Sprintf("Jumlah struk %d lalu %d (%s). Rata-rata belanja per struk %s lalu %s (%s).",
			o.PrevTrx, o.RecentTrx, bizNaikTurun(trxPct), bizRupiah(prevATV), bizRupiah(recATV), bizNaikTurun(atvPct)))
	}
	if rep.Model != nil {
		if r, ok := rep.Model.Outlets[o.Code]; ok {
			f = append(f, aiFactsModel(r, rep.BlockWeeks)...)
		}
	}
	return f
}

// bizAlasanVonis = sebab kesimpulan satu outlet dalam satu kalimat, supaya
// penulis ulang tidak menebak. Sebab tiap label berbeda: "Belum Bisa Dinilai"
// lahir dari derau yang terlalu lebar, bukan dari konsistensi.
func bizAlasanVonis(o *models.BizOutlet, k int) string {
	thr := deref(o.Threshold)
	abs := math.Abs(deref(o.RGI4))
	switch o.Diagnosis {
	case models.BizDiagNoData:
		return "riwayat outlet ini belum cukup untuk dibandingkan dengan outlet lain."
	case models.BizDiagWeak:
		return fmt.Sprintf("penjualan mingguan outlet ini terlalu naik-turun; batas wajarnya ±%s poin jauh lebih lebar daripada outlet pada umumnya, jadi selisihnya belum bisa dinilai secara mingguan. Ini BUKAN soal konsistensi.", bizNum(thr))
	case models.BizDiagLagging, models.BizDiagLeading:
		return fmt.Sprintf("selisih %s poin melewati batas wajar ±%s dan searah pada %d dari %d minggu (syarat %d).", bizNum(deref(o.RGI4)), bizNum(thr), o.Consistency, k, o.ConsistencyNeed)
	case models.BizDiagWatch:
		if abs >= thr {
			return fmt.Sprintf("selisih %s poin sudah melewati batas wajar ±%s, tetapi baru searah pada %d dari %d minggu (syarat %d), jadi dipantau dulu.", bizNum(deref(o.RGI4)), bizNum(thr), o.Consistency, k, o.ConsistencyNeed)
		}
		return fmt.Sprintf("selisih %s poin masih di dalam batas wajar ±%s, tetapi sudah searah pada %d dari %d minggu, jadi mulai dipantau.", bizNum(deref(o.RGI4)), bizNum(thr), o.Consistency, k)
	default:
		return fmt.Sprintf("selisih %s poin masih di dalam batas wajar ±%s, jadi geraknya belum bisa dibedakan dari outlet lain.", bizNum(deref(o.RGI4)), bizNum(thr))
	}
}

// aiFactsModel = fakta tren, pergeseran level, minggu aneh, pola hari, perkiraan.
func aiFactsModel(r *models.BizModelResult, k int) []string {
	var f []string
	if t := r.Trend; t != nil {
		nyata := "BELUM nyata (selangnya memuat nol)"
		if t.Significant {
			nyata = "nyata (selangnya tidak memuat nol)"
		}
		f = append(f, fmt.Sprintf("Tren %d minggu setara kalender: %s, selang %s%% sampai %s%% per minggu, %s.",
			t.Weeks, bizPerMinggu(t.SlopePct), bizNum(t.Lo), bizNum(t.Hi), nyata))
		if t.RecentWeeks > 0 {
			nyata = "BELUM nyata"
			if t.RecentSignificant {
				nyata = "nyata"
			}
			f = append(f, fmt.Sprintf("Tren %d minggu terakhir: %s, %s.", t.RecentWeeks, bizPerMinggu(t.RecentSlopePct), nyata))
		}
	}
	if c := r.Changepoint; c != nil && c.Significant && c.AfterWeeks <= bizModelRecent {
		f = append(f, fmt.Sprintf("Level penjualan bergeser %s sejak minggu %s: rata-rata %s per minggu sebelumnya, %s sesudahnya (setara kalender).",
			bizNaikTurun(c.ShiftPct), bizRentangTgl(c.WeekStart, bizWeekEnd(c.WeekStart)), bizRupiah(c.BeforeMean), bizRupiah(c.AfterMean)))
	}
	for _, a := range r.Anomalies {
		if !bizWithinRecentWeeks(a.WeekStart, bizModelRecent) {
			continue
		}
		arah := "DI ATAS"
		if a.ActualAdj < a.Lo {
			arah = "DI BAWAH"
		}
		f = append(f, fmt.Sprintf("Minggu %s berada %s rentang wajar model: %s setara kalender, rentang wajarnya %s sampai %s.",
			bizRentangTgl(a.WeekStart, bizWeekEnd(a.WeekStart)), arah, bizRupiah(a.ActualAdj), bizRupiah(a.Lo), bizRupiah(a.Hi)))
	}
	if d := r.Dow; d != nil && d.WeekendPct != nil && d.WeekdayPct != nil {
		f = append(f, fmt.Sprintf("Pola hari (hari biasa saja, %d minggu terakhir lawan %d minggu sebelumnya): akhir pekan %s, hari kerja %s. %s", k, k,
			bizNaikTurun(*d.WeekendPct), bizNaikTurun(*d.WeekdayPct), bizHariEkstrem(d.WorstDow, d.WorstPct, d.BestDow, d.BestPct)))
	}
	if fc := r.Forecast; fc != nil && fc.TotalRaw > 0 {
		var kal []string
		for _, w := range fc.Weeks {
			if w.Calendar != "" {
				kal = append(kal, bizRentangTgl(w.WeekStart, bizWeekEnd(w.WeekStart))+": "+w.Calendar)
			}
		}
		ket := "tidak ada libur pada minggu-minggu itu"
		if len(kal) > 0 {
			ket = "kalender ke depan sudah dihitung (" + strings.Join(kal, "; ") + ")"
		}
		dasar := "dasarnya garis tren"
		if fc.Basis == "datar" {
			dasar = "dasarnya level empat minggu terakhir, karena trennya tidak nyata (bukan pola hari)"
		}
		f = append(f, fmt.Sprintf("Perkiraan %s kalau pola bertahan: %s sampai %s, paling mungkin %s; %s; %s.",
			bizRentangTgl(fc.From, fc.To), bizRupiah(fc.TotalLo), bizRupiah(fc.TotalHi), bizRupiah(fc.TotalRaw), ket, dasar))
	}
	return f
}

func aiFactsMarket(rep *models.BusinessAnalysis) []string {
	k := rep.BlockWeeks
	f := []string{fmt.Sprintf("Periode yang digambar: %s sampai %s (%d minggu). Yang dibandingkan: %d minggu terakhir lawan %d minggu sebelumnya.",
		rep.PeriodFrom, rep.PeriodTo, rep.WeeksCount, k, k)}
	if rep.GroupGrowth4 != nil {
		f = append(f, fmt.Sprintf("Gerak pasar %d minggu terakhir lawan %d minggu sebelumnya, setara kalender: %s. Angka mentah: %s.",
			k, k, aiPctWord(rep.GroupGrowth4), aiPctWord(rep.GroupGrowthRaw4)))
	}
	if rep.CalendarEffect != nil && math.Abs(*rep.CalendarEffect) >= 0.5 {
		f = append(f, fmt.Sprintf("Selisih %s poin antara angka mentah dan setara kalender dibuat oleh kalender: %s.", bizNum(math.Abs(*rep.CalendarEffect)), rep.CalendarReason))
	}
	if rep.MarketBand != nil && rep.GroupGrowth4 != nil {
		posisi := "MASIH DI DALAM batas itu, jadi pasar TIDAK disebut turun"
		if *rep.GroupGrowth4 < -*rep.MarketBand {
			posisi = "MELEWATI batas itu, jadi pasar disebut turun"
		}
		f = append(f, fmt.Sprintf("Batas wajar pasar untuk gerak %d minggu lawan %d minggu (bukan per minggu): ±%s%%. Gerak setara kalender %s %s.",
			k, k, bizNum(*rep.MarketBand), aiPctWord(rep.GroupGrowth4), posisi))
	}
	var minG, maxG *float64
	for _, g := range rep.Group {
		if g.Growth == nil {
			continue
		}
		if minG == nil || *g.Growth < *minG {
			v := *g.Growth
			minG = &v
		}
		if maxG == nil || *g.Growth > *maxG {
			v := *g.Growth
			maxG = &v
		}
	}
	if minG != nil && maxG != nil {
		f = append(f, fmt.Sprintf("Gerak pasar per SATU minggu (setara kalender) sepanjang periode berayun antara %s dan %s; ayunan per minggu ini tidak dibandingkan dengan batas wajar pasar.",
			bizNaikTurun(*minG), bizNaikTurun(*maxG)))
	}
	if rep.PanelShare != nil {
		f = append(f, fmt.Sprintf("Cakupan pasar: %d dari %d outlet yang datanya lengkap (%s%% omzet grup).", len(rep.PanelCodes), len(rep.Outlets), bizNum(*rep.PanelShare)))
	}
	daftar := func(xs []string) string {
		if len(xs) == 0 {
			return "tidak ada"
		}
		return strings.Join(xs, ", ")
	}
	var weak, nodata []string
	for _, o := range rep.Outlets {
		switch o.Diagnosis {
		case models.BizDiagWeak:
			weak = append(weak, o.Code)
		case models.BizDiagNoData:
			nodata = append(nodata, o.Code)
		}
	}
	f = append(f, fmt.Sprintf("Kesimpulan outlet — Tertinggal: %s. Lebih Baik: %s. Perlu Dipantau: %s. Belum Bisa Dinilai: %s. Data Kurang: %s. Sisanya Sama Saja.",
		daftar(rep.Lagging), daftar(rep.Leading), daftar(rep.Watch), daftar(weak), daftar(nodata)))
	var naik, turun []string
	for _, o := range rep.Outlets {
		if !o.InPanel || o.Growth4 == nil {
			continue
		}
		if *o.Growth4 < 0 {
			turun = append(turun, o.Code)
		} else {
			naik = append(naik, o.Code)
		}
	}
	f = append(f, fmt.Sprintf("Outlet pembanding yang TURUN setara kalender: %s. Yang NAIK setara kalender: %s. Ringkasnya: %s.",
		daftar(turun), daftar(naik), bizArahOutlet(rep.Outlets)))
	for _, o := range rep.Outlets {
		if o.RGI4 == nil || o.Threshold == nil {
			continue
		}
		posisi := "di dalam batas"
		if math.Abs(*o.RGI4) >= *o.Threshold {
			posisi = "melewati batas"
		}
		f = append(f, fmt.Sprintf("%s (%s): setara kalender %s, selisih %s poin, %s ±%s.",
			o.Code, o.DiagnosisLabel, aiPctWord(o.Growth4), bizNum(*o.RGI4), posisi, bizNum(*o.Threshold)))
	}
	var awal string
	var pasarAkhir *float64
	for _, g := range rep.Group {
		if g.Index == nil {
			continue
		}
		if awal == "" {
			awal = g.Label
		}
		pasarAkhir = g.Index
	}
	if awal != "" && pasarAkhir != nil {
		f = append(f, fmt.Sprintf("Indeks perjalanan: minggu %s = 100; pasar minggu terakhir %s.", awal, bizNum(*pasarAkhir)))
	}
	if m := rep.CalendarModel; m != nil {
		f = append(f, fmt.Sprintf("Pengali kalender: hari libur di hari kerja %s hari biasa (dari %d hari libur); libur sekolah %s di hari kerja dan %s di akhir pekan; kalender terisi sampai %s.",
			bizMult(m.HolidayMult), m.HolidayObs, bizMult(m.SchoolWeekdayMult), bizMult(m.SchoolWeekendMult), m.CoverageUntil))
	}
	if rep.Model != nil && rep.Model.Group != nil {
		for _, s := range aiFactsModel(rep.Model.Group, rep.BlockWeeks) {
			f = append(f, "Pasar — "+s)
		}
	}
	return f
}

// ── Pemeriksa fakta ──────────────────────────────────────────────────────────

var (
	aiNumRe  = regexp.MustCompile(`(?i)(\d[\d.,]*\d|\d)(\s*(juta|jt|miliar|milyar|ribu|rb)\b)?`)
	aiWordRe = regexp.MustCompile(`\bkonsisten\b`)
)

type aiNum struct {
	val      float64 // setelah dikalikan satuan
	base     float64 // angka yang tertulis
	decimals int
	scale    float64
	small    bool // bilangan bulat kecil yang bukan persen/poin/rupiah
}

func aiParseNumbers(s string) []aiNum {
	var out []aiNum
	for _, m := range aiNumRe.FindAllStringSubmatchIndex(s, -1) {
		tok := s[m[2]:m[3]]
		unit := ""
		if m[6] >= 0 {
			unit = strings.ToLower(s[m[6]:m[7]])
		}
		dec := 0
		var norm string
		switch {
		case strings.Contains(tok, ","):
			parts := strings.Split(strings.ReplaceAll(tok, ".", ""), ",")
			norm = parts[0]
			if len(parts) > 1 {
				norm += "." + parts[len(parts)-1]
				dec = len(parts[len(parts)-1])
			}
		case strings.Contains(tok, "."):
			if regexp.MustCompile(`^\d{1,3}(\.\d{3})+$`).MatchString(tok) {
				norm = strings.ReplaceAll(tok, ".", "")
			} else {
				parts := strings.Split(tok, ".")
				norm = parts[0] + "." + parts[len(parts)-1]
				dec = len(parts[len(parts)-1])
			}
		default:
			norm = tok
		}
		var v float64
		if _, err := fmt.Sscanf(norm, "%g", &v); err != nil {
			continue
		}
		scale := 1.0
		switch unit {
		case "juta", "jt":
			scale = 1e6
		case "miliar", "milyar":
			scale = 1e9
		case "ribu", "rb":
			scale = 1e3
		}
		after := strings.ToLower(strings.TrimSpace(s[m[1]:min(len(s), m[1]+8)]))
		before := strings.ToLower(s[max(0, m[0]-4):m[0]])
		small := dec == 0 && scale == 1 && v <= 12 &&
			!strings.HasPrefix(after, "%") && !strings.HasPrefix(after, "poin") && !strings.Contains(before, "rp")
		out = append(out, aiNum{val: v * scale, base: v, decimals: dec, scale: scale, small: small})
	}
	return out
}

func aiRound(v float64, d int) float64 {
	p := math.Pow(10, float64(d))
	return math.Round(v*p) / p
}

// aiNumberAllowed: angka di teks AI harus ada di bahan — tepat, hasil
// pembulatan dari angka bahan dengan jumlah desimal yang sama, atau selisih
// relatif ≤ 0,2% untuk angka ≥ 1.000 (rupiah yang dibulatkan ke ribuan).
func aiNumberAllowed(n aiNum, allowed []float64) bool {
	if n.small {
		return true
	}
	for _, a := range allowed {
		a = math.Abs(a)
		if math.Abs(n.val-a) < 1e-9 {
			return true
		}
		if aiRound(a/n.scale, n.decimals) == n.base {
			return true
		}
		if a >= 1000 && math.Abs(n.val-a)/a <= 0.002 {
			return true
		}
	}
	return false
}

// Pasangan frasa yang saling bertentangan: frasa pertama di teks AI ditolak
// bila bahan tidak memuatnya tetapi memuat frasa kedua.
var aiOpposites = [][2]string{
	{"di atas rentang", "di bawah rentang"}, {"di bawah rentang", "di atas rentang"},
	{"di atas garis", "di bawah rentang"}, {"di bawah garis", "di atas rentang"},
	{"melewati batas", "di dalam batas"}, {"melampaui batas", "di dalam batas"},
	{"di luar batas", "di dalam batas"}, {"di atas batas", "di dalam batas"},
	{"di dalam batas", "melewati batas"}, {"masih dalam batas", "melewati batas"},
	{"sudah konsisten", "belum konsisten"}, {"belum konsisten", "sudah konsisten"},
}

// aiNormPhrases menyamakan variasi frasa sebelum diperiksa, supaya
// "masih dalam batas" dan "masih di dalam batas" dianggap sama.
func aiNormPhrases(s string) string {
	return strings.NewReplacer("masih dalam batas", "masih di dalam batas", "berada dalam batas", "berada di dalam batas",
		"‑", "-", "–", "-").Replace(s)
}

// aiForbidden = pola yang terbukti keliru pada percobaan nyata. Pola ditolak
// kecuali bahannya sendiri memuat frasa unless.
var aiForbidden = []struct {
	re     *regexp.Regexp
	unless string
	why    string
}{
	{regexp.MustCompile(`patokan[^.]{0,40}rata-rata|rata-rata[^.]{0,25}outlet (pembanding|lain)|rata-rata grup`), "rata-rata outlet pembanding", "menyebut patokan sebagai rata-rata, padahal nilai tengah"},
	{regexp.MustCompile(`berturut-turut`), "berturut-turut", "menyebut berturut-turut, padahal syaratnya searah sekian minggu dari blok"},
	{regexp.MustCompile(`asalkan tidak ada libur`), "asalkan tidak ada libur", "mengubah fakta tanpa libur menjadi syarat"},
	{regexp.MustCompile(`semua (\d+ )?outlet[^.]{0,40}(ikut )?turun`), "yang naik setara kalender: tidak ada", "menyebut semua outlet turun padahal ada yang naik"},
}

var aiLabels = []string{"Tertinggal", "Lebih Baik", "Perlu Dipantau", "Sama Saja", "Belum Bisa Dinilai"}

// aiCheck memeriksa satu kolom teks AI terhadap bahan panggilannya.
// ownLabel diisi untuk kolom milik satu outlet: label kesimpulan lain tidak
// boleh muncul di sana.
func aiCheck(text, context string, allowed []float64, ownLabel string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	for _, n := range aiParseNumbers(text) {
		if !aiNumberAllowed(n, allowed) {
			return fmt.Sprintf("angka %s tidak ada di FAKTA/DRAF", strconv.FormatFloat(n.base, 'f', -1, 64))
		}
	}
	lt, lc := aiNormPhrases(strings.ToLower(text)), aiNormPhrases(strings.ToLower(context))
	for _, rule := range aiForbidden {
		if rule.re.MatchString(lt) && (rule.unless == "" || !strings.Contains(lc, rule.unless)) {
			return rule.why
		}
	}
	for _, p := range aiOpposites {
		if strings.Contains(lt, p[0]) && !strings.Contains(lc, p[0]) && strings.Contains(lc, p[1]) {
			return fmt.Sprintf("menulis \"%s\" padahal FAKTA \"%s\"", p[0], p[1])
		}
	}
	if strings.Contains(lc, "belum konsisten") && !strings.Contains(lc, "sudah konsisten") {
		for _, loc := range aiWordRe.FindAllStringIndex(lt, -1) {
			pre := lt[max(0, loc[0]-10):loc[0]]
			if !strings.Contains(pre, "belum") && !strings.Contains(pre, "tidak") && !strings.Contains(pre, "kurang") {
				return "menyebut konsisten padahal FAKTA belum konsisten"
			}
		}
	}
	if ownLabel != "" {
		for _, l := range aiLabels {
			if l != ownLabel && strings.Contains(text, l) {
				return fmt.Sprintf("memakai label \"%s\" padahal kesimpulannya \"%s\"", l, ownLabel)
			}
		}
	}
	return ""
}

func aiAllowedNumbers(payload []byte) []float64 {
	var out []float64
	for _, n := range aiParseNumbers(string(payload)) {
		out = append(out, n.val)
	}
	return out
}

// aiFixMonths merapikan keluaran: nama bulan Inggris yang kadang terselip dan
// spasi sebelum tanda persen ("7,6 %" menjadi "7,6%").
func aiFixMonths(s string) string {
	r := strings.NewReplacer("Oct", "Okt", "October", "Oktober", "Aug", "Agu", "August", "Agustus", "Dec", "Des", "December", "Desember", "May ", "Mei ")
	return aiPctSpace.ReplaceAllString(r.Replace(s), "$1%")
}

var aiPctSpace = regexp.MustCompile(`(\d)\s+%`)

// ── Bahan per panggilan ──────────────────────────────────────────────────────

func aiPayloadSummary(rep *models.BusinessAnalysis) map[string]any {
	insights := make([]map[string]any, 0, len(rep.Insights))
	for i, in := range rep.Insights {
		insights = append(insights, map[string]any{"index": i, "kind": in.Kind, "title_draft": in.Title, "body_draft": in.Body})
	}
	return map[string]any{"FAKTA": aiFactsMarket(rep), "verdict": rep.Verdict,
		"headline_draft": rep.Headline, "verdict_text_draft": rep.VerdictText, "insights_draft": insights}
}

func aiPayloadSections(rep *models.BusinessAnalysis, part []models.BizSection) map[string]any {
	drafts := make([]map[string]any, 0, len(part))
	for _, s := range part {
		drafts = append(drafts, map[string]any{"key": s.Key, "title": s.Title, "lead_draft": s.Lead, "hint_draft": s.Hint})
	}
	return map[string]any{"FAKTA": aiFactsMarket(rep), "sections_draft": drafts}
}

func aiPayloadOutlet(rep *models.BusinessAnalysis, o *models.BizOutlet) map[string]any {
	return map[string]any{"FAKTA": aiFactsOutlet(rep, o), "code": o.Code, "diagnosis_label": o.DiagnosisLabel,
		"note_draft": o.Note, "advice_draft": o.Advice, "breakdown_draft": o.Breakdown, "model_notes_draft": o.ModelNotes}
}

func aiPayloadSocial(rep *models.BusinessAnalysis) map[string]any {
	so := make([]map[string]any, 0, len(rep.Social.Outlets))
	for _, o := range rep.Social.Outlets {
		so = append(so, map[string]any{"code": o.Code, "quadrant_label": o.QuadrantLabel, "reading_draft": o.Reading})
	}
	return map[string]any{"outlets": so}
}

// ── Titik masuk dari GetBusinessAnalysis ─────────────────────────────────────
//
// Narasi disimpan PER BAGIAN, bukan per laporan: ringkasan pasar, kelompok
// bagian halaman, tiap outlet, medsos. Kuncinya hash isi bagian itu sendiri.
// Akibatnya:
//   - rentang 8, 12, 26, 52 minggu memakai ulang narasi outlet yang sama (fakta
//     outlet tidak bergantung pada panjang gambar), jadi jatah harian penyedia
//     gratis tidak habis untuk rentang lain;
//   - bila jatah habis di tengah jalan, bagian yang sudah jadi tetap tampil,
//     sisanya memakai templat dan dilanjutkan sendiri setelah jatah pulih.

var (
	narrativeQuotaMu    sync.Mutex
	narrativeQuotaUntil time.Time
)

func aiQuotaWait() time.Time {
	narrativeQuotaMu.Lock()
	defer narrativeQuotaMu.Unlock()
	return narrativeQuotaUntil
}

func aiResetQuota() {
	narrativeQuotaMu.Lock()
	narrativeQuotaUntil = time.Time{}
	narrativeQuotaMu.Unlock()
}

func aiSetQuota(until time.Time) {
	narrativeQuotaMu.Lock()
	if until.After(narrativeQuotaUntil) {
		narrativeQuotaUntil = until
	}
	narrativeQuotaMu.Unlock()
}

// aiPart = satu bagian narasi: bahannya, cara memintanya, dan cara
// memeriksa hasilnya menjadi aiNarrative parsial.
type aiPart struct {
	name    string
	hash    string
	payload map[string]any
	instr   string
	schema  map[string]any
	maxTok  int
	effort  string
	// decode mengurai teks model; check memeriksa hasil urai terakhir dan
	// mengembalikan kolom yang ditolak; result mengubahnya jadi aiNarrative.
	decode func(string) error
	check  func(ctx string, allowed []float64) []aiRej
	result func() aiNarrative
}

func aiPartHash(name string, payload map[string]any) string {
	raw, _ := json.Marshal(payload)
	h := sha256.Sum256([]byte(narrativeLabel + "|" + narrativePromptVersion + "|" + name + "|" + string(raw)))
	return hex.EncodeToString(h[:])
}

// bizAIParts menyusun seluruh bagian narasi untuk satu laporan.
func bizAIParts(rep *models.BusinessAnalysis) []*aiPart {
	var parts []*aiPart

	// Ringkasan pasar.
	{
		var res struct {
			Headline    string `json:"headline"`
			VerdictText string `json:"verdict_text"`
			Insights    []struct {
				Index int    `json:"index"`
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"insights"`
		}
		p := &aiPart{name: "ringkasan", payload: aiPayloadSummary(rep), maxTok: narrativeTokSummary, effort: "medium",
			instr: fmt.Sprintf(`Tulis RINGKASAN PASAR halaman.
- headline: satu baris, paling banyak 12 kata, memuat angka inti.
- verdict_text: 2 sampai 4 kalimat: apa yang terjadi, seberapa besar (angka setara kalender dan mentah), siapa yang perlu bergerak, apa yang belum bisa disimpulkan.
- insights: tepat %d butir, "index" 0 sampai %d berurutan sesuai insights_draft; title paling banyak 8 kata; body 1 sampai 3 kalimat yang mempertahankan makna body_draft.`, len(rep.Insights), len(rep.Insights)-1),
			schema: aiObj(map[string]any{
				"headline": aiStr(), "verdict_text": aiStr(),
				"insights": aiArr(aiObj(map[string]any{"index": map[string]any{"type": "integer"}, "title": aiStr(), "body": aiStr()}, "index", "title", "body")),
			}, "headline", "verdict_text", "insights")}
		p.decode = func(t string) error {
			res.Headline, res.VerdictText, res.Insights = "", "", nil
			return json.Unmarshal([]byte(t), &res)
		}
		p.check = func(c string, al []float64) []aiRej {
			var rej []aiRej
			if w := aiCheck(res.Headline, c, al, ""); w != "" {
				rej = append(rej, aiRej{"headline", w, func() { res.Headline = "" }})
			}
			if w := aiCheck(res.VerdictText, c, al, ""); w != "" {
				rej = append(rej, aiRej{"verdict_text", w, func() { res.VerdictText = "" }})
			}
			for i := range res.Insights {
				i := i
				if w := aiCheck(res.Insights[i].Title+" "+res.Insights[i].Body, c, al, ""); w != "" {
					rej = append(rej, aiRej{fmt.Sprintf("insights[%d]", res.Insights[i].Index), w,
						func() { res.Insights[i].Title, res.Insights[i].Body = "", "" }})
				}
			}
			return rej
		}
		p.result = func() aiNarrative {
			n := aiNarrative{Headline: res.Headline, VerdictText: res.VerdictText}
			n.Insights = res.Insights
			return n
		}
		parts = append(parts, p)
	}

	// Bagian halaman, beberapa per panggilan.
	for from := 0; from < len(rep.Sections); from += narrativeSectionsPer {
		part := rep.Sections[from:min(from+narrativeSectionsPer, len(rep.Sections))]
		var keys []string
		for _, sct := range part {
			keys = append(keys, sct.Key)
		}
		var res struct {
			Sections []struct {
				Key  string `json:"key"`
				Lead string `json:"lead"`
				Hint string `json:"hint"`
			} `json:"sections"`
		}
		p := &aiPart{name: "bagian " + strings.Join(keys, ","), payload: aiPayloadSections(rep, part), maxTok: narrativeTokSections, effort: "low",
			instr: fmt.Sprintf(`Tulis ulang BAGIAN halaman: satu butir untuk setiap key berikut, dengan key yang sama persis: %s.
- lead: 1 sampai 3 kalimat yang mempertahankan makna lead_draft dan memuat angka periode ini dari FAKTA bila relevan.
- hint: 1 sampai 2 kalimat cara membaca bagian itu; string kosong bila hint_draft kosong.`, strings.Join(keys, ", ")),
			schema: aiObj(map[string]any{
				"sections": aiArr(aiObj(map[string]any{"key": aiStr(), "lead": aiStr(), "hint": aiStr()}, "key", "lead", "hint")),
			}, "sections")}
		p.decode = func(t string) error { res.Sections = nil; return json.Unmarshal([]byte(t), &res) }
		p.check = func(c string, al []float64) []aiRej {
			var rej []aiRej
			for i := range res.Sections {
				i := i
				if w := aiCheck(res.Sections[i].Lead+" "+res.Sections[i].Hint, c, al, ""); w != "" {
					rej = append(rej, aiRej{"section " + res.Sections[i].Key, w, func() { res.Sections[i].Lead, res.Sections[i].Hint = "", "" }})
				}
			}
			return rej
		}
		p.result = func() aiNarrative { n := aiNarrative{}; n.Sections = res.Sections; return n }
		parts = append(parts, p)
	}

	// Tiap outlet.
	for i := range rep.Outlets {
		o := &rep.Outlets[i]
		var res struct {
			Code       string   `json:"code"`
			Note       string   `json:"note"`
			Advice     string   `json:"advice"`
			Breakdown  string   `json:"breakdown"`
			ModelNotes []string `json:"model_notes"`
		}
		p := &aiPart{name: "outlet " + o.Code, payload: aiPayloadOutlet(rep, o), maxTok: narrativeTokOutlet, effort: "medium",
			instr: fmt.Sprintf(`Tulis bagian OUTLET %s (%s). Kesimpulannya "%s" — jangan diubah.
- code: "%s".
- note: 2 sampai 3 kalimat mengapa kesimpulannya begitu. Sebabnya WAJIB mengikuti "ALASAN KESIMPULAN" di FAKTA; angka pendukungnya dari FAKTA.
- advice: 2 sampai 4 kalimat langkah konkret untuk manajer outlet ini; pakai pola hari, tren, pergeseran level, atau minggu aneh dari FAKTA bila ada; bila kesimpulannya Sama Saja atau Belum Bisa Dinilai, katakan juga apa yang tidak perlu dilakukan.
- breakdown: 1 sampai 2 kalimat tentang jumlah struk lawan belanja per struk (string kosong bila breakdown_draft kosong).
- model_notes: 2 sampai 4 kalimat pengamatan tren, perkiraan, minggu aneh, pergeseran level, pola hari (array kosong bila model_notes_draft kosong).`,
				o.Name, o.Code, o.DiagnosisLabel, o.Code),
			schema: aiSchemaOutlet()}
		label := o.DiagnosisLabel
		code := o.Code
		p.decode = func(t string) error {
			res.Code, res.Note, res.Advice, res.Breakdown, res.ModelNotes = "", "", "", "", nil
			return json.Unmarshal([]byte(t), &res)
		}
		p.check = func(c string, al []float64) []aiRej {
			var rej []aiRej
			if w := aiCheck(res.Note, c, al, label); w != "" {
				rej = append(rej, aiRej{"note", w, func() { res.Note = "" }})
			}
			if w := aiCheck(res.Advice, c, al, label); w != "" {
				rej = append(rej, aiRej{"advice", w, func() { res.Advice = "" }})
			}
			if w := aiCheck(res.Breakdown, c, al, label); w != "" {
				rej = append(rej, aiRej{"breakdown", w, func() { res.Breakdown = "" }})
			}
			if w := aiCheck(strings.Join(res.ModelNotes, " "), c, al, label); w != "" {
				rej = append(rej, aiRej{"model_notes", w, func() { res.ModelNotes = nil }})
			}
			return rej
		}
		p.result = func() aiNarrative {
			n := aiNarrative{}
			n.Outlets = append(n.Outlets, struct {
				Code       string   `json:"code"`
				Note       string   `json:"note"`
				Advice     string   `json:"advice"`
				Breakdown  string   `json:"breakdown"`
				ModelNotes []string `json:"model_notes"`
			}{Code: code, Note: res.Note, Advice: res.Advice, Breakdown: res.Breakdown, ModelNotes: res.ModelNotes})
			return n
		}
		parts = append(parts, p)
	}

	// Medsos.
	if rep.Social != nil && rep.Social.Enabled && len(rep.Social.Outlets) > 0 {
		var res struct {
			Readings []struct {
				Code    string `json:"code"`
				Reading string `json:"reading"`
			} `json:"readings"`
		}
		p := &aiPart{name: "medsos", payload: aiPayloadSocial(rep), maxTok: narrativeTokSocial, effort: "low",
			instr:  "Tulis pembacaan MEDSOS: satu butir untuk setiap outlet di outlets (kunci \"code\" sama persis); reading 1 sampai 2 kalimat yang mempertahankan makna reading_draft dan quadrant_label.",
			schema: aiSchemaSocial()}
		p.decode = func(t string) error { res.Readings = nil; return json.Unmarshal([]byte(t), &res) }
		p.check = func(c string, al []float64) []aiRej {
			var rej []aiRej
			for i := range res.Readings {
				i := i
				if w := aiCheck(res.Readings[i].Reading, c, al, ""); w != "" {
					rej = append(rej, aiRej{"medsos " + res.Readings[i].Code, w, func() { res.Readings[i].Reading = "" }})
				}
			}
			return rej
		}
		p.result = func() aiNarrative { n := aiNarrative{}; n.SocialOutlets = res.Readings; return n }
		parts = append(parts, p)
	}

	for _, p := range parts {
		p.hash = aiPartHash(p.name, p.payload)
	}
	return parts
}

func bizApplyNarrativeAI(rep *models.BusinessAnalysis) {
	rep.Narrative = &models.BizNarrativeStatus{Status: "template"}
	if narrativeProv == nil {
		rep.Narrative.Note = "Narasi AI belum diaktifkan (NARRATIVE_PROVIDER kosong); halaman memakai kalimat templat yang dirakit dari angka."
		return
	}
	rep.Narrative.Model = narrativeLabel

	parts := bizAIParts(rep)
	hashes := make([]string, 0, len(parts))
	for _, p := range parts {
		hashes = append(hashes, p.hash)
	}
	stored := map[string][]byte{}
	var latest time.Time
	rows, err := database.DB.Query(`SELECT hash, payload, created_at FROM business_narratives WHERE hash = ANY($1::text[])`, pgTextArray(hashes))
	if err == nil {
		for rows.Next() {
			var h string
			var raw []byte
			var at time.Time
			if rows.Scan(&h, &raw, &at) == nil {
				stored[h] = raw
				if at.After(latest) {
					latest = at
				}
			}
		}
		rows.Close()
	}

	// Terapkan dulu yang sudah ada; ringkasan terakhir supaya tidak tertimpa.
	rejected := 0
	var missing []*aiPart
	for _, p := range parts {
		raw, ok := stored[p.hash]
		if !ok {
			missing = append(missing, p)
			continue
		}
		var n aiNarrative
		if json.Unmarshal(raw, &n) == nil {
			bizAIApply(rep, &n)
			rejected += len(n.Rejected)
		}
	}
	done := len(parts) - len(missing)
	if done > 0 {
		rep.Narrative.GeneratedAt = latest.In(GetTimezoneLocation()).Format("2006-01-02 15:04")
	}

	switch {
	case len(missing) == 0:
		rep.Narrative.Status = "ai"
		if rejected > 0 {
			rep.Narrative.Note = fmt.Sprintf("%d kalimat tetap memakai templat karena tidak lolos pemeriksaan fakta otomatis.", rejected)
		} else {
			rep.Narrative.Note = "Seluruh kalimat AI lolos pemeriksaan fakta otomatis (angka dan arah dicocokkan dengan data)."
		}
	case time.Now().Before(aiQuotaWait()):
		rep.Narrative.Status = "partial"
		if done == 0 {
			rep.Narrative.Status = "template"
		}
		rep.Narrative.Note = fmt.Sprintf("Jatah harian AI gratis sedang habis; %d dari %d bagian sudah ditulis AI, sisanya memakai templat dan dilanjutkan otomatis sekitar pukul %s.",
			done, len(parts), aiQuotaWait().In(GetTimezoneLocation()).Format("15:04"))
	default:
		rep.Narrative.Status = "generating"
		rep.Narrative.Note = fmt.Sprintf("Narasi AI sedang disusun di latar: %d dari %d bagian sudah jadi. Halaman memuat ulang otomatis.", done, len(parts))
		go bizGenerateParts(missing)
	}
}

// aiRej = satu kolom yang ditolak pemeriksa, beserta cara mengosongkannya.
type aiRej struct {
	field, why string
	blank      func()
}

// bizGenerateParts membuat bagian-bagian yang belum ada, satu per satu,
// memeriksa tiap kolom, dan menyimpan tiap bagian begitu jadi.
func bizGenerateParts(parts []*aiPart) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	saved, rejectedTotal := 0, 0

	for _, p := range parts {
		if time.Now().Before(aiQuotaWait()) {
			break
		}
		narrativeMu.Lock()
		if narrativeRunning[p.hash] {
			narrativeMu.Unlock()
			continue
		}
		narrativeRunning[p.hash] = true
		narrativeMu.Unlock()

		n, ok := bizRunPart(ctx, p)

		narrativeMu.Lock()
		delete(narrativeRunning, p.hash)
		narrativeMu.Unlock()
		if !ok {
			continue
		}
		raw, _ := json.Marshal(n)
		if _, err := database.DB.Exec(`
			INSERT INTO business_narratives (hash, model, payload, created_at)
			VALUES ($1, $2, $3, now() AT TIME ZONE 'UTC')
			ON CONFLICT (hash) DO UPDATE SET model = EXCLUDED.model, payload = EXCLUDED.payload, created_at = EXCLUDED.created_at`,
			p.hash, narrativeLabel, raw); err != nil {
			log.Printf("[Narasi AI] %s gagal disimpan: %v", p.name, err)
			continue
		}
		saved++
		rejectedTotal += len(n.Rejected)
	}
	database.DB.Exec(`DELETE FROM business_narratives WHERE created_at < now() AT TIME ZONE 'UTC' - interval '45 days'`)
	if saved > 0 {
		log.Printf("[Narasi AI] selesai: %d dari %d bagian disimpan dalam %.0fs (%s, %d kolom kembali ke templat)",
			saved, len(parts), time.Since(start).Seconds(), narrativeLabel, rejectedTotal)
	}
}

// bizRunPart memanggil model untuk satu bagian, memeriksa, dan sekali
// mengulang dengan umpan balik bila ada kolom yang ditolak.
func bizRunPart(ctx context.Context, p *aiPart) (aiNarrative, bool) {
	raw, _ := json.Marshal(p.payload)
	allowed := aiAllowedNumbers(raw)
	ctxText := string(raw)
	feedback := ""
	for attempt := 0; attempt < 2; attempt++ {
		text, err := narrativeProv.Generate(ctx, aiSystemPrompt, p.instr+feedback+"\n\nDATA (JSON):\n"+ctxText, p.schema, p.maxTok, p.effort)
		if err != nil {
			if q, habis := err.(errQuotaExhausted); habis {
				aiSetQuota(time.Now().Add(q.wait))
				log.Printf("[Narasi AI] %s: %v — dilanjutkan setelah jatah pulih", p.name, err)
				return aiNarrative{}, false
			}
			log.Printf("[Narasi AI] %s gagal: %v", p.name, err)
			continue
		}
		if err := p.decode(aiExtractJSON(aiFixMonths(text))); err != nil {
			log.Printf("[Narasi AI] %s JSON tidak terbaca: %v", p.name, err)
			continue
		}
		rej := p.check(ctxText, allowed)
		if len(rej) == 0 {
			return p.result(), true
		}
		if attempt == 0 {
			var why []string
			for _, r := range rej {
				why = append(why, r.field+": "+r.why)
			}
			feedback = "\n\nPERCOBAAN SEBELUMNYA DITOLAK PEMERIKSA:\n- " + strings.Join(why, "\n- ") +
				"\nTulis ulang SELURUH JSON. Setiap angka harus ada di FAKTA/DRAF; kata arah harus sama dengan FAKTA."
			continue
		}
		n := aiNarrative{}
		for _, r := range rej {
			r.blank()
			log.Printf("[Narasi AI] %s · %s ditolak: %s", p.name, r.field, r.why)
		}
		n = p.result()
		for _, r := range rej {
			n.Rejected = append(n.Rejected, p.name+" · "+r.field+": "+r.why)
		}
		return n, true
	}
	return aiNarrative{}, false
}

// aiExtractJSON memotong teks ke objek JSON terluar — model kadang
// membungkus jawabannya dengan pagar kode atau kalimat pengantar.
func aiExtractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}

// bizAIApply menempelkan narasi ke laporan. Hanya mengganti teks; angka dan
// vonis tidak tersentuh. Kolom kosong (ditolak pemeriksa atau tidak diisi)
// membiarkan kalimat templatnya. Mengembalikan false bila tidak ada satu pun
// yang bisa diterapkan.
func bizAIApply(rep *models.BusinessAnalysis, n *aiNarrative) bool {
	if n == nil {
		return false
	}
	applied := 0
	set := func(dst *string, v string) {
		if v = strings.TrimSpace(v); v != "" {
			*dst = v
			applied++
		}
	}
	byCode := map[string]int{}
	for i := range rep.Outlets {
		byCode[rep.Outlets[i].Code] = i
	}
	for _, o := range n.Outlets {
		i, ok := byCode[o.Code]
		if !ok {
			continue
		}
		set(&rep.Outlets[i].Note, o.Note)
		set(&rep.Outlets[i].Advice, o.Advice)
		if rep.Outlets[i].Breakdown != "" {
			set(&rep.Outlets[i].Breakdown, o.Breakdown)
		}
		if len(o.ModelNotes) > 0 && len(rep.Outlets[i].ModelNotes) > 0 {
			var notes []string
			for _, s := range o.ModelNotes {
				if strings.TrimSpace(s) != "" {
					notes = append(notes, strings.TrimSpace(s))
				}
			}
			if len(notes) > 0 {
				rep.Outlets[i].ModelNotes = notes
				applied++
				if rep.Model != nil {
					if r, ok := rep.Model.Outlets[o.Code]; ok {
						r.Notes = notes
					}
				}
			}
		}
	}
	set(&rep.Headline, n.Headline)
	set(&rep.VerdictText, n.VerdictText)
	for _, in := range n.Insights {
		if in.Index >= 0 && in.Index < len(rep.Insights) {
			set(&rep.Insights[in.Index].Title, in.Title)
			set(&rep.Insights[in.Index].Body, in.Body)
		}
	}
	secIdx := map[string]int{}
	for i := range rep.Sections {
		secIdx[rep.Sections[i].Key] = i
	}
	for _, s := range n.Sections {
		if i, ok := secIdx[s.Key]; ok {
			set(&rep.Sections[i].Lead, s.Lead)
			if rep.Sections[i].Hint != "" {
				set(&rep.Sections[i].Hint, s.Hint)
			}
		}
	}
	if rep.Social != nil {
		socIdx := map[string]int{}
		for i := range rep.Social.Outlets {
			socIdx[rep.Social.Outlets[i].Code] = i
		}
		for _, s := range n.SocialOutlets {
			if i, ok := socIdx[s.Code]; ok {
				set(&rep.Social.Outlets[i].Reading, s.Reading)
			}
		}
	}
	return applied > 0
}

// ── Pemanas cache ────────────────────────────────────────────────────────────

// StartNarrativeScheduler memanaskan narasi untuk rentang bawaan tiap pagi
// (setelah minggu berjalan berganti data) dan beberapa menit setelah boot.
func StartNarrativeScheduler() {
	if narrativeProv == nil {
		return
	}
	go func() {
		time.Sleep(4 * time.Minute)
		bizWarmNarratives("boot")
		lastDay := ""
		t := time.NewTicker(10 * time.Minute)
		defer t.Stop()
		for range t.C {
			now := time.Now().In(GetTimezoneLocation())
			day := now.Format("2006-01-02")
			if now.Hour() == 5 && now.Minute() >= 30 && lastDay != day {
				lastDay = day
				bizWarmNarratives("pagi")
				continue
			}
			// Jatah penyedia sempat habis dan kini pulih: lanjutkan bagian
			// yang tertunda tanpa menunggu ada yang membuka halaman.
			if q := aiQuotaWait(); !q.IsZero() && time.Now().After(q) {
				aiResetQuota()
				bizWarmNarratives("lanjutan")
			}
		}
	}()
}

func bizWarmNarratives(alasan string) {
	// Hanya rentang bawaan halaman. Rentang lain dibuat saat pertama dibuka,
	// supaya jatah harian penyedia gratis tidak habis untuk yang tidak dibaca.
	for _, w := range []int{12} {
		if _, err := GetBusinessAnalysis(w); err != nil {
			log.Printf("[Narasi AI] pemanasan %s (weeks=%d) gagal: %v", alasan, w, err)
		}
	}
}

package services

import (
	"testing"
	"time"
)

// Yang diuji di sini bukan ketepatan angka, melainkan janji-janji yang menjaga
// alamat IP ini tetap boleh membaca: jadwalnya tidak pernah berbunyi pada menit
// yang sama tiap hari, penyamarannya tetap masuk akal sebagai satu rombongan,
// dan akun yang baru ditolak benar-benar didiamkan.

func TestMenitTargetBerpindahTiapHari(t *testing.T) {
	hari := []string{
		"2026-09-11", "2026-09-12", "2026-09-13", "2026-09-14",
		"2026-09-15", "2026-09-16", "2026-09-17", "2026-09-18",
	}
	seen := map[int]bool{}
	for _, h := range hari {
		m := socialMenitTarget(h)
		if m < socialJendelaMulai || m >= socialJendelaMulai+socialJendelaLebar {
			t.Errorf("%s: menit %d di luar jendela %d..%d", h, m,
				socialJendelaMulai, socialJendelaMulai+socialJendelaLebar)
		}
		// Stabil sepanjang hari itu: restart tidak boleh menggeser jadwal.
		if lagi := socialMenitTarget(h); lagi != m {
			t.Errorf("%s: menit berubah antar panggilan (%d lalu %d)", h, m, lagi)
		}
		seen[m] = true
	}
	// Delapan hari yang semuanya jatuh pada menit yang sama berarti jadwalnya
	// tidak benar-benar berpindah — justru keteraturan itu yang dihindari.
	if len(seen) < 4 {
		t.Errorf("delapan hari hanya menghasilkan %d menit berbeda: %v", len(seen), seen)
	}
}

func TestSamaranTetapMasukAkal(t *testing.T) {
	// Safari di iPhone TIDAK PERNAH mengirim Sec-CH-UA. Menempelkannya akan
	// menandai permintaan ini sebagai palsu — lebih mencolok daripada tidak
	// menyamar sama sekali.
	for k := range samaranPonsel.header {
		if k == "Sec-Ch-Ua" || k == "Sec-Ch-Ua-Mobile" || k == "Sec-Ch-Ua-Platform" {
			t.Errorf("penyamaran ponsel mengirim header Chrome: %s", k)
		}
	}
	// Perayap pratinjau tidak mengirim header navigasi peramban.
	for k := range samaranPratinjau.header {
		if len(k) > 10 && k[:10] == "Sec-Fetch-" {
			t.Errorf("penyamaran pratinjau mengirim header peramban: %s", k)
		}
	}
	// Accept-Encoding tidak boleh diisi di mana pun: Go menambahkannya sendiri
	// dan membuka kompresinya: mengisinya manual membuat parser menerima byte
	// terkompresi.
	for _, sam := range []socialSamaran{samaranPonsel, samaranPratinjau, samaranMeja} {
		if _, ada := sam.header["Accept-Encoding"]; ada {
			t.Errorf("%s mengisi Accept-Encoding sendiri", sam.nama)
		}
	}
}

func TestSamaranPonselSetiaPerAkun(t *testing.T) {
	// Satu akun harus selalu dikunjungi "peramban" yang sama. Versi yang
	// berubah-ubah tiap hari untuk profil yang sama jauh lebih aneh daripada
	// versi yang tetap.
	a1 := samaranPonselUntuk("nusantara_healing_chill")
	a2 := samaranPonselUntuk("nusantara_healing_chill")
	if a1.ua != a2.ua {
		t.Error("penyamaran satu akun berubah antar panggilan")
	}
	beda := map[string]bool{}
	for _, u := range []string{"satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan"} {
		beda[samaranPonselUntuk(u).ua] = true
	}
	if len(beda) < 2 {
		t.Errorf("delapan akun berbeda hanya memakai %d penyamaran", len(beda))
	}
}

func TestAkunYangDitolakDidiamkan(t *testing.T) {
	const id = "AKUN-UJI-ISTIRAHAT"
	if _, boleh := socialBolehDitarik(id); !boleh {
		t.Fatal("akun bersih seharusnya boleh ditarik")
	}

	socialIstirahatkan(id, time.Hour)
	sisa, boleh := socialBolehDitarik(id)
	if boleh {
		t.Error("akun yang baru ditolak masih boleh ditarik")
	}
	if sisa <= 0 || sisa > time.Hour {
		t.Errorf("sisa istirahat tidak masuk akal: %v", sisa)
	}

	// Masa istirahat yang sudah lewat harus membuka sendiri, bukan menahan
	// akunnya selamanya.
	socialIstirahatkan(id, -time.Second)
	if _, boleh := socialBolehDitarik(id); !boleh {
		t.Error("masa istirahat sudah lewat tetapi akunnya masih ditahan")
	}
}

func TestPenolakanDikenaliSebagaiPenolakan(t *testing.T) {
	biasa := errorBiasa("halaman berubah bentuk")
	if _, ya := socialIstirahat(biasa); ya {
		t.Error("galat biasa salah dikenali sebagai penolakan laju")
	}

	tolak := &socialDitolak{pesan: "dibatasi", tunggu: 2 * time.Hour}
	lama, ya := socialIstirahat(tolak)
	if !ya || lama != 2*time.Hour {
		t.Errorf("penolakan tidak dikenali: ya=%v lama=%v", ya, lama)
	}
}

type errorBiasa string

func (e errorBiasa) Error() string { return string(e) }

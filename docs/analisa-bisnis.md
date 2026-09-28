# Analisa Bisnis — cara angkanya dihitung

Halaman **Laporan → Analisa Bisnis** menjawab satu keputusan: penurunan
penjualan ini masalah **outlet** (manajer) atau masalah **pasar/Markom**?
Dokumen ini menjelaskan urutan perhitungannya, kenapa tiap langkah ada, dan
apa yang harus dirawat supaya vonisnya tetap jujur.

Kode: `services/business_analysis.go` (mesin), `services/business_calendar.go`
(kalender & bobot hari), `services/business_analysis_narrative.go` (kalimat),
`services/business_analysis_social.go` (medsos), `models/business_analysis.go`.

## 1. Penjualan setara kalender

Bisnis ini hidup dari akhir pekan dan hari libur. Pada data Jul–Sep 2026:
58% omzet mingguan jatuh di Sabtu–Minggu; Senin 17 Agustus menjual 6× Senin
biasa; Selasa 25 Agustus (Maulid) 5× Selasa biasa; minggu terakhir libur
sekolah 2× minggu biasa.

Karena itu, sebelum dibandingkan, tiap minggu diskalakan ke **minggu biasa**:

```
bobot hari(outlet, tanggal) = penjualan wajar outlet pada nama hari itu
                              (nilai tengah hari-hari BIASA dengan nama hari sama)
                              × pengali libur      bila libur nasional/cuti bersama di Sen–Jum
                              × pengali libur sekolah  bila hari libur sekolah
bobot minggu       = jumlah bobot 7 harinya
faktor kalender    = bobot minggu biasa ÷ bobot minggu ini
setara kalender    = penjualan sebenarnya × faktor kalender
```

Pengalinya **dipelajari dari data sendiri**: nilai tengah rasio (penjualan
hari bertanda ÷ penjualan wajar hari itu). Nilai grup digabung dari seluruh
outlet; nilai per outlet = rasio outlet itu sendiri **ditarik ke nilai grup**
sebanding jumlah pengamatannya (skala log, `bizBlendMult`, k = 5) — kafe
keluarga dan wahana ATV tidak merasakan libur sekolah dengan besaran yang
sama. Syarat minimum untuk nilai grup: 3 hari libur teramati, 5 hari libur
sekolah per kelompok (hari kerja / akhir pekan). Sebelum cukup, dipakai patokan awal yang
disebutkan terang-terangan di halaman: hari libur dianggap seperti hari
Minggu; libur sekolah tidak dikoreksi.

Angka sebenarnya **tetap ditampilkan** di setiap tabel; angka setara kalender
berdiri di sampingnya dan dipakai untuk semua perbandingan.

## 2. Kalender bisnis

Tabel `business_calendar` (kunci: tanggal + jenis). Jenis:

| kind | Pengaruh |
|---|---|
| `libur_nasional`, `cuti_bersama` | pengali libur, hanya bila jatuh Senin–Jumat |
| `libur_sekolah` | pengali libur sekolah (hari kerja / akhir pekan terpisah) |
| `kejadian` | catatan saja (jalan ditutup, festival, cuaca ekstrem) |

Isi awal: SKB 3 Menteri 2026 + libur sekolah **perkiraan** (22 Jun–12 Jul
2026, 21 Des 2026–3 Jan 2027). Disisipkan sekali (penanda
`mig_business_calendar_seed_v1` di `app_settings`), jadi koreksi tangan tidak
ditimpa saat boot. Kelola dari kartu **Kalender Libur & Hari Khusus** di
halaman (izin `reports.business_analysis.manage`), atau lewat API:

```
GET    /api/v1/admin/business-calendar?from=YYYY-MM-DD&to=YYYY-MM-DD
PUT    /api/v1/admin/business-calendar        {day, kind, name}
DELETE /api/v1/admin/business-calendar/:day?kind=...
```

**Yang harus dirawat:** libur keagamaan 2027 (bergeser tiap tahun) dan libur
sekolah provinsi. Halaman memberi peringatan bila kalender belum menjangkau
akhir periode.

## 3. Garis pasar

Pertumbuhan *same-store* seluruh outlet panel, ditimbang omzet, pada angka
setara kalender. Blok **tetap 4 minggu** lawan 4 minggu sebelumnya (3 bila
riwayat < 8 minggu), selalu menempel di ujung riwayat.

**Pilihan panjang blok** (`?block=4|2`, kotak "Pembanding" di halaman;
bawaan 4): blok 2 minggu menangkap perubahan lebih cepat, tetapi galat baku
selisihnya ∝ 1/√K sehingga batas wajar outlet dan pasar ≈ 1,41× lebih lebar,
dan vonis butuh **kedua** minggu searah (`bizConsistencyNeed`: 3 dari 4,
2 dari 3, 2 dari 2). Jendela pola hari di model Python ikut panjang blok
(`dow_days`). Narasi AI disimpan terpisah per panjang blok (fakta outlet
berbeda), jadi mode 2 minggu memakai jatah penyedia tambahan saat pertama
dibuka; pemanasan pagi hanya untuk blok 4.

**Jendela hitung tetap:** blok, batas wajar, dan pengali kalender selalu
dihitung dari **26 minggu terakhir yang tersedia** (`bizLearnWeeks`), berapa
pun rentang yang dipilih. Rentang hanya memotong gambar (`bizTrimDisplay`,
indeks disandarkan ulang ke 100 pada minggu pertama yang tergambar). Vonis di
8 minggu dan di 52 minggu karena itu identik. Ditampilkan pula angka mentah dan **berapa poin yang dijelaskan
kalender** (mentah − setara kalender).

Batas wajar pasar = 2 × simpangan robust (MAD) pertumbuhan mingguan pasar
÷ √(panjang blok). Pasar disebut turun hanya bila melewati batas ini.

## 4. Selisih tiap outlet (RGI)

```
RGI = pertumbuhan blok outlet (setara kalender)
      − nilai tengah pertumbuhan blok SELURUH outlet panel
```

Nilai tengah semua outlet, **termasuk outlet yang dinilai**. Median hampir
tidak terpengaruh satu anggota, dan patokan yang satu untuk semua menghapus
lompatan yang dibuat pembanding tanpa-diri (dulu NHC −42,0% dan SB −44,5%
terbaca +2,5 dan −2,5: beda 5 poin dari beda 2,5).

## 5. Batas wajar & konsistensi

Derau tiap outlet = gabungan (bobot 50/50) dari:

- simpangan **robust** (MAD × 1,4826) RGI mingguan outlet itu **sebelum blok
  terakhir** (supaya kemerosotan yang diuji tidak ikut melebarkan batas yang
  mengujinya, dan satu minggu yang meledak tidak menguasai taksirannya);
- model derau grup `c / √(struk per minggu)`, dengan `c` = nilai tengah
  (derau outlet × √struk) seluruh outlet — outlet kecil otomatis lebih
  longgar.

Batas wajar = 2 × derau ÷ √(panjang blok), lantai 2 poin. Outlet yang batas
wajarnya > 3 × median batas grup (minimum 12) masuk **Belum Bisa Dinilai**.

Vonis:

| Kesimpulan | Syarat |
|---|---|
| Tertinggal / Lebih Baik | \|RGI\| ≥ batas **dan** searah pada ≥ 3 dari 4 minggu terakhir |
| Perlu Dipantau | \|RGI\| ≥ batas tapi belum konsisten, **atau** \|RGI\| ≥ 60% batas dan konsisten |
| Sama Saja | sisanya |

Terjemahan rupiah: `seharusnya = blok sebelumnya (setara kalender) × (1 +
patokan) ÷ faktor kalender blok terakhir`; selisih = kenyataan − seharusnya.

## 6. Vonis halaman

Dua uji bebas: pasar turun (setara kalender, di bawah batas) dan ada outlet
Tertinggal. Keduanya bisa benar bersamaan. Kalimat vonis selalu menyebut angka
mentah, poin yang dijelaskan kalender, dan libur mana yang membuatnya.
**Markom tidak pernah ditagih dari angka penjualan saja**: bila pasar turun
setelah kalender, kalimatnya meminta mencari sebab di luar outlet dan menunjuk
blok medsos sebagai ujinya.

## 7. Blok medsos (Kinerja Markom)

- Sumbu penjualan pada penyandingan = **RGI** (selisih dengan outlet lain),
  bukan pertumbuhan mentah — kalau tidak, bulan tanpa libur melempar semua
  outlet ke kuadran bawah.
- Gerak jangkauan punya **pita derau**: 2 galat baku Poisson dari besar
  hitungannya, minimum 10 poin. Kuadran hanya ditetapkan bila kedua sisi
  melewati deraunya; kalau tidak, "Belum Berarti".
- Korelasi Pearson diberi uji-t 5%; dengan 8 outlet, |r| < 0,7 dilaporkan
  "belum bisa dibedakan dari kebetulan".
- Snapshot yang dibulatkan platform ("1,2M") kalah dari pembacaan persis pada
  minggu yang sama (`services/social.go`, LATERAL `ORDER BY approx ASC`).
- Blok pembanding medsos kosong sampai riwayat snapshot mencapai 2 × panjang
  blok (data mulai 11 Sep 2026 → sekitar pertengahan Nov 2026).

## 8. Layanan analitik Python (tren, perkiraan, pola hari)

Folder `analytics/` (FastAPI + numpy + scipy; compose service `analytics`,
env `ANALYTICS_URL` di app). Go mengirim deret **setara kalender** tiap
outlet dan pasar (`services/business_analysis_model.go`), Python
mengembalikan angka, Go merakit kalimatnya (`bizModelStatements`). Tanpa
layanan ini halaman tetap tampil; hanya bagian model yang hilang.

| Keluaran | Metode |
|---|---|
| Tren %/minggu (jendela & 8 mgg terakhir) | Theil–Sen pada log penjualan, selang 95%; "nyata" bila selang tidak memuat nol |
| Perkiraan 4 minggu | garis tren (atau level 4 mgg terakhir bila tren tidak nyata) diekstrapolasi; rentang 80% dari MAD residual, melebar seiring jarak; angka sebenarnya = setara ÷ faktor kalender minggu itu |
| Minggu aneh | residual ≥ 2,5 simpangan robust dari garis |
| Pergeseran level | segmentasi biner pada log; skor beda dua segmen ≥ 3 |
| Pola hari | 4 mgg terakhir vs 4 mgg sebelumnya, hari biasa saja; akhir pekan vs hari kerja, hari terlemah/terkuat |

Kalimat dinamis yang dihasilkan (per outlet dan pasar): tren dan arah
belakangan, pergeseran level yang masih baru, minggu aneh dalam 8 minggu
terakhir, pola hari, perkiraan dengan libur ke depan. Temuan tingkat pasar:
tren pasar, pergeseran level pasar, akhir pekan/hari kerja yang melemah,
outlet dengan minggu aneh terbaru, tren turun yang belum tertangkap blok.

Tes: `python -m pytest analytics/test_stats.py`; kalimat: `TestKalimatModelDinamis`.

## 9. Narasi AI (semua kalimat ditulis ulang dari angka)

`services/business_analysis_ai.go`. Sebuah model bahasa menulis ulang seluruh
teks halaman — judul, vonis, temuan, judul dan "cara membaca" tiap bagian,
catatan/saran/uraian tiap outlet, pengamatan model, pembacaan medsos — dari
DATA angka laporan, dengan keluaran JSON berskema dan aturan keras: tidak
boleh ada angka/tanggal/nama yang tidak ada di data, vonis tidak boleh
berubah, kalimat harus spesifik per outlet.

**Penyedia** (`NARRATIVE_PROVIDER` di `.env`; kosong = kalimat templat):

| Penyedia | Biaya | Catatan |
|---|---|---|
| `openai` (dipakai) | gratis berbatas | Groq: `NARRATIVE_BASE_URL=https://api.groq.com/openai/v1`, `NARRATIVE_MODEL=openai/gpt-oss-120b`, `NARRATIVE_API_KEY=gsk_…`. Batas gratis terbaca dari header: 1.000 permintaan/hari, 8.000 token/menit. Angka penjualan dikirim ke Groq. |
| `anthropic` | berbayar | Claude lewat SDK resmi (`NARRATIVE_API_KEY`, model bawaan `claude-opus-5`); belum pernah dijalankan di server ini. |
| `ollama` | gratis, lokal | **Jangan dipakai di server produksi ini.** 28 Sep 2026 gemma3:4b membuat server 8 GB tanpa swap membeku dan reboot paksa; satu bagian narasi pun >5 menit di 6 inti. Service-nya sudah dihapus dari compose. |

Pengatur laju (`openaiProvider`): semua panggilan dijalankan satu per satu;
sebelum tiap panggilan, sisa token dari header `x-ratelimit-remaining-tokens`
dibandingkan dengan perkiraan kebutuhan dan ditunggu sampai
`x-ratelimit-reset-tokens` bila kurang; 429 ditunggu sesuai `retry-after`;
jeda > 10 menit dianggap jatah harian habis dan pembuatan dihentikan. Model
gpt-oss dikirim dengan `reasoning_effort: low`. Penyedia yang tidak mendukung
`json_schema` ketat otomatis diberi mode `json_object` dengan skema di prompt.

**Pemeriksa fakta** (wajib; ditambahkan setelah percobaan pertama 28 Sep 2026
membalik arah minggu aneh NS, membandingkan pertumbuhan dengan batas wajar,
dan mengarang "selisih 7 poin"):

1. Model tidak menerima angka mentah untuk ditafsirkan, melainkan **FAKTA**
   yang sudah ditafsirkan kode (`aiFactsOutlet`, `aiFactsMarket`,
   `aiFactsModel`), misalnya "Selisih −12,2 poin … MASIH DI DALAM batas
   wajar", "searah 2 dari 4 minggu … BELUM konsisten", "Minggu 14–20 Sep
   berada DI BAWAH rentang wajar", "Yang NAIK setara kalender: TS".
2. Tiap kolom teks diperiksa `aiCheck`: setiap angka harus ada di FAKTA/DRAF
   panggilan itu (tepat, hasil pembulatan dengan jumlah desimal yang ditulis,
   atau ≤ 0,2% untuk rupiah ≥ 1.000); frasa arah tidak boleh bertentangan
   (`aiOpposites`: di atas/di bawah rentang, di dalam/melewati batas,
   sudah/belum konsisten); label kesimpulan outlet lain tidak boleh muncul;
   pola keliru yang pernah terjadi ditolak (`aiForbidden`: patokan disebut
   rata-rata, "berturut-turut", "asalkan tidak ada libur", "semua outlet
   turun" padahal ada yang naik).
3. Kolom yang ditolak dikirim ulang sekali dengan alasan penolakannya; bila
   masih gagal, kolom itu dikosongkan dan **kalimat templatnya yang tampil**.
   Jumlahnya ditampilkan di badge halaman dan dicatat di log
   (`[Narasi AI] … ditolak: …`).

Model `gpt-oss-120b` dipanggil dengan `reasoning_effort: medium`
(penalaran `low` terbukti ceroboh dengan angka).

Narasi dibuat dan **disimpan per bagian** (`bizAIParts`): ringkasan pasar
(judul, vonis, temuan; penalaran medium), bagian halaman per lima (penalaran
low), satu per outlet (medium), dan medsos (low) — 14 bagian untuk data
sekarang. Kunci cache tiap bagian = sha256(penyedia/model + versi prompt +
nama bagian + bahannya) di `business_narratives`; baris > 45 hari dihapus.

- Bahan outlet tidak bergantung pada panjang gambar, jadi rentang 8, 12, 26,
  dan 52 minggu **memakai ulang** narasi outlet yang sama; rentang lain hanya
  menambah ringkasan dan bagian halaman.
- **Jatah Groq gratis juga dibatasi per hari** (terbukti 28 Sep 2026: habis
  setelah empat narasi penuh, pulih ±30 menit kemudian). Saat jatah habis,
  bagian yang sudah jadi tetap dipakai, sisanya memakai templat, badge
  berstatus `partial` dengan jam lanjutan, dan penjadwal melanjutkan sendiri
  begitu jatah pulih (cek tiap 10 menit).
- Satu narasi penuh ±60–70 ribu token dan ±12 menit (jatah 8.000 token/menit).
- Halaman tidak menunggu: status `generating` + UI memuat ulang tiap 30 detik
  (paling lama 15 menit). Penjadwal memanaskan rentang 12 minggu empat menit
  setelah boot dan tiap pagi pukul 05.30 waktu aplikasi.

Tanpa penyedia: `status = "template"`; kalimat templat pun sudah memuat
angka outlet (selang tren, hari terlemah, minggu pergeseran level;
`bizEnrichAdvice`).

## 10. Batas yang diketahui

- Pengali libur dipelajari dari data yang sama yang dianalisa (in-sample);
  wajar untuk riwayat pendek, dan akan makin stabil seiring bertambahnya libur.
- Belum ada pembanding tahun-ke-tahun; baru mungkin setelah ≥ 53 minggu data.
- Jumlah tonton konten dibaca hari ini untuk konten yang terbit minggu itu,
  sehingga blok terakhir cenderung terbaca lebih rendah (bias kebaruan).
- Tingkat gratis Groq bisa berubah batasnya atau modelnya dipensiunkan
  (Llama 3.3 70B sudah tidak tersedia per 28 Sep 2026). Bila narasi berhenti
  terbarui, lihat `docker logs cloud-pos-app-1 | grep "Narasi AI"`; halaman
  otomatis kembali ke kalimat templat.

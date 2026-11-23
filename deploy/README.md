# deploy/

Perkakas lingkungan demo dan skema rilis. Panduan lengkap: [`docs/lingkungan-demo.md`](../docs/lingkungan-demo.md).

| Berkas | Fungsi |
|---|---|
| `demo.sh` | Lingkungan demo `demo.nbp.co.id`: pasang, reset data dari produksi (disanitasi), jalankan rilis, status |
| `release.sh` | Build rilis sekali → uji di demo → promosi image yang sama ke produksi → rollback |
| `lib.sh` | Nama container, proyek compose, jaringan, dan lokasi arsip (satu-satunya sumber kebenaran) |
| `demo/compose.yml` | Stack demo (`cloud-pos-demo`): db, app, analytics, web — jaringan tanpa internet |
| `demo/nginx.conf` | Konfigurasi web demo: SPA + API demo + penanda DEMO + noindex |
| `demo/sanitize.sql` | Sanitasi salinan produksi (kunci API, WA, CCTV, medsos, nomor HP, sesi) |
| `demo/training-users.example.csv` | Contoh akun latihan |

Alur singkat:

```bash
deploy/release.sh build <nama>              # 1. build image rilis
deploy/demo.sh deploy <rilis> --reset       # 2. jalankan di demo dengan data produksi segar
#                                             3. uji di https://demo.nbp.co.id
deploy/release.sh promote <rilis>           # 4. naikkan image yang sama ke produksi
```

Keadaan & rahasia demo serta arsip rilis disimpan di luar repo:
`../pos-integration-staging/demo/` dan `../pos-integration-staging/releases/`.

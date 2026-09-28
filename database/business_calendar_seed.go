package database

import "time"

// ── Isi awal kalender bisnis ─────────────────────────────────────────────────
//
// Analisa Bisnis menskalakan minggu yang memuat hari libur ke "minggu biasa"
// sebelum membandingkan outlet, jadi kalender ini ikut menentukan vonisnya.
// Isi awal di bawah diambil dari SKB 3 Menteri tentang hari libur nasional dan
// cuti bersama 2026, ditambah libur sekolah yang PERKIRAAN (kalender dinas
// pendidikan tiap provinsi berbeda). Semuanya bersumber 'seed' dan bisa
// diubah/dihapus di halaman Analisa Bisnis; penyisipan hanya dilakukan sekali
// (penanda di app_settings) supaya koreksi tangan tidak ditimpa saat boot.

type businessCalendarEntry struct {
	Day, Kind, Name string
}

func businessCalendarSeed() []businessCalendarEntry {
	out := []businessCalendarEntry{
		// Hari libur nasional 2026
		{"2026-01-01", "libur_nasional", "Tahun Baru 2026"},
		{"2026-01-16", "libur_nasional", "Isra Mikraj Nabi Muhammad SAW"},
		{"2026-02-17", "libur_nasional", "Tahun Baru Imlek 2577"},
		{"2026-03-19", "libur_nasional", "Hari Suci Nyepi"},
		{"2026-03-20", "libur_nasional", "Idul Fitri 1447 H"},
		{"2026-03-21", "libur_nasional", "Idul Fitri 1447 H (hari kedua)"},
		{"2026-04-03", "libur_nasional", "Wafat Yesus Kristus"},
		{"2026-04-05", "libur_nasional", "Kebangkitan Yesus Kristus (Paskah)"},
		{"2026-05-01", "libur_nasional", "Hari Buruh Internasional"},
		{"2026-05-14", "libur_nasional", "Kenaikan Yesus Kristus"},
		{"2026-05-27", "libur_nasional", "Idul Adha 1447 H"},
		{"2026-05-31", "libur_nasional", "Hari Raya Waisak 2570"},
		{"2026-06-01", "libur_nasional", "Hari Lahir Pancasila"},
		{"2026-06-16", "libur_nasional", "Tahun Baru Islam 1448 H"},
		{"2026-08-17", "libur_nasional", "HUT ke-81 Kemerdekaan RI"},
		{"2026-08-25", "libur_nasional", "Maulid Nabi Muhammad SAW"},
		{"2026-12-25", "libur_nasional", "Hari Raya Natal"},

		// Cuti bersama 2026 (SKB 3 Menteri — periksa bila ada revisi)
		{"2026-02-16", "cuti_bersama", "Cuti bersama Tahun Baru Imlek"},
		{"2026-03-18", "cuti_bersama", "Cuti bersama Hari Suci Nyepi"},
		{"2026-03-23", "cuti_bersama", "Cuti bersama Idul Fitri"},
		{"2026-03-24", "cuti_bersama", "Cuti bersama Idul Fitri"},
		{"2026-05-15", "cuti_bersama", "Cuti bersama Kenaikan Yesus Kristus"},
		{"2026-05-28", "cuti_bersama", "Cuti bersama Idul Adha"},
		{"2026-12-24", "cuti_bersama", "Cuti bersama Natal"},

		// 2027: hanya yang tanggalnya tetap. Libur keagamaan 2027 bergeser dan
		// harus diisi setelah SKB-nya terbit.
		{"2027-01-01", "libur_nasional", "Tahun Baru 2027"},
		{"2027-05-01", "libur_nasional", "Hari Buruh Internasional"},
		{"2027-06-01", "libur_nasional", "Hari Lahir Pancasila"},
		{"2027-08-17", "libur_nasional", "HUT ke-82 Kemerdekaan RI"},
		{"2027-12-25", "libur_nasional", "Hari Raya Natal"},
	}

	// Libur sekolah (perkiraan; sesuaikan dengan kalender dinas pendidikan).
	// Akhir tahun ajaran 2025/2026 dan libur semester ganjil 2026/2027.
	for _, r := range [][2]string{
		{"2026-06-22", "2026-07-12"},
		{"2026-12-21", "2027-01-03"},
	} {
		a, _ := time.Parse("2006-01-02", r[0])
		b, _ := time.Parse("2006-01-02", r[1])
		for d := a; !d.After(b); d = d.AddDate(0, 0, 1) {
			out = append(out, businessCalendarEntry{
				d.Format("2006-01-02"), "libur_sekolah", "Libur sekolah (perkiraan)",
			})
		}
	}
	return out
}

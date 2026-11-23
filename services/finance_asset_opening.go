package services

import (
	"fmt"
	"strings"
	"time"
)

// ── Tanggal saldo awal aset ────────────────────────────────────────────────
//
// Register aset diisi sekaligus pada 19 Nov 2026 dengan tanggal beli asli
// (sebagian sejak 2022). Tanpa batas, penyusutannya mundur ke Laba-Rugi
// bulan-bulan yang sudah lewat. Keputusan user 19 Nov 2026: saldo awal aset
// per 1 Nov 2026 (app_settings asset_opening_date).
//
// Aset terdaftar manual (tanpa pengadaan) yang dibeli SEBELUM tanggal itu:
//   - dicatat sekali pada tanggal saldo awal: Dr Aset Tetap / Cr Modal Awal
//     senilai NILAI BUKU per hari sebelumnya (perolehan − akumulasi);
//   - beban penyusutan baru dihitung sejak tanggal itu.
// Aset dari pengadaan dan aset yang dibeli pada/sesudah tanggal itu tetap
// seperti biasa. Nilai buku di modul Aset dan Neraca tidak berubah.

const settingAssetOpeningDate = "asset_opening_date"

// AssetOpeningDate: tanggal saldo awal aset (YYYY-MM-DD), "" bila tidak diatur
// atau tidak sah.
func AssetOpeningDate() string {
	v, err := GetSetting(settingAssetOpeningDate)
	if err != nil {
		return ""
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(v))
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// assetOpeningLiteral: tanggal saldo awal sebagai literal SQL yang aman
// (sudah lolos time.Parse), mis. DATE '2026-11-01'; "" bila tidak diatur.
func assetOpeningLiteral(cut string) string {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(cut))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("DATE '%s'", t.Format("2006-01-02"))
}

// assetMonthsSQL: bulan penuh sejak tanggal beli pada tanggal d (rumus yang
// sama dengan depreciationForPeriod & modul aset).
func assetMonthsSQL(d string) string {
	return fmt.Sprintf(`GREATEST(0, (DATE_PART('year', AGE(%[1]s, a.purchase_date)) * 12
		+ DATE_PART('month', AGE(%[1]s, a.purchase_date)))::int)`, d)
}

// assetUnitBookValueSQL: nilai buku per unit pada tanggal d, persis
// perolehan − akumulasi depreciationForPeriod (bentuk pembagian yang sama).
func assetUnitBookValueSQL(d string) string {
	return fmt.Sprintf(`(a.purchase_price - CASE WHEN COALESCE(a.useful_life_months, 0) > 0
		THEN LEAST(LEAST(%[1]s, a.useful_life_months) * GREATEST(a.purchase_price - COALESCE(a.residual_value, 0), 0)
		           / a.useful_life_months, GREATEST(a.purchase_price - COALESCE(a.residual_value, 0), 0))
		ELSE 0 END)`, assetMonthsSQL(d))
}

// assetPreOpeningCond: aset terdaftar manual yang dibeli sebelum tanggal saldo
// awal ("FALSE" bila tidak diatur).
func assetPreOpeningCond(lit string) string {
	if lit == "" {
		return "FALSE"
	}
	return fmt.Sprintf("(a.purchase_request_id IS NULL AND a.purchase_date IS NOT NULL AND a.purchase_date < %s)", lit)
}

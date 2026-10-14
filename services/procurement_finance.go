package services

import (
	"fmt"
	"strings"
)

// ── Status pengajuan pengadaan ─────────────────────────────────────────────
//
// Alur: pending (Menunggu Verifikasi) → verified (Menunggu Persetujuan) →
// approved → payment_requested → partial/paid → received; atau rejected /
// cancelled.
//
// "Berkomitmen" = sudah disetujui penyetuju. HANYA status ini yang dihitung
// sebagai belanja berjalan: hutang usaha, akrual Buku Besar, projek berjalan
// di Neraca, on-order MRP, tolak-GRN-manual, dan belanja vendor. pending dan
// verified masih pra-persetujuan; rejected dan cancelled mati.
//
// Selalu DAFTAR PUTIH. Filter lama berbentuk daftar hitam (NOT IN ('pending',
// 'rejected','cancelled')) membuat setiap status baru otomatis ikut terhitung —
// begitu 'verified' ditambahkan, pengajuan yang belum disetujui owner langsung
// masuk hutang, GL, dan MRP (T-42).

// PurchaseStatuses: seluruh status yang sah (juga isi CHECK constraint di
// database/migrations.go — ubah keduanya bersamaan).
var PurchaseStatuses = []string{
	"pending", "verified", "approved", "payment_requested", "partial", "paid", "received", "rejected", "cancelled",
}

// prCommittedStatuses: sudah disetujui — dihitung sebagai belanja berjalan.
var prCommittedStatuses = []string{"approved", "payment_requested", "partial", "paid", "received"}

// prPreApprovalStatuses: masih hidup tetapi belum disetujui.
var prPreApprovalStatuses = []string{"pending", "verified"}

func sqlStatusList(statuses []string) string {
	return "'" + strings.Join(statuses, "','") + "'"
}

func statusIn(s string, list []string) bool {
	for _, v := range list {
		if s == v {
			return true
		}
	}
	return false
}

// prCommittedCond: baris berstatus berkomitmen (sudah disetujui).
func prCommittedCond(alias string) string {
	return fmt.Sprintf("%sstatus IN (%s)", prefixOf(alias), sqlStatusList(prCommittedStatuses))
}

// prPreApprovalCond: baris yang masih menunggu verifikasi atau persetujuan.
func prPreApprovalCond(alias string) string {
	return fmt.Sprintf("%sstatus IN (%s)", prefixOf(alias), sqlStatusList(prPreApprovalStatuses))
}

// prLiveCond: baris yang masih hidup — pra-persetujuan + berkomitmen (tidak
// ditolak, tidak dibatalkan). Dipakai untuk angka "pengajuan berjalan" dan
// pemesanan anggaran projek, BUKAN untuk uang/hutang.
func prLiveCond(alias string) string {
	return fmt.Sprintf("%sstatus IN (%s)", prefixOf(alias),
		sqlStatusList(append(append([]string{}, prPreApprovalStatuses...), prCommittedStatuses...)))
}

// IsCommittedPurchaseStatus: status sudah disetujui (belanja berjalan).
func IsCommittedPurchaseStatus(s string) bool { return statusIn(s, prCommittedStatuses) }

// PurchaseStatusCommittedFilter: nilai filter status khusus (bukan status
// dokumen) untuk ListPurchaseRequests — semua status berkomitmen sekaligus.
// Pilihan "Semua" di halaman Pembayaran dan ekspor Excel-nya memakai ini:
// dokumen yang belum disetujui owner, ditolak, atau batal bukan tagihan.
const PurchaseStatusCommittedFilter = "committed"

// isLivePurchaseStatus: status belum ditolak/dibatalkan (versi Go prLiveCond).
func isLivePurchaseStatus(s string) bool {
	return statusIn(s, prPreApprovalStatuses) || statusIn(s, prCommittedStatuses)
}

// IsValidPurchaseStatus: status dikenal (filter daftar, validasi input).
func IsValidPurchaseStatus(s string) bool { return statusIn(s, PurchaseStatuses) }

// Definisi tunggal angka pengadaan untuk keuangan.
//
// Sebelum berkas ini ada, "hutang usaha" punya TIGA definisi berbeda yang
// hidup bersamaan dan tidak pernah cocok satu sama lain:
//
//   - Halaman Pembayaran : status NOT IN (pending,rejected,cancelled)
//                          AND total_final > paid_amount        → benar
//   - Laporan Neraca     : status IN ('approved','priced','partial')
//                          ('priced' bahkan bukan status yang pernah ada, dan
//                          'payment_requested' — tagihan yang justru sudah di
//                          meja keuangan — tidak ikut terhitung)
//   - Buku Besar         : status IN ('approved','payment_requested','partial')
//                          (lebih baik, tapi tetap melewatkan 'received' yang
//                          belum lunas)
//
// Semua konsumen sekarang memakai builder di bawah ini. Kalau definisinya perlu
// berubah, ubah di sini sekali.

// prefixOf mengembalikan "alias." atau "" bila tanpa alias.
func prefixOf(alias string) string {
	if alias == "" {
		return ""
	}
	return alias + "."
}

// outstandingCondFor: baris yang masih menyisakan kewajiban bayar ke vendor.
// Berbasis nominal, bukan nama status, supaya pengajuan yang sudah diterima
// tapi belum lunas tetap terhitung sebagai hutang. Hanya status berkomitmen:
// pengajuan yang baru diverifikasi (belum disetujui) bukan kewajiban kepada
// siapa pun, walau harga finalnya sudah terisi.
func outstandingCondFor(alias string) string {
	p := prefixOf(alias)
	return fmt.Sprintf("%[2]s AND %[1]stotal_final > %[1]spaid_amount", p, prCommittedCond(alias))
}

// payableExpr: nilai sisa kewajiban satu baris. Selalu dipakai bersama
// outstandingCondFor supaya tidak pernah menghasilkan angka negatif.
func payableExpr(alias string) string {
	p := prefixOf(alias)
	return fmt.Sprintf("(%[1]stotal_final - %[1]spaid_amount)", p)
}

// countableCond: baris pengadaan yang boleh ikut dijumlahkan di laporan mana
// pun. Master yang seluruh itemnya sudah pindah ke pecahan adalah cangkang
// bernilai 0 — nilainya sudah diwakili pecahannya.
//
// Ini menggantikan "split_status IS NULL" yang dulu dipakai laporan keuangan.
// Filter lama itu membuang SELURUH master, termasuk master yang masih memegang
// sisa item dan punya pembayaran sendiri — uangnya lenyap dari Arus Kas,
// Laba/Rugi, Neraca dan Buku Besar sekaligus.
func countableCond(alias string) string {
	return "NOT " + fullySplitMasterCond(alias)
}

// procurementClassExpr: kelas belanja sebuah pengajuan untuk laporan keuangan.
//
//	bahan  → HPP bahan baku: pengajuan berjenis 'bahan' (Pengadaan Bahan
//	         Baku, apa pun nama barisnya), atau pengajuan barang dapur yang
//	         ada di katalog stok
//	jasa   → beban jasa & layanan
//	modal  → belanja modal: barang perlengkapan → Aset Tetap, BUKAN beban.
//	         Bebannya masuk P&L lewat penyusutan.
//	projek → belanja projek (barang/jasa bertaut projek) → Projek Berjalan
//
// Sebelum ini semua pengajuan 'barang' dihitung HPP, sehingga membeli AC 15
// juta menurunkan laba kotor bulan itu sebesar 15 juta dan tidak pernah muncul
// sebagai aset. Kelasnya diturunkan dari kolom yang sudah ada: goods_kind
// (ditentukan saat pengajuan dibuat) dan project_id. Dokumen lama tanpa
// goods_kind tetap dihitung 'bahan' — sama seperti sebelumnya. Pengajuan
// berjenis 'bahan' diperiksa sebelum goods_kind supaya tidak pernah jatuh ke
// 'modal' (bahan yang belum ada di katalog bukan aset tetap).
func procurementClassExpr(alias string) string {
	p := prefixOf(alias)
	return fmt.Sprintf(`(CASE
		WHEN %[1]sproject_id IS NOT NULL THEN 'projek'
		WHEN %[1]srequest_type = 'bahan' THEN 'bahan'
		WHEN %[1]srequest_type = 'jasa' THEN 'jasa'
		WHEN COALESCE(%[1]sgoods_kind, '') = 'perlengkapan' THEN 'modal'
		ELSE 'bahan' END)`, p)
}

// procurementCashOutFrom: sumber arus kas KELUAR pengadaan, satu baris per
// PEMBAYARAN.
//
// Sengaja dari payment_histories, bukan purchase_requests.paid_amount:
// satu pengajuan bisa dicicil beberapa kali, sementara paid_at hanya menyimpan
// tanggal cicilan TERAKHIR. Memakai paid_amount+paid_at menaruh seluruh nilai
// cicilan pada bulan pembayaran terakhir dan mengosongkan kas bulan-bulan
// sebelumnya — laporan jadi tidak cocok dengan mutasi rekening.
const procurementCashOutFrom = `payment_histories ph
			JOIN purchase_requests pr ON pr.id = ph.purchase_request_id`

// procurementCashOutWhere: syarat baris pembayaran yang ikut dihitung pada
// rentang tanggal $1..$2. extra diisi filter tambahan (mis. scope outlet).
func procurementCashOutWhere(extra string) string {
	return fmt.Sprintf(
		"WHERE %s\n\t\t\t  AND ph.created_at >= tz_day_start($1::date) AND ph.created_at < tz_day_start($2::date + 1)%s",
		countableCond("pr"), extra)
}

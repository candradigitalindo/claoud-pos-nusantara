package services

import "fmt"

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
// tapi belum lunas tetap terhitung sebagai hutang.
func outstandingCondFor(alias string) string {
	p := prefixOf(alias)
	return fmt.Sprintf("%[1]sstatus NOT IN ('pending','rejected','cancelled') AND %[1]stotal_final > %[1]spaid_amount", p)
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

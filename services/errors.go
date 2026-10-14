package services

import "fmt"

// ValidationError menandai kesalahan yang berasal dari ISI PERMINTAAN, bukan
// dari kegagalan sistem.
//
// Handler pengadaan sengaja menyembunyikan error internal di balik pesan umum
// agar detail database tidak bocor. Akibatnya pesan validasi yang justru perlu
// dibaca pengguna ("kenapa pengajuan saya ditolak") ikut tertelan. Tipe ini
// memisahkan keduanya: yang ber-tipe ini boleh ditampilkan apa adanya.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

func Invalid(format string, a ...interface{}) error {
	return ValidationError{Msg: fmt.Sprintf(format, a...)}
}

// ConflictError menandai dokumen yang sudah berubah sejak dibaca pemanggil
// (version/updated_at atau statusnya berbeda). Handler memetakannya ke HTTP
// 409 supaya layar memuat ulang dan keputusan diambil atas isi terbaru —
// bukan atas angka yang sudah tidak berlaku.
type ConflictError struct{ Msg string }

func (e ConflictError) Error() string { return e.Msg }

func Conflict(format string, a ...interface{}) error {
	return ConflictError{Msg: fmt.Sprintf(format, a...)}
}

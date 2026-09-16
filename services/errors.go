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

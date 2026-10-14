package database

import "fmt"

// RunAuthMigrations — skema tambahan autentikasi. Dipanggil main.go tepat
// setelah RunMigrations; sengaja di berkas terpisah dari migrations.go. Tiap
// pernyataan idempoten sehingga aman dijalankan di setiap boot.
//
// cloud_admins.token_version (T-55): versi sesi per akun. Disalin ke klaim
// "ver" JWT saat login, lalu dicocokkan middleware pada setiap permintaan
// (dengan cache singkat). Versi dinaikkan saat password diganti/direset, akun
// diaktifkan/dinonaktifkan, role akun diganti atau di-rename, dan scope
// role-nya berubah — semua token lama akun itu langsung tertolak. Token lama
// tanpa klaim "ver" dianggap versi 0, jadi sesi yang sudah ada tetap berjalan
// setelah deploy selama versi akunnya belum pernah dinaikkan.
func RunAuthMigrations() error {
	stmts := []string{
		`ALTER TABLE cloud_admins ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 0`,
	}
	for _, s := range stmts {
		if _, err := DB.Exec(s); err != nil {
			return fmt.Errorf("migrasi autentikasi: %w", err)
		}
	}
	return nil
}

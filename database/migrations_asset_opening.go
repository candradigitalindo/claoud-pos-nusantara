package database

// RunAssetOpeningMigrations: tanggal saldo awal aset (keputusan user 19 Nov
// 2026: 1 Nov 2026). Aset terdaftar manual yang dibeli sebelum tanggal ini
// masuk Buku Besar sebagai saldo awal senilai nilai bukunya per tanggal itu;
// penyusutan sebelumnya tidak dibebankan ke Laba-Rugi bulan-bulan lalu.
// ON CONFLICT DO NOTHING: perubahan nilai sesudahnya bertahan saat boot.
func RunAssetOpeningMigrations() error {
	_, err := DB.Exec(`INSERT INTO app_settings (key, value) VALUES ('asset_opening_date', '2026-11-01') ON CONFLICT (key) DO NOTHING`)
	return err
}

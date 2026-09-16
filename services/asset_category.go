package services

import (
	"fmt"
	"strings"

	"cloud-pos/database"
	"cloud-pos/models"
)

// Master kategori aset.
//
// Kategori dulu diketik bebas, sehingga "Elektronik", "elektronik", dan
// "Elektronic" menjadi tiga kelompok berbeda di dashboard, laporan penyusutan,
// dan pencocokan interval perawatan. Master ini membuat kelompoknya tunggal,
// sekaligus menyimpan dua angka bawaan per kategori: umur ekonomis dan
// interval perawatan preventif.

func ListAssetCategories(includeInactive bool) ([]models.AssetCategory, error) {
	q := `
		SELECT c.id, c.name, COALESCE(c.useful_life_months,0), COALESCE(c.maintenance_interval_months,0),
		       COALESCE(c.notes,''), c.is_active,
		       (SELECT COUNT(*) FROM assets a WHERE lower(a.category) = lower(c.name) AND a.is_deleted = false)
		FROM asset_categories c`
	if !includeInactive {
		q += ` WHERE c.is_active = true`
	}
	q += ` ORDER BY c.name`
	rows, err := database.DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]models.AssetCategory, 0)
	for rows.Next() {
		var c models.AssetCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.UsefulLifeMonths, &c.MaintenanceIntervalMonths,
			&c.Notes, &c.IsActive, &c.AssetCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func normalizeCategoryInput(req *models.AssetCategoryRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return Invalid("nama kategori wajib diisi")
	}
	if req.UsefulLifeMonths < 0 || req.MaintenanceIntervalMonths < 0 {
		return Invalid("umur ekonomis dan interval perawatan tidak boleh negatif")
	}
	return nil
}

func CreateAssetCategory(req models.AssetCategoryRequest) (*models.AssetCategory, error) {
	if err := normalizeCategoryInput(&req); err != nil {
		return nil, err
	}
	var exists bool
	database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM asset_categories WHERE lower(name) = lower($1))`,
		req.Name).Scan(&exists)
	if exists {
		return nil, Invalid("kategori %q sudah ada", req.Name)
	}
	id := NewULID()
	if _, err := database.DB.Exec(`
		INSERT INTO asset_categories (id, name, useful_life_months, maintenance_interval_months, notes, is_active)
		VALUES ($1,$2,$3,$4,$5,true)`,
		id, req.Name, req.UsefulLifeMonths, req.MaintenanceIntervalMonths, req.Notes); err != nil {
		return nil, err
	}
	return getAssetCategory(id)
}

func UpdateAssetCategory(id string, req models.AssetCategoryRequest) (*models.AssetCategory, error) {
	if err := normalizeCategoryInput(&req); err != nil {
		return nil, err
	}
	var oldName string
	if err := database.DB.QueryRow(`SELECT name FROM asset_categories WHERE id = $1`, id).Scan(&oldName); err != nil {
		return nil, fmt.Errorf("kategori tidak ditemukan")
	}
	var clash bool
	database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM asset_categories WHERE lower(name) = lower($1) AND id <> $2)`,
		req.Name, id).Scan(&clash)
	if clash {
		return nil, Invalid("kategori %q sudah ada", req.Name)
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		UPDATE asset_categories SET name=$1, useful_life_months=$2, maintenance_interval_months=$3,
			notes=$4, is_active=$5, updated_at=(now() AT TIME ZONE 'UTC') WHERE id=$6`,
		req.Name, req.UsefulLifeMonths, req.MaintenanceIntervalMonths, req.Notes, req.IsActive, id); err != nil {
		return nil, err
	}
	// Ganti nama ikut memperbarui aset yang memakainya — kalau tidak, asetnya
	// mendadak bernaung di kategori yang sudah tidak ada.
	if !strings.EqualFold(oldName, req.Name) {
		if _, err := tx.Exec(`UPDATE assets SET category=$1, updated_at=(now() AT TIME ZONE 'UTC')
			WHERE lower(category) = lower($2)`, req.Name, oldName); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return getAssetCategory(id)
}

// DeleteAssetCategory menolak menghapus kategori yang masih dipakai — data aset
// tidak boleh menunjuk kategori yang lenyap. Yang sudah tidak dipakai lagi
// sebaiknya dinonaktifkan, bukan dihapus.
func DeleteAssetCategory(id string) error {
	var name string
	if err := database.DB.QueryRow(`SELECT name FROM asset_categories WHERE id = $1`, id).Scan(&name); err != nil {
		return fmt.Errorf("kategori tidak ditemukan")
	}
	var used int
	database.DB.QueryRow(`SELECT COUNT(*) FROM assets WHERE lower(category) = lower($1) AND is_deleted = false`,
		name).Scan(&used)
	if used > 0 {
		return Invalid("kategori %q masih dipakai %d aset — nonaktifkan saja bila tidak dipakai lagi", name, used)
	}
	_, err := database.DB.Exec(`DELETE FROM asset_categories WHERE id = $1`, id)
	return err
}

func getAssetCategory(id string) (*models.AssetCategory, error) {
	var c models.AssetCategory
	err := database.DB.QueryRow(`
		SELECT c.id, c.name, COALESCE(c.useful_life_months,0), COALESCE(c.maintenance_interval_months,0),
		       COALESCE(c.notes,''), c.is_active,
		       (SELECT COUNT(*) FROM assets a WHERE lower(a.category) = lower(c.name) AND a.is_deleted = false)
		FROM asset_categories c WHERE c.id = $1`, id).
		Scan(&c.ID, &c.Name, &c.UsefulLifeMonths, &c.MaintenanceIntervalMonths, &c.Notes, &c.IsActive, &c.AssetCount)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// resolveAssetCategory memastikan kategori yang dipakai aset terdaftar di
// master, dan mengembalikan ejaan resminya (supaya huruf besar-kecil seragam).
func resolveAssetCategory(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil // kategori boleh kosong; yang dilarang adalah yang tidak terdaftar
	}
	var official string
	err := database.DB.QueryRow(`SELECT name FROM asset_categories WHERE lower(name) = lower($1)`, name).Scan(&official)
	if err != nil {
		return "", Invalid("kategori %q tidak terdaftar — pilih dari daftar Kategori Aset, atau tambahkan dulu di sana", name)
	}
	return official, nil
}

// categoryDefaults mengambil umur ekonomis & interval perawatan bawaan.
func categoryDefaults(name string) (life int, interval int) {
	database.DB.QueryRow(`
		SELECT COALESCE(useful_life_months,0), COALESCE(maintenance_interval_months,0)
		FROM asset_categories WHERE lower(name) = lower($1)`, strings.TrimSpace(name)).Scan(&life, &interval)
	return
}

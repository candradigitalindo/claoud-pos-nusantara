package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"strings"

	"github.com/lib/pq"
)

func ListFnbDevelopments(outlet string, scope []string) ([]models.FnbDevelopment, error) {
	filter, err := fnbScope(outlet, scope)
	if err != nil {
		return nil, err
	}
	rows, err := database.DB.Query(`SELECT d.id,d.outlet_id,o.name,d.name,d.menu_kind,d.status,d.brief,d.trial_notes,
		d.target_price,d.target_cost_pct,COALESCE(d.recipe_master_id,''),COALESCE(r.name,''),COALESCE(d.product_id,''),COALESCE(p.name,''),
		d.version,d.created_by,d.updated_by,d.approved_by,d.updated_at::text
		FROM fnb_developments d JOIN outlets o ON o.id=d.outlet_id LEFT JOIN recipe_masters r ON r.id=d.recipe_master_id
		LEFT JOIN cloud_products p ON p.id=d.product_id WHERE ($1 OR d.outlet_id=ANY($2)) ORDER BY d.updated_at DESC,d.id`, filter == nil, pq.Array(filter))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.FnbDevelopment{}
	for rows.Next() {
		var d models.FnbDevelopment
		if err := rows.Scan(&d.ID, &d.OutletID, &d.OutletName, &d.Name, &d.MenuKind, &d.Status, &d.Brief, &d.TrialNotes,
			&d.TargetPrice, &d.TargetCostPct, &d.RecipeID, &d.RecipeName, &d.ProductID, &d.ProductName, &d.Version, &d.CreatedBy, &d.UpdatedBy, &d.ApprovedBy, &d.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func fnbCheckReferences(tx *sql.Tx, d models.FnbDevelopment, scope []string) error {
	if _, err := fnbScope(d.OutletID, scope); err != nil {
		return err
	}
	var ok bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM outlets WHERE id=$1 AND is_active)`, d.OutletID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return Invalid("pilih outlet yang aktif")
	}
	if d.RecipeID != "" {
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM recipe_masters r WHERE id=$1 AND (visibility='public'
			OR EXISTS(SELECT 1 FROM recipe_outlet_access a WHERE a.recipe_master_id=r.id AND a.outlet_id=$2))
			AND EXISTS(SELECT 1 FROM recipe_items i WHERE i.recipe_master_id=r.id))`, d.RecipeID, d.OutletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return Invalid("resep tidak tersedia untuk outlet ini atau belum berisi bahan")
		}
	}
	if d.ProductID != "" {
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cloud_products WHERE id=$1 AND outlet_id=$2 AND NOT is_deleted)`, d.ProductID, d.OutletID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return Invalid("produk harus aktif dan berasal dari outlet yang dipilih")
		}
	}
	return nil
}

func SaveFnbDevelopment(id string, d models.FnbDevelopment, scope []string, actor string) (string, error) {
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || len(d.Name) > 200 || d.OutletID == "" || !validMenuKind(d.MenuKind, false) {
		return "", Invalid("nama, outlet, dan kategori makanan/minuman wajib diisi")
	}
	if !fnbFinite(d.TargetPrice) || d.TargetPrice < 0 || !fnbFinite(d.TargetCostPct) || d.TargetCostPct <= 0 || d.TargetCostPct > 100 {
		return "", Invalid("harga target dan target HPP tidak valid")
	}
	if len(d.Brief) > 20000 || len(d.TrialNotes) > 20000 {
		return "", Invalid("catatan terlalu panjang")
	}
	tx, err := database.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if id != "" {
		var oid, status string
		var version int
		if err := tx.QueryRow(`SELECT outlet_id,status,version FROM fnb_developments WHERE id=$1 FOR UPDATE`, id).Scan(&oid, &status, &version); err != nil {
			return "", err
		}
		if _, err := fnbScope(oid, scope); err != nil {
			return "", err
		}
		if oid != d.OutletID {
			return "", Invalid("outlet pengembangan tidak dapat diganti")
		}
		if version != d.Version {
			return "", Conflict("pengembangan telah berubah; muat ulang sebelum menyimpan")
		}
		if status == "approved" || status == "launched" || status == "archived" {
			return "", Invalid("kembalikan ke uji coba sebelum mengubah produk yang disetujui")
		}
	}
	if err := fnbCheckReferences(tx, d, scope); err != nil {
		return "", err
	}
	if id == "" {
		id = NewULID()
		_, err = tx.Exec(`INSERT INTO fnb_developments(id,outlet_id,name,menu_kind,brief,trial_notes,target_price,target_cost_pct,recipe_master_id,product_id,created_by,updated_by)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$11)`, id, d.OutletID, d.Name, d.MenuKind, d.Brief, d.TrialNotes, d.TargetPrice, d.TargetCostPct, nilIfEmpty(d.RecipeID), nilIfEmpty(d.ProductID), actor)
	} else {
		_, err = tx.Exec(`UPDATE fnb_developments SET name=$2,menu_kind=$3,brief=$4,trial_notes=$5,target_price=$6,target_cost_pct=$7,recipe_master_id=$8,product_id=$9,
			status=CASE WHEN status='review' THEN 'trial' ELSE status END,version=version+1,updated_by=$10,updated_at=NOW() WHERE id=$1`,
			id, d.Name, d.MenuKind, d.Brief, d.TrialNotes, d.TargetPrice, d.TargetCostPct, nilIfEmpty(d.RecipeID), nilIfEmpty(d.ProductID), actor)
	}
	if err != nil {
		return "", err
	}
	return id, tx.Commit()
}

func TransitionFnbDevelopment(id, status string, version int, scope []string, actor string, canApprove bool) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var d models.FnbDevelopment
	if err := tx.QueryRow(`SELECT outlet_id,status,version,target_price,trial_notes,COALESCE(recipe_master_id,''),COALESCE(product_id,'') FROM fnb_developments WHERE id=$1 FOR UPDATE`, id).
		Scan(&d.OutletID, &d.Status, &d.Version, &d.TargetPrice, &d.TrialNotes, &d.RecipeID, &d.ProductID); err != nil {
		return err
	}
	if _, err := fnbScope(d.OutletID, scope); err != nil {
		return err
	}
	if version != d.Version {
		return Conflict("status pengembangan telah berubah; muat ulang halaman")
	}
	allowed := map[string][]string{"idea": {"trial", "archived"}, "trial": {"review", "archived"}, "review": {"trial", "approved", "archived"}, "approved": {"trial", "launched", "archived"}, "launched": {"archived"}}
	ok := false
	for _, next := range allowed[d.Status] {
		if status == next {
			ok = true
		}
	}
	if !ok {
		return Invalid("perubahan status %s ke %s tidak diizinkan", d.Status, status)
	}
	if status == "approved" && !canApprove {
		return ErrFnbScope
	}
	if status == "review" || status == "approved" {
		if strings.TrimSpace(d.TrialNotes) == "" || d.TargetPrice <= 0 {
			return Invalid("lengkapi hasil uji coba dan harga target sebelum review")
		}
	}
	if status == "launched" {
		if d.ProductID == "" {
			return Invalid("tautkan produk POS sebelum mengajukan review untuk peluncuran")
		}
		if err := fnbCheckReferences(tx, d, scope); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`UPDATE fnb_developments SET status=$2::varchar,version=version+1,updated_by=$3::varchar,
		approved_by=CASE WHEN $2::varchar='approved' THEN $3::varchar WHEN $2::varchar='trial' THEN '' ELSE approved_by END,updated_at=NOW() WHERE id=$1`, id, status, actor)
	if err != nil {
		return err
	}
	return tx.Commit()
}

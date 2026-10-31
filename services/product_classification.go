package services

import (
	"cloud-pos/database"
	"cloud-pos/internal/productclassification"
	"cloud-pos/models"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lib/pq"
)

const MaxProductClassificationApply = 1000

type classificationQuerier interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

// The effective view is shared with normal product/F&B reads. Recipe evidence
// participates in the fingerprint but is not exposed by products.view, which
// does not grant access to private recipe names or composition.
func classificationInputs(q classificationQuerier, filter, ids []string) ([]models.ProductClassificationInput, error) {
	rows, err := q.Query(`SELECT p.id,p.outlet_id,o.name,p.name,COALESCE(p.category_name,''),
		COALESCE(p.description,''),COALESCE(k.menu_kind,''),COALESCE(k.menu_kind_source,''),p.updated_at,
		COALESCE((SELECT jsonb_agg(jsonb_build_object('recipe_id',rm.id,'name',rm.name,'menu_kind',rm.menu_kind,'position',ps.position)
			ORDER BY ps.position,rm.id) FROM product_recipe_sources ps JOIN recipe_masters rm ON rm.id=ps.recipe_master_id
			WHERE ps.product_id=p.id),'[]'::jsonb)::text
		FROM cloud_products p JOIN outlets o ON o.id=p.outlet_id
		LEFT JOIN product_menu_kinds k ON k.product_id=p.id
		WHERE NOT p.is_deleted AND ($1 OR p.outlet_id=ANY($2::text[])) AND ($3 OR p.id=ANY($4::text[]))
		ORDER BY o.name,p.name,p.id`, filter == nil, pq.Array(filter), ids == nil, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	inputs := []models.ProductClassificationInput{}
	for rows.Next() {
		var input models.ProductClassificationInput
		var updated time.Time
		var recipes string
		if err := rows.Scan(&input.ProductID, &input.OutletID, &input.OutletName, &input.Name, &input.CategoryName,
			&input.Description, &input.CurrentKind, &input.MenuKindSource, &updated, &recipes); err != nil {
			return nil, err
		}
		input.ProductID, input.OutletID = strings.TrimSpace(input.ProductID), strings.TrimSpace(input.OutletID)
		input.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
		if err := json.Unmarshal([]byte(recipes), &input.RecipeEvidence); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
	}
	return inputs, rows.Err()
}

func PreviewProductClassification(outlet string, scope []string) (models.ProductClassificationReport, error) {
	filter, err := fnbScope(outlet, scope)
	if err != nil {
		return models.ProductClassificationReport{}, err
	}
	inputs, err := classificationInputs(database.DB, filter, nil)
	if err != nil {
		return models.ProductClassificationReport{}, err
	}
	return productclassification.Report(inputs), nil
}

func ValidateProductClassificationApply(items []models.ApplyProductClassificationItem) error {
	if len(items) == 0 || len(items) > MaxProductClassificationApply {
		return Invalid("pilih 1–%d produk untuk diterapkan", MaxProductClassificationApply)
	}
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if item.ProductID == "" || len(item.ProductID) > 26 || strings.TrimSpace(item.ProductID) != item.ProductID || seen[item.ProductID] {
			return Invalid("ID produk kosong, tidak valid, atau duplikat")
		}
		seen[item.ProductID] = true
		if len(item.Fingerprint) != 64 {
			return Invalid("fingerprint pratinjau tidak valid")
		}
		if _, err := hex.DecodeString(item.Fingerprint); err != nil {
			return Invalid("fingerprint pratinjau tidak valid")
		}
		if item.SuggestedKind != "food" && item.SuggestedKind != "beverage" && item.SuggestedKind != "activity" && item.SuggestedKind != "other" {
			return Invalid("jenis usulan tidak valid")
		}
	}
	return nil
}

func ApplyProductClassification(outlet string, scope []string, actor string, items []models.ApplyProductClassificationItem) (int, error) {
	filter, err := fnbScope(outlet, scope)
	if err != nil {
		return 0, err
	}
	if err := ValidateProductClassificationApply(items); err != nil {
		return 0, err
	}
	if strings.TrimSpace(actor) == "" || len(actor) > 100 {
		return 0, Invalid("identitas pelaku tidak valid")
	}
	applied, err := applyProductClassification(filter, actor, items)
	var pg *pq.Error
	if errors.As(err, &pg) && (pg.Code == "40001" || pg.Code == "40P01") {
		return 0, Conflict("katalog berubah saat diterapkan; muat ulang pratinjau")
	}
	return applied, err
}

func applyProductClassification(filter []string, actor string, items []models.ApplyProductClassificationItem) (int, error) {
	tx, err := database.DB.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	ids := make([]string, 0, len(items))
	wanted := make(map[string]models.ApplyProductClassificationItem, len(items))
	for _, item := range items {
		ids = append(ids, item.ProductID)
		wanted[item.ProductID] = item
	}
	// All product writers acquire this same row lock before changing a profile.
	// Lock in ID order so overlapping bulk selections cannot invert lock order.
	rows, err := tx.Query(`SELECT id,outlet_id,is_deleted FROM cloud_products
		WHERE id=ANY($1::text[]) ORDER BY id FOR UPDATE`, pq.Array(ids))
	if err != nil {
		return 0, err
	}
	locked := 0
	for rows.Next() {
		var id, outlet string
		var deleted bool
		if err := rows.Scan(&id, &outlet, &deleted); err != nil {
			rows.Close()
			return 0, err
		}
		if !classificationOutletAllowed(strings.TrimSpace(outlet), filter) {
			rows.Close()
			return 0, ErrFnbScope
		}
		if deleted {
			rows.Close()
			return 0, Conflict("produk telah dihapus; muat ulang pratinjau")
		}
		locked++
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if locked != len(items) {
		return 0, Conflict("produk tidak lagi tersedia; muat ulang pratinjau")
	}
	inputs, err := classificationInputs(tx, filter, ids)
	if err != nil {
		return 0, err
	}
	if len(inputs) != len(items) {
		return 0, Conflict("katalog berubah; muat ulang pratinjau")
	}
	// Validate the entire selection before changing any row.
	for _, input := range inputs {
		row := productclassification.Classify(input)
		item := wanted[input.ProductID]
		if row.Status != "suggested" || row.Confidence != "high" || row.CurrentKind != "" ||
			row.Fingerprint != item.Fingerprint || row.SuggestedKind != item.SuggestedKind {
			return 0, Conflict("usulan berubah atau tidak dapat diterapkan; muat ulang pratinjau")
		}
	}
	for _, input := range inputs {
		item := wanted[input.ProductID]
		// Only the kind and audit fields are written: all existing F&B target,
		// packaging and overhead costs remain intact.
		result, err := tx.Exec(`INSERT INTO fnb_product_profiles(product_id,menu_kind,updated_by,updated_at)
			VALUES($1,$2,$3,NOW()) ON CONFLICT(product_id) DO UPDATE
			SET menu_kind=EXCLUDED.menu_kind,updated_by=EXCLUDED.updated_by,updated_at=NOW()
			WHERE fnb_product_profiles.menu_kind=''`, input.ProductID, item.SuggestedKind, actor)
		if err != nil {
			return 0, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		if count != 1 {
			return 0, Conflict("jenis produk telah diubah; muat ulang pratinjau")
		}
		if _, err := tx.Exec(`UPDATE cloud_products SET updated_at=clock_timestamp(),version=version+1 WHERE id=$1`, input.ProductID); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(items), nil
}

func classificationOutletAllowed(outlet string, filter []string) bool {
	if filter == nil {
		return true
	}
	for _, id := range filter {
		if id == outlet {
			return true
		}
	}
	return false
}

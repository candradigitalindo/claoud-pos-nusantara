package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"math"
	"testing"
)

func TestStockItemRecipeRejectsInvalidComponents(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []models.StockItemRecipeItem
	}{
		{"self", []models.StockItemRecipeItem{{ItemID: "result", QtyBase: 1}}},
		{"padded-self", []models.StockItemRecipeItem{{ItemID: " result ", QtyBase: 1}}},
		{"duplicate", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: 1}, {ItemID: "raw", QtyBase: 2}}},
		{"padded-duplicate", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: 1}, {ItemID: " raw ", QtyBase: 2}}},
		{"missing", []models.StockItemRecipeItem{{QtyBase: 1}}},
		{"zero", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: 0}}},
		{"negative", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: -1}}},
		{"unrepresentable", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: 0.000001}}},
		{"nan", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: math.NaN()}}},
		{"infinity", []models.StockItemRecipeItem{{ItemID: "raw", QtyBase: math.Inf(1)}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := SaveStockItemRecipes(models.StockItemRecipeRequest{ParentItemID: "result", Visibility: "public", Items: tc.items}); err == nil {
				t.Fatal("invalid formula accepted")
			}
		})
	}
}

func TestStockItemRecipeInvalidReplacementPreservesFormula(t *testing.T) {
	requireDB(t)
	f := newBreakdownFixture(t)
	raw := f.item(t, "Bahan resep", "g", "g", 1)
	result := f.item(t, "Hasil resep", "ml", "liter", 1000)
	valid := models.StockItemRecipeRequest{ParentItemID: result, Visibility: "public", Items: []models.StockItemRecipeItem{{ItemID: raw, QtyBase: 2}}}
	if err := SaveStockItemRecipes(valid); err != nil {
		t.Fatal(err)
	}
	// A missing foreign-key target fails after the replacement transaction
	// starts; the old definition must still survive the rollback.
	invalid := valid
	invalid.Items = []models.StockItemRecipeItem{{ItemID: NewULID(), QtyBase: 1}}
	if err := SaveStockItemRecipes(invalid); err == nil {
		t.Fatal("missing component accepted")
	}
	rows, err := GetStockItemRecipes(result)
	if err != nil || len(rows) != 1 || rows[0].ChildItemID != raw || rows[0].QtyBase != 2 {
		t.Fatalf("old definition lost: %+v, %v", rows, err)
	}
	if err := SaveStockItemRecipes(models.StockItemRecipeRequest{ParentItemID: result}); err != nil {
		t.Fatalf("explicit formula removal: %v", err)
	}
	var count int
	if err := database.DB.QueryRow(`SELECT COUNT(*) FROM stock_item_recipes WHERE parent_item_id=$1`, result).Scan(&count); err != nil || count != 0 {
		t.Fatalf("formula was not cleared: %d %v", count, err)
	}
}

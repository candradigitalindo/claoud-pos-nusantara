package productclassification

import (
	"cloud-pos/models"
	"testing"
)

func TestClassifyConservativeMenuKinds(t *testing.T) {
	for _, test := range []struct {
		name, category, kind string
	}{
		{"Nasi Goreng", "", "food"}, {"Steak", "", "food"}, {"Air Mineral", "", "beverage"},
		{"Es Krim Vanilla", "", "food"}, {"Ice Cream", "Dessert", "food"}, {"Pannacota", "", "food"},
		{"Donut", "", "food"}, {"Chocolate", "DESERT", "food"}, {"Iced Chocolate", "DESERT", ""},
		{"Eclair Matcha", "Dessert", "food"}, {"Donut Mochi Matcha", "Donut", "food"}, {"Melted Cheese Cake Matcha", "Dessert", "food"},
		{"Donut + Matcha", "Donut", ""}, {"Coffee Cake", "Dessert", ""}, {"Hot Chocolate", "Parfait & Ice Cream", ""},
		{"ESPRESSO10L", "Custom Payment", "beverage"}, {"Vanilla Latte", "Custom Payment", "beverage"},
		{"Chocolate Latte", "Baverage", "beverage"}, {"Nama Kreasi", "COFFE", "beverage"},
		{"Nama Kreasi", "Maincourse", "food"}, {"Nama Kreasi", "", ""},
		{"Jus Alpukat", "", "beverage"}, {"PS4 1 jam", "", "activity"}, {"Tiket Masuk", "", "activity"},
		{"GOLD ATV", "ATV", "activity"}, {"Dokumentasi", "ATV", "activity"}, {"VIP", "TIKET ACARA", "activity"},
		{"Jas Hujan", "ATV", ""}, {"Custom Payment", "", ""}, {"Lainnya", "Event", ""},
		{"Kopi Event", "Event", "beverage"}, {"Sate Event", "Event", "food"},
		{"BUNDLING GOLD ATV (2KOPI-2DONAT)", "ATV", ""}, {"BUNDLING GOLD ATV", "ATV", ""},
		{"Paket Nasi + Ice Tea", "", ""}, {"Black Tea + Pizza", "", ""},
		{"Ayam Bakar + Banana Split + Es Kreasi", "Paket", ""}, {"Udang Bakar + Banana Split + Es Kreasi", "Paket", ""},
		{"Nasi + Ayam", "Makanan", "food"}, {"Fish & Chip", "Makanan", "food"}, {"Coffee + Tea", "Minuman", "beverage"},
		{"Dango", "ADD ON MINUMAN", ""}, {"Pie", "ADD ON MINUMAN", ""}, {"Marshmello", "ADD ON MINUMAN", ""},
		{"Vanilla", "ADD ON MINUMAN", ""}, {"Es Batu", "ADD ON MINUMAN", "beverage"}, {"Sambal", "ADD ON MAKANAN", "food"},
		{"1 SOFT DRINK + 1 SNACK", "Paket Acara", ""}, {"4 FOOD + 4 SOFT DRINK", "Paket Acara", ""},
		{"1 BEER", "Paket Acara", "beverage"}, {"4 SOJU", "Paket Acara", "beverage"},
		{"2KOPI-2DONAT", "", ""}, {"AYAMLATTE", "Karyawan", ""}, {"MIELATTE", "Karyawan", ""},
		{"Ayam Jasmine", "Karyawan", ""}, {"Mie Jasmine", "Karyawan", ""},
		{"Ayam Amer", "Karyawan", ""}, {"Mie Amer", "Karyawan", ""},
		{"Es Buah", "Minuman", ""}, {"Sop Buah", "", ""},
		{"Latte", "Makanan", ""}, {"Cake Coffee", "", ""}, {"Moringa", "Signature", ""},
		{"TST", "Tradisional", ""}, {"Mineral", "", ""}, {"Team Building", "", ""},
	} {
		t.Run(test.name+"/"+test.category, func(t *testing.T) {
			row := Classify(models.ProductClassificationInput{Name: test.name, CategoryName: test.category})
			if row.SuggestedKind != test.kind {
				t.Fatalf("suggestion=%q want=%q: %+v", row.SuggestedKind, test.kind, row)
			}
			if test.kind == "" && (row.Status != "needs_review" || row.Confidence != "none") {
				t.Fatalf("uncertain result can be applied: %+v", row)
			}
			if test.kind != "" && (row.Status != "suggested" || row.Confidence != "high") {
				t.Fatalf("clear result lacks high-confidence suggestion: %+v", row)
			}
		})
	}
}

func TestClassifyPreservesLabelsAndWeakEvidence(t *testing.T) {
	for _, kind := range []string{"food", "beverage", "activity"} {
		row := Classify(models.ProductClassificationInput{Name: "Kopi Nasi ATV", CurrentKind: kind})
		if row.Status != "existing" || row.CurrentKind != kind || row.SuggestedKind != "" {
			t.Fatalf("existing label overwritten: %+v", row)
		}
	}
	for _, input := range []models.ProductClassificationInput{
		{Name: "Kreasi", Description: "kopi"},
		{Name: "Nasi", CurrentKind: "legacy"},
		{Name: "Nasi", MenuKindSource: "profile", RecipeEvidence: []models.ProductClassificationRecipe{{MenuKind: "beverage"}}},
	} {
		if row := Classify(input); row.Status != "needs_review" || row.SuggestedKind != "" {
			t.Fatalf("weak/contradictory evidence became automatic: %+v", row)
		}
	}
}

func TestClassificationFingerprintBindsRelevantInputs(t *testing.T) {
	base := models.ProductClassificationInput{ProductID: "product", OutletID: "outlet", Name: "Nasi", CategoryName: "Food", UpdatedAt: "2026-01-01T00:00:00Z",
		RecipeEvidence: []models.ProductClassificationRecipe{{RecipeID: "two", Position: 1}, {RecipeID: "one", Position: 0}}}
	fingerprint := Fingerprint(base)
	reordered := base
	reordered.RecipeEvidence = []models.ProductClassificationRecipe{base.RecipeEvidence[1], base.RecipeEvidence[0]}
	if Fingerprint(reordered) != fingerprint {
		t.Fatal("recipe array order changed fingerprint")
	}
	for _, change := range []func(*models.ProductClassificationInput){
		func(p *models.ProductClassificationInput) { p.Name = "Jus" },
		func(p *models.ProductClassificationInput) { p.CategoryName = "Minuman" },
		func(p *models.ProductClassificationInput) { p.Description = "new evidence" },
		func(p *models.ProductClassificationInput) { p.CurrentKind = "activity" },
		func(p *models.ProductClassificationInput) { p.MenuKindSource = "profile" },
		func(p *models.ProductClassificationInput) { p.UpdatedAt = "2026-01-02T00:00:00Z" },
		func(p *models.ProductClassificationInput) { p.OutletID = "another" },
		func(p *models.ProductClassificationInput) {
			p.RecipeEvidence = []models.ProductClassificationRecipe{{RecipeID: "one", MenuKind: "food"}}
		},
	} {
		changed := base
		change(&changed)
		if Fingerprint(changed) == fingerprint {
			t.Fatal("changed classification evidence retained fingerprint")
		}
	}
	report := Report([]models.ProductClassificationInput{{Name: "Nasi"}, {CurrentKind: "beverage"}, {Name: "ATV"}, {Name: "Unknown"}})
	if report.Totals != (models.ProductClassificationTotals{Total: 4, Existing: 1, Suggested: 2, NeedsReview: 1, Food: 1, Beverage: 1, Activity: 1}) {
		t.Fatalf("incorrect review totals: %+v", report.Totals)
	}
}

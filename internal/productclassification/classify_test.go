package productclassification

import (
	"cloud-pos/models"
	"strings"
	"testing"
)

func TestClassifyConservativeMenuKinds(t *testing.T) {
	for _, test := range []struct {
		name, category, kind string
	}{
		{"Nasi Goreng", "", "food"}, {"Steak", "", "food"}, {"Air Mineral", "", "beverage"},
		{"Es Krim Vanilla", "", "food"}, {"Ice Cream", "Dessert", "food"}, {"Pannacota", "", "food"},
		{"Donut", "", "food"}, {"Chocolate", "DESERT", "food"}, {"Iced Chocolate", "DESERT", "other"},
		{"Eclair Matcha", "Dessert", "food"}, {"Donut Mochi Matcha", "Donut", "food"}, {"Melted Cheese Cake Matcha", "Dessert", "food"},
		{"Donut + Matcha", "Donut", "other"}, {"Coffee Cake", "Dessert", "other"}, {"Hot Chocolate", "Parfait & Ice Cream", "other"},
		{"ESPRESSO10L", "Custom Payment", "beverage"}, {"Vanilla Latte", "Custom Payment", "beverage"},
		{"Chocolate Latte", "Baverage", "beverage"}, {"Nama Kreasi", "COFFE", "beverage"},
		{"Nama Kreasi", "Maincourse", "food"}, {"Nama Kreasi", "", "other"},
		{"Jus Alpukat", "", "beverage"}, {"PS4 1 jam", "", "activity"}, {"Tiket Masuk", "", "activity"},
		{"GOLD ATV", "ATV", "activity"}, {"Dokumentasi", "ATV", "activity"}, {"VIP", "TIKET ACARA", "activity"},
		{"Jas Hujan", "ATV", "other"}, {"Custom Payment", "", "other"}, {"Lainnya", "Event", "other"},
		{"Kopi Event", "Event", "beverage"}, {"Sate Event", "Event", "food"},
		{"BUNDLING GOLD ATV (2KOPI-2DONAT)", "ATV", "other"}, {"BUNDLING GOLD ATV", "ATV", "other"},
		{"Paket Nasi + Ice Tea", "", "other"}, {"Black Tea + Pizza", "", "other"},
		{"Ayam Bakar + Banana Split + Es Kreasi", "Paket", "other"}, {"Udang Bakar + Banana Split + Es Kreasi", "Paket", "other"},
		{"Nasi + Ayam", "Makanan", "food"}, {"Fish & Chip", "Makanan", "food"}, {"Coffee + Tea", "Minuman", "beverage"},
		{"Dango", "ADD ON MINUMAN", "other"}, {"Pie", "ADD ON MINUMAN", "other"}, {"Marshmello", "ADD ON MINUMAN", "other"},
		{"Vanilla", "ADD ON MINUMAN", "other"}, {"Es Batu", "ADD ON MINUMAN", "beverage"}, {"Sambal", "ADD ON MAKANAN", "food"},
		{"1 SOFT DRINK + 1 SNACK", "Paket Acara", "other"}, {"4 FOOD + 4 SOFT DRINK", "Paket Acara", "other"},
		{"1 BEER", "Paket Acara", "beverage"}, {"4 SOJU", "Paket Acara", "beverage"},
		{"2KOPI-2DONAT", "", "other"}, {"AYAMLATTE", "Karyawan", "other"}, {"MIELATTE", "Karyawan", "other"},
		{"Ayam Jasmine", "Karyawan", "other"}, {"Mie Jasmine", "Karyawan", "other"},
		{"Ayam Amer", "Karyawan", "other"}, {"Mie Amer", "Karyawan", "other"},
		{"Es Buah", "Minuman", "other"}, {"Sop Buah", "", "other"},
		{"Latte", "Makanan", "other"}, {"Cake Coffee", "", "other"}, {"Moringa", "Signature", "other"},
		{"TST", "Tradisional", "other"}, {"Mineral", "", "other"}, {"Team Building", "", "other"},
	} {
		t.Run(test.name+"/"+test.category, func(t *testing.T) {
			row := Classify(models.ProductClassificationInput{Name: test.name, CategoryName: test.category})
			if row.SuggestedKind != test.kind {
				t.Fatalf("suggestion=%q want=%q: %+v", row.SuggestedKind, test.kind, row)
			}
			if row.Status != "suggested" || row.Confidence != "high" {
				t.Fatalf("result lacks an applicable suggestion: %+v", row)
			}
			if test.kind == "other" && !strings.Contains(row.Reason, "Ketentuan Lainnya") {
				t.Fatalf("fallback reason must explain the owner's grouping policy: %+v", row)
			}
		})
	}
}

func TestClassifyPreservesLabelsAndWeakEvidence(t *testing.T) {
	for _, kind := range []string{"food", "beverage", "activity", "other"} {
		row := Classify(models.ProductClassificationInput{Name: "Kopi Nasi ATV", CurrentKind: kind})
		if row.Status != "existing" || row.CurrentKind != kind || row.SuggestedKind != "" {
			t.Fatalf("existing label overwritten: %+v", row)
		}
	}
	for _, input := range []models.ProductClassificationInput{
		{Name: "Kreasi", Description: "kopi"},
		{Name: "Nasi", MenuKindSource: "profile", RecipeEvidence: []models.ProductClassificationRecipe{{MenuKind: "beverage"}}},
	} {
		if row := Classify(input); row.Status != "suggested" || row.SuggestedKind != "other" || row.Confidence != "high" {
			t.Fatalf("unclassified evidence did not follow the Lainnya policy: %+v", row)
		}
	}
	row := Classify(models.ProductClassificationInput{Name: "Nasi", CurrentKind: "legacy"})
	if row.Status != "needs_review" || row.SuggestedKind != "" || row.CurrentKind != "legacy" {
		t.Fatalf("unknown existing kind was overwritten by fallback: %+v", row)
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
		func(p *models.ProductClassificationInput) { p.OutletName = "ATV Overland" },
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
	if report.Totals != (models.ProductClassificationTotals{Total: 4, Existing: 1, Suggested: 3, Food: 1, Beverage: 1, Activity: 1, Other: 1}) {
		t.Fatalf("incorrect review totals: %+v", report.Totals)
	}
}

func TestClassifyATVOverlandOutletPolicy(t *testing.T) {
	for _, name := range []string{"Dokumentasi", "GOLD ATV", "Jas Hujan", "SILVER ATV", "Nasi Goreng", "Espresso", "BUNDLING GOLD ATV (2KOPI-2DONAT)", "Produk baru"} {
		row := Classify(models.ProductClassificationInput{OutletName: "ATV Overland", Name: name, CategoryName: "ATV KALA"})
		if row.Status != "suggested" || row.SuggestedKind != "activity" || row.Confidence != "high" {
			t.Fatalf("outlet policy did not cover %q: %+v", name, row)
		}
	}
	for _, outlet := range []string{"ATV", "ATV Overland Cafe", "Cabang ATV Overland", "Kala Mont View", ""} {
		row := Classify(models.ProductClassificationInput{OutletName: outlet, Name: "Jas Hujan", CategoryName: "ATV KALA"})
		if row.Status != "suggested" || row.SuggestedKind != "other" {
			t.Fatalf("outlet policy leaked to %q: %+v", outlet, row)
		}
	}
	row := Classify(models.ProductClassificationInput{OutletName: "  atv   OVERLAND  ", Name: "Jas Hujan"})
	if row.SuggestedKind != "activity" {
		t.Fatalf("case/whitespace variation lost policy: %+v", row)
	}
	for _, kind := range []string{"food", "beverage", "activity", "other"} {
		row := Classify(models.ProductClassificationInput{OutletName: "ATV Overland", Name: "Jas Hujan", CurrentKind: kind})
		if row.Status != "existing" || row.CurrentKind != kind || row.SuggestedKind != "" {
			t.Fatalf("preview silently replaced an existing label: %+v", row)
		}
	}
}

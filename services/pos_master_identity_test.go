package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"testing"
)

func TestPOSMasterIDsStayScopedAndRoundTripToLocalCategories(t *testing.T) {
	setupDB(t)
	a, _ := seedOutlet(t)
	b, _ := seedOutlet(t)
	categoryLocal, productLocal, addonLocal, chargeLocal := NewULID(), NewULID(), NewULID(), NewULID()
	t.Cleanup(func() {
		for _, outlet := range []string{a, b} {
			database.DB.Exec(`DELETE FROM cloud_product_addons WHERE outlet_id=$1`, outlet)
			database.DB.Exec(`DELETE FROM cloud_products WHERE outlet_id=$1`, outlet)
			database.DB.Exec(`DELETE FROM cloud_categories WHERE outlet_id=$1`, outlet)
		}
	})
	productIDs := map[string]string{}
	for _, outlet := range []string{a, b} {
		// Product arrives before its category, so the parent identity is reserved.
		p := models.PushProductRequest{LocalID: productLocal, Name: "Menu bersama", CategoryID: categoryLocal, CategoryName: "Minuman", Price: 12000}
		productID, err := SaveProduct(outlet, p)
		if err != nil {
			t.Fatal(err)
		}
		productIDs[outlet] = productID
		catID, err := SaveCategory(outlet, models.PushCategoryRequest{LocalID: categoryLocal, Name: "Minuman"})
		if err != nil {
			t.Fatal(err)
		}
		var storedCategory string
		if err := database.DB.QueryRow(`SELECT category_id FROM cloud_products WHERE id=$1`, productID).Scan(&storedCategory); err != nil || storedCategory != catID {
			t.Fatalf("parent reservation was lost: %s != %s (%v)", storedCategory, catID, err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			got, err := SaveProduct(outlet, p)
			if err != nil || got != productID {
				t.Fatalf("product retry changed canonical ID: %s %v", got, err)
			}
			if _, err := SaveProductAddon(outlet, models.PushProductAddonRequest{LocalID: addonLocal, ProductLocalID: productLocal, Name: "Extra", Price: 1000, IsActive: 1}); err != nil {
				t.Fatal(err)
			}
			if _, err := SaveAdditionalCharge(outlet, models.PushAdditionalChargeRequest{LocalID: chargeLocal, Name: "Kemasan", ChargeType: "fixed", Value: 500, IsActive: 1}); err != nil {
				t.Fatal(err)
			}
		}
		products, _, err := GetProducts(outlet, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, p := range products {
			if p.LocalID == productLocal {
				found = true
				if p.ID != productID || p.CategoryID != categoryLocal {
					t.Fatalf("full pull must return local category ID: %+v", p)
				}
			}
		}
		if !found {
			t.Fatal("product missing from full pull")
		}
		updates, err := GetUpdatesSince(outlet, "2020-01-01T00:00:00Z")
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range updates.Products {
			if p.LocalID == productLocal && p.CategoryID != categoryLocal {
				t.Fatalf("incremental pull leaked canonical category ID: %+v", p)
			}
		}
		for _, table := range []string{"cloud_product_addons", "cloud_additional_charges"} {
			localID := addonLocal
			if table == "cloud_additional_charges" {
				localID = chargeLocal
			}
			var count int
			if err := database.DB.QueryRow(`SELECT count(*) FROM `+table+` WHERE outlet_id=$1 AND local_id=$2`, outlet, localID).Scan(&count); err != nil || count != 1 {
				t.Fatalf("%s duplicated/lost on retry: %d %v", table, count, err)
			}
		}
	}
	if productIDs[a] == productIDs[b] {
		t.Fatal("two outlets share a canonical product ID")
	}
}

func TestPOSMasterPreservesHistoricalCanonicalProductID(t *testing.T) {
	setupDB(t)
	outlet, _ := seedOutlet(t)
	localID, canonical := produkID(outlet), NewULID()
	if _, err := database.DB.Exec(`UPDATE cloud_products SET id=$1 WHERE outlet_id=$2 AND local_id=$3`, canonical, outlet, localID); err != nil {
		t.Fatal(err)
	}
	got, err := SaveProduct(outlet, models.PushProductRequest{LocalID: localID, Name: "Nama diperbarui", Price: 27000})
	if err != nil || got != canonical {
		t.Fatalf("historical canonical product identity rewritten: %s %v", got, err)
	}
}

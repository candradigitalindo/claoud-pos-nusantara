package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"sort"
	"testing"
)

func TestPOSCatalogPaginationOrdersDuplicateNamesByStableID(t *testing.T) {
	setupDB(t)
	outlet, _ := seedOutlet(t)
	if _, err := database.DB.Exec(`DELETE FROM cloud_products WHERE outlet_id=$1`, outlet); err != nil {
		t.Fatal(err)
	}
	ids := []string{NewULID(), NewULID(), NewULID(), NewULID()}
	// Insert in reverse identity order. A name-only sort can vary with heap order.
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	for _, id := range ids {
		if _, err := SaveProduct(outlet, models.PushProductRequest{LocalID: id, Name: "Nama sama", Price: 10000}); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(ids)
	for page, want := range ids {
		got, total, err := GetProducts(outlet, page+1, 1)
		if err != nil || total != len(ids) || len(got) != 1 {
			t.Fatalf("page %d total %d rows %d: %v", page+1, total, len(got), err)
		}
		if got[0].ID != want {
			t.Fatalf("page %d: got %s want %s", page+1, got[0].ID, want)
		}
	}
}

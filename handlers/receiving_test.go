package handlers

// Uji handler serah terima & katalog stok: wewenang penerimaan per role
// (T-10, T-17), cakupan dokumen pusat (T-43), dan katalog global hanya untuk
// pengguna yang berhak atas Gudang Induk (T-20). Butuh Postgres uji
// (TestMain di purchase_test.go).

import (
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func receivingTestApp(locals map[string]interface{}) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		for k, v := range locals {
			c.Locals(k, v)
		}
		return c.Next()
	})
	app.Get("/pr/:id/receiving-draft", GetReceivingDraft)
	app.Post("/pr/:id/receive-goods", ReceiveGoods)
	app.Post("/stock-items", CreateStockItem)
	app.Put("/stock-items/:id", UpdateStockItem)
	app.Post("/stock-items/:id/toggle", ToggleStockItemActive)
	app.Delete("/stock-items/:id", DeleteStockItem)
	return app
}

// testRole membuat role sementara berizin perms.
func testRole(t *testing.T, perms ...string) string {
	t.Helper()
	role := "uji_terima_" + strings.ToLower(services.NewULID()[16:])
	for _, p := range perms {
		if _, err := database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1, $2)`, role, p); err != nil {
			t.Fatalf("izin uji: %v", err)
		}
	}
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, role) })
	return role
}

// approvedTestPR: pengajuan pusat (tanpa outlet) yang sudah disetujui.
func approvedTestPR(t *testing.T, reqType, itemName string, price float64) *models.PurchaseRequest {
	t.Helper()
	pr, err := services.CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: reqType, RequestedBy: "pengaju uji",
		Items: []models.PurchaseRequestItem{{Name: "Uji", Items: []models.PurchaseSubItem{
			{Name: itemName, Qty: 1, Unit: "pcs", HpsPrice: price, FinalPrice: price},
		}}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM pr_receiving_decisions WHERE purchase_request_id = $1`, pr.ID)
		database.DB.Exec(`DELETE FROM handover_photos WHERE ref_id = $1`, pr.ID)
		database.DB.Exec(`DELETE FROM purchase_requests WHERE id = $1`, pr.ID)
	})
	// Bahan baku lahir 'verified' (tanpa tahap verifikasi).
	if pr.Status == "pending" {
		if _, err := services.UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "verify", ActorName: "v", Version: pr.Version}); err != nil {
			t.Fatalf("verify: %v", err)
		}
	}
	if _, err := services.UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "a"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	return pr
}

func TestReceivingAccessByRole(t *testing.T) {
	requireHandlerDB(t)
	sfx := strings.ToLower(services.NewULID()[16:])
	barang := approvedTestPR(t, services.RequestTypeBarang, "Kipas Uji Handler "+sfx, 200000)
	bahan := approvedTestPR(t, services.RequestTypeBahan, "Tepung Uji Handler "+sfx, 9000)
	jasa := approvedTestPR(t, services.RequestTypeJasa, "Servis Uji Handler "+sfx, 50000)
	sess := func(role string) *fiber.App {
		return receivingTestApp(map[string]interface{}{
			"admin_id": services.NewULID(), "admin_username": "petugas_" + sfx, "admin_role": role, "admin_scope": "all",
		})
	}

	aset := sess(testRole(t, "assets.create", "assets.view"))
	if code, res := doJSON(t, aset, "GET", "/pr/"+barang.ID+"/receiving-draft", nil); code != fiber.StatusOK {
		t.Fatalf("bagian Aset membuka draf barang: %d %q (T-10)", code, res.Error)
	}
	if code, _ := doJSON(t, aset, "GET", "/pr/"+bahan.ID+"/receiving-draft", nil); code != fiber.StatusForbidden {
		t.Errorf("bagian Aset membuka draf bahan baku: %d, mau 403", code)
	}
	if code, _ := doJSON(t, aset, "GET", "/pr/"+jasa.ID+"/receiving-draft", nil); code != fiber.StatusBadRequest {
		t.Errorf("draf serah terima jasa: %d, mau 400", code)
	}
	d, err := services.BuildReceivingDraft(barang.ID, nil)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	line := d.Lines[0]
	body := map[string]interface{}{"photo_url": "/uploads/uji-handler.jpg", "lines": []map[string]interface{}{{
		"pr_item_key": line.PRItemKey, "destination": "habis", "qty": 1, "reason": "langsung dipakai di dapur",
	}}}

	gudang := sess(testRole(t, "stockledger.adjust"))
	if code, res := doJSON(t, gudang, "POST", "/pr/"+barang.ID+"/receive-goods", body); code != fiber.StatusBadRequest ||
		!strings.Contains(res.Error, "bagian Aset") {
		t.Errorf("petugas gudang mencatat peralatan: %d %q, mau 400 per baris (T-17)", code, res.Error)
	}
	lain := sess(testRole(t, "procurement.rawmaterials.submit"))
	if code, _ := doJSON(t, lain, "POST", "/pr/"+barang.ID+"/receive-goods", body); code != fiber.StatusForbidden {
		t.Errorf("pengaju bahan baku menerima pengadaan barang: %d, mau 403", code)
	}
	if code, res := doJSON(t, aset, "POST", "/pr/"+barang.ID+"/receive-goods", body); code != fiber.StatusOK {
		t.Fatalf("bagian Aset mencatat peralatan: %d %q", code, res.Error)
	}
	var by string
	database.DB.QueryRow(`SELECT COALESCE(received_by, '') FROM purchase_requests WHERE id = $1`, barang.ID).Scan(&by)
	if by != "petugas_"+sfx {
		t.Errorf("received_by = %q, mau pelaku sesi (T-7)", by)
	}
}

// T-43: role per outlet tidak bisa membuka/menerima pengajuan pusat (tanpa outlet).
func TestReceivingCentralPRNeedsScopeAll(t *testing.T) {
	requireHandlerDB(t)
	pr := approvedTestPR(t, services.RequestTypeBarang, "Rak Uji Pusat "+strings.ToLower(services.NewULID()[16:]), 100000)
	app := receivingTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "manajer_outlet", "admin_role": testRole(t, "procurement.requests.submit"),
		"admin_scope": "specific", "admin_outlet_ids": []string{"01KOUTLETLAINUJI0000000000"}, "admin_work_unit_ids": []string{},
	})
	if code, _ := doJSON(t, app, "GET", "/pr/"+pr.ID+"/receiving-draft", nil); code != fiber.StatusForbidden {
		t.Errorf("role per outlet membuka draf pengajuan pusat: %d, mau 403 (T-43)", code)
	}
}

// T-20: katalog global hanya untuk role bercakupan semua outlet; role per
// outlet hanya kolom per gudang outletnya sendiri.
func TestCatalogRequiresCentralScope(t *testing.T) {
	requireHandlerDB(t)
	sfx := strings.ToLower(services.NewULID()[16:])
	outlet, wh := services.NewULID(), services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
		outlet, "KT"+sfx[:8], "Outlet Uji Katalog "+sfx, "key-katalog-"+outlet); err != nil {
		t.Fatalf("outlet uji: %v", err)
	}
	if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type, outlet_id) VALUES ($1, $2, 'Gudang Outlet Uji', 'outlet', $3)`,
		wh, "WK"+strings.ToUpper(sfx), outlet); err != nil {
		t.Fatalf("gudang uji: %v", err)
	}
	item, err := services.CreateStockItem(models.StockItemRequest{Name: "Saus Uji Katalog " + sfx, BaseUnit: "ml"}, "uji")
	if err != nil {
		t.Fatalf("item uji: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM stock_ledger WHERE item_id = $1`, item.ID)
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1 OR name LIKE $2`, item.ID, "%Uji Katalog Baru "+sfx)
		database.DB.Exec(`DELETE FROM warehouses WHERE id = $1`, wh)
		database.DB.Exec(`DELETE FROM outlets WHERE id = $1`, outlet)
	})
	role := testRole(t, "stockitems.create", "stockitems.update", "stockitems.delete", "stockitems.view")
	scoped := receivingTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "manajer_" + sfx, "admin_role": role,
		"admin_scope": "specific", "admin_outlet_ids": []string{outlet}, "admin_work_unit_ids": []string{},
	})
	if code, _ := doJSON(t, scoped, "POST", "/stock-items", map[string]interface{}{"name": "Item Uji Katalog Baru " + sfx}); code != fiber.StatusForbidden {
		t.Errorf("role per outlet membuat item katalog: %d, mau 403 (T-20)", code)
	}
	if code, _ := doJSON(t, scoped, "PUT", "/stock-items/"+item.ID, map[string]interface{}{"name": "Ganti Nama"}); code != fiber.StatusForbidden {
		t.Errorf("role per outlet mengubah katalog tanpa gudang: %d, mau 403 (T-20)", code)
	}
	if code, _ := doJSON(t, scoped, "POST", "/stock-items/"+item.ID+"/toggle", nil); code != fiber.StatusForbidden {
		t.Errorf("role per outlet menonaktifkan item katalog: %d, mau 403 (T-20)", code)
	}
	if code, _ := doJSON(t, scoped, "DELETE", "/stock-items/"+item.ID, nil); code != fiber.StatusForbidden {
		t.Errorf("role per outlet menghapus item katalog: %d, mau 403", code)
	}
	// Dari gudang outletnya sendiri: hanya stok minimum per gudang, katalog tetap.
	code, res := doJSON(t, scoped, "PUT", "/stock-items/"+item.ID, map[string]interface{}{
		"name": "Ganti Nama", "warehouse_id": wh, "warehouse_min_stock": 7,
	})
	if code != fiber.StatusOK {
		t.Fatalf("role per outlet mengubah stok minimum gudangnya: %d %q", code, res.Error)
	}
	after, _ := services.GetStockItem(item.ID)
	if after.Name != item.Name {
		t.Errorf("nama katalog berubah dari konteks outlet: %q", after.Name)
	}
	all := receivingTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "gudang_induk", "admin_role": role, "admin_scope": "all",
	})
	if code, res := doJSON(t, all, "POST", "/stock-items", map[string]interface{}{"name": "Item Uji Katalog Baru " + sfx}); code != fiber.StatusCreated {
		t.Errorf("Gudang Induk membuat item katalog: %d %q", code, res.Error)
	}
}

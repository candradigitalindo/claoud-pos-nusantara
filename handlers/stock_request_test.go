package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

// Permintaan stok outlet ke Gudang Induk: outlet membuat transfer Gudang Induk
// → gudang outletnya, Gudang Induk (role semua outlet) menyetujui dan mengirim,
// outlet menerima. Role outlet yang kebetulan memegang izin setujui/kirim
// tetap tidak boleh menjalankan tahap atas nama Gudang Induk.

const stockFlowPhoto = "/uploads/2026-10/01UJIKIRIMGUDANGINDUK00000.jpg"

// stockFlowFixture: outlet A & B dengan gudang outletnya, dua Gudang Induk,
// satu item (gr, distribusi kg) bersaldo 10 kg di Gudang Induk C.
type stockFlowFixture struct {
	outletA, outletB          string
	whA, whB, whC, whC2, item string
}

func newStockFlowFixture(t *testing.T) *stockFlowFixture {
	t.Helper()
	sfx := strings.ToLower(services.NewULID()[16:])
	up := strings.ToUpper(sfx)
	f := &stockFlowFixture{outletA: services.NewULID(), outletB: services.NewULID(),
		whA: services.NewULID(), whB: services.NewULID(), whC: services.NewULID(), whC2: services.NewULID(), item: services.NewULID()}
	whs := []string{f.whA, f.whB, f.whC, f.whC2}
	t.Cleanup(func() {
		for _, w := range whs {
			tr := `(SELECT id FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $1)`
			database.DB.Exec(`DELETE FROM stock_quality_cases WHERE warehouse_id=$1 OR return_warehouse_id=$1`, w)
			database.DB.Exec(`DELETE FROM stock_transfer_receipts WHERE transfer_id IN `+tr, w)
			database.DB.Exec(`DELETE FROM stock_evidence WHERE warehouse_id=$1`, w)
			database.DB.Exec(`DELETE FROM handover_photos WHERE ref_type = 'stock_transfer' AND ref_id IN `+tr, w)
			database.DB.Exec(`DELETE FROM stock_transfer_items WHERE transfer_id IN `+tr, w)
			database.DB.Exec(`DELETE FROM stock_transfers WHERE from_warehouse_id = $1 OR to_warehouse_id = $1`, w)
			database.DB.Exec(`DELETE FROM stock_breakdown_outputs WHERE breakdown_id IN (SELECT id FROM stock_breakdowns WHERE warehouse_id = $1)`, w)
			database.DB.Exec(`DELETE FROM stock_breakdowns WHERE warehouse_id = $1`, w)
			database.DB.Exec(`DELETE FROM stock_wastes WHERE warehouse_id = $1`, w)
			database.DB.Exec(`DELETE FROM stock_batches WHERE warehouse_id = $1`, w)
			database.DB.Exec(`DELETE FROM stock_movements WHERE warehouse_id = $1`, w)
			database.DB.Exec(`DELETE FROM stock_ledger WHERE warehouse_id = $1`, w)
		}
		database.DB.Exec(`DELETE FROM stock_items WHERE id = $1`, f.item)
		for _, w := range whs {
			database.DB.Exec(`DELETE FROM warehouses WHERE id = $1`, w)
		}
		database.DB.Exec(`DELETE FROM outlets WHERE id IN ($1, $2)`, f.outletA, f.outletB)
	})
	for i, o := range []string{f.outletA, f.outletB} {
		if _, err := database.DB.Exec(`INSERT INTO outlets (id, code, name, api_key) VALUES ($1, $2, $3, $4)`,
			o, string(rune('A'+i))+"T"+sfx[:8], "Outlet Uji Minta "+string(rune('A'+i))+" "+sfx, "key-minta-"+o); err != nil {
			t.Fatalf("outlet uji: %v", err)
		}
	}
	for _, w := range []struct{ id, code, name, typ, outlet string }{
		{f.whA, "WRA" + up, "Gudang Uji A " + sfx, "outlet", f.outletA},
		{f.whB, "WRB" + up, "Gudang Uji B " + sfx, "outlet", f.outletB},
		{f.whC, "WRC" + up, "Gudang Induk Uji " + sfx, "central", ""},
		{f.whC2, "WRD" + up, "Gudang Induk Uji Dua " + sfx, "central", ""},
	} {
		var outlet interface{}
		if w.outlet != "" {
			outlet = w.outlet
		}
		if _, err := database.DB.Exec(`INSERT INTO warehouses (id, code, name, type, outlet_id) VALUES ($1, $2, $3, $4, $5)`,
			w.id, w.code, w.name, w.typ, outlet); err != nil {
			t.Fatalf("gudang uji: %v", err)
		}
	}
	if _, err := database.DB.Exec(`INSERT INTO stock_items (id, code, name, base_unit, dist_unit, dist_ratio) VALUES ($1, $2, $3, 'gr', 'kg', 1000)`,
		f.item, "BRS"+up, "Beras Uji Minta "+sfx); err != nil {
		t.Fatalf("item uji: %v", err)
	}
	if err := services.CreateAdjustment(models.AdjustmentRequest{ItemID: f.item, WarehouseID: f.whC, QtyBase: 10000, CostPerBase: 12, MovType: "purchase_in"}, "uji"); err != nil {
		t.Fatalf("stok Gudang Induk: %v", err)
	}
	return f
}

func stockFlowTestApp(locals map[string]interface{}) *fiber.App {
	a := fiber.New()
	a.Use(func(c *fiber.Ctx) error {
		for k, v := range locals {
			c.Locals(k, v)
		}
		return c.Next()
	})
	a.Get("/warehouses", ListWarehouses)
	a.Post("/stock-transfers", CreateStockTransfer)
	a.Put("/stock-transfers/:id/status", UpdateTransferStatus)
	a.Post("/stock-transfers/:id/receive", ReceiveStockTransfer)
	a.Post("/stock-evidence", UploadStockEvidence)
	a.Put("/stock-transfers/:id/items/:itemId/received", UpdateReceivedQty)
	a.Put("/stock-items/:id/breakdown", SaveStockItemBreakdown)
	a.Post("/stock-breakdowns", CreateStockBreakdown)
	a.Get("/stock-breakdowns", ListStockBreakdowns)
	return a
}

// stockFlowUsers: role per outlet (outlet A) dan role semua outlet; keduanya
// memegang SEMUA izin transfer — yang membedakan hanya cakupannya.
func stockFlowUsers(t *testing.T, f *stockFlowFixture, perms ...string) (outlet, central *fiber.App) {
	t.Helper()
	if len(perms) == 0 {
		perms = []string{"stocktransfers.view", "stocktransfers.create", "stocktransfers.update", "stocktransfers.approve", "stocktransfers.receive"}
	}
	role := testRole(t, perms...)
	outlet = stockFlowTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "kepala_dapur_a", "admin_role": role,
		"admin_scope": "specific", "admin_outlet_ids": []string{f.outletA}, "admin_work_unit_ids": []string{},
	})
	central = stockFlowTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "gudang_induk", "admin_role": role, "admin_scope": "all",
	})
	return outlet, central
}

func decodeData(t *testing.T, res models.APIResponse, into interface{}) {
	t.Helper()
	raw, _ := json.Marshal(res.Data)
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

func ledgerQty(itemID, warehouseID string) float64 {
	var q float64
	database.DB.QueryRow(`SELECT qty_base FROM stock_ledger WHERE item_id = $1 AND warehouse_id = $2`, itemID, warehouseID).Scan(&q)
	return q
}

func createTransfer(t *testing.T, app *fiber.App, from, to, item string, qtyDist float64) (int, models.StockTransfer, string) {
	t.Helper()
	code, res := doJSON(t, app, "POST", "/stock-transfers", map[string]interface{}{
		"from_warehouse_id": from, "to_warehouse_id": to, "notes": "uji minta stok",
		"items": []map[string]interface{}{{"item_id": item, "qty_dist": qtyDist}},
	})
	var tr models.StockTransfer
	if code == fiber.StatusCreated {
		decodeData(t, res, &tr)
	}
	return code, tr, res.Error
}

func setTransferStatus(t *testing.T, app *fiber.App, id, status, photo string) (int, string) {
	t.Helper()
	code, res := doJSON(t, app, "PUT", "/stock-transfers/"+id+"/status", map[string]interface{}{"status": status, "photo_url": photo})
	return code, res.Error
}

// Daftar gudang: Gudang Induk hanya ikut bila diminta (form transfer), dan
// gudang outlet lain tidak pernah ikut.
func TestWarehouseListIncludeCentralOptIn(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	outlet, central := stockFlowUsers(t, f, "warehouses.view")
	ids := func(app *fiber.App, url string) map[string]models.Warehouse {
		code, res := doJSON(t, app, "GET", url, nil)
		if code != fiber.StatusOK {
			t.Fatalf("GET %s: %d %q", url, code, res.Error)
		}
		var list []models.Warehouse
		decodeData(t, res, &list)
		m := map[string]models.Warehouse{}
		for _, w := range list {
			m[w.ID] = w
		}
		return m
	}
	plain := ids(outlet, "/warehouses?limit=1000")
	if _, ok := plain[f.whA]; !ok || len(plain) != 1 {
		t.Errorf("daftar biasa role outlet: %d gudang, mau hanya gudang outlet A", len(plain))
	}
	withCentral := ids(outlet, "/warehouses?limit=1000&include_central=1")
	for _, want := range []string{f.whA, f.whC, f.whC2} {
		if _, ok := withCentral[want]; !ok {
			t.Errorf("include_central: gudang %s tidak muncul", want)
		}
	}
	if _, ok := withCentral[f.whB]; ok {
		t.Errorf("include_central: gudang outlet B ikut muncul")
	}
	for _, w := range withCentral {
		if w.Type != "central" && w.OutletID != f.outletA {
			t.Errorf("include_central: gudang %s (%s) di luar cakupan", w.Name, w.Type)
		}
	}
	// Gudang Induk nonaktif tidak ditawarkan sebagai asal permintaan.
	database.DB.Exec(`UPDATE warehouses SET is_active = false WHERE id = $1`, f.whC2)
	if _, ok := ids(outlet, "/warehouses?limit=1000&include_central=1")[f.whC2]; ok {
		t.Errorf("Gudang Induk nonaktif ikut ditawarkan")
	}
	// Role semua outlet: tidak berubah, semua gudang tampil.
	all := ids(central, "/warehouses?limit=1000")
	for _, want := range []string{f.whA, f.whB, f.whC} {
		if _, ok := all[want]; !ok {
			t.Errorf("role semua outlet: gudang %s tidak muncul", want)
		}
	}
}

// Alur penuh: outlet meminta → Gudang Induk menyetujui & mengirim → outlet
// menerima. Tahap Gudang Induk tertolak bagi role outlet walau izinnya ada.
func TestOutletRequestsStockFromCentral(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	outlet, central := stockFlowUsers(t, f)

	code, tr, msg := createTransfer(t, outlet, f.whC, f.whA, f.item, 2)
	if code != fiber.StatusCreated {
		t.Fatalf("outlet meminta Gudang Induk → gudangnya: %d %q, mau 201", code, msg)
	}
	if tr.FromWarehouseType != "central" || tr.ToWarehouseType != "outlet" || tr.Status != "draft" {
		t.Errorf("transfer baru: asal %q tujuan %q status %q", tr.FromWarehouseType, tr.ToWarehouseType, tr.Status)
	}
	if code, _, _ := createTransfer(t, outlet, f.whC, f.whB, f.item, 1); code != fiber.StatusForbidden {
		t.Errorf("outlet membuat Gudang Induk → outlet lain: %d, mau 403", code)
	}
	if code, _, _ := createTransfer(t, outlet, f.whC, f.whC2, f.item, 1); code != fiber.StatusForbidden {
		t.Errorf("outlet membuat Gudang Induk → Gudang Induk: %d, mau 403", code)
	}

	if code, _ := setTransferStatus(t, outlet, tr.ID, "approved", ""); code != fiber.StatusForbidden {
		t.Errorf("outlet menyetujui permintaannya sendiri: %d, mau 403", code)
	}
	if code, msg := setTransferStatus(t, central, tr.ID, "approved", ""); code != fiber.StatusOK {
		t.Fatalf("Gudang Induk menyetujui: %d %q", code, msg)
	}
	if code, _ := setTransferStatus(t, outlet, tr.ID, "cancelled", ""); code != fiber.StatusForbidden {
		t.Errorf("outlet membatalkan permintaan yang sudah disetujui: %d, mau 403", code)
	}
	if code, _ := setTransferStatus(t, outlet, tr.ID, "sent", stockFlowPhoto); code != fiber.StatusForbidden {
		t.Errorf("outlet menandai terkirim atas nama Gudang Induk: %d, mau 403", code)
	}
	if got := ledgerQty(f.item, f.whC); got != 10000 {
		t.Errorf("stok Gudang Induk berubah sebelum dikirim: %.2f", got)
	}
	if code, msg := setTransferStatus(t, central, tr.ID, "sent", stockFlowPhoto); code != fiber.StatusOK {
		t.Fatalf("Gudang Induk mengirim: %d %q", code, msg)
	}
	if got := ledgerQty(f.item, f.whC); got != 8000 {
		t.Errorf("stok Gudang Induk setelah kirim %.2f, mau 8000", got)
	}
	code, res := doJSON(t, outlet, "PUT", "/stock-transfers/"+tr.ID+"/items/"+f.item+"/received", map[string]interface{}{"received_qty_base": 2000})
	if code != fiber.StatusOK {
		t.Errorf("outlet mengisi jumlah diterima: %d %q", code, res.Error)
	}
	if code, _ := setTransferStatus(t, outlet, tr.ID, "received", ""); code != fiber.StatusBadRequest {
		t.Errorf("penerimaan lama tanpa bukti foto masih diterima: %d", code)
	}
	if code, msg := receiveTransferWithPhoto(t, outlet, tr.ID); code != fiber.StatusOK {
		t.Fatalf("outlet menerima: %d %q", code, msg)
	}
	if got := ledgerQty(f.item, f.whA); got != 2000 {
		t.Errorf("stok gudang outlet A setelah terima %.2f, mau 2000", got)
	}
}

// Outlet boleh menarik permintaannya selagi masih draft.
func TestOutletCancelsDraftRequest(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	outlet, _ := stockFlowUsers(t, f)
	code, tr, msg := createTransfer(t, outlet, f.whC, f.whA, f.item, 1)
	if code != fiber.StatusCreated {
		t.Fatalf("buat permintaan: %d %q", code, msg)
	}
	if code, msg := setTransferStatus(t, outlet, tr.ID, "cancelled", ""); code != fiber.StatusOK {
		t.Errorf("outlet membatalkan draft permintaannya: %d %q, mau 200", code, msg)
	}
	var status string
	database.DB.QueryRow(`SELECT status FROM stock_transfers WHERE id = $1`, tr.ID).Scan(&status)
	if status != "cancelled" {
		t.Errorf("status %q, mau cancelled", status)
	}
}

// Kebalikannya: barang dari outlet ke Gudang Induk dikirim outlet, tetapi
// penerimaannya di Gudang Induk dicatat Gudang Induk.
func TestReceivingIntoCentralRequiresAllOutletRole(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	outlet, central := stockFlowUsers(t, f)
	if err := services.CreateAdjustment(models.AdjustmentRequest{ItemID: f.item, WarehouseID: f.whA, QtyBase: 3000, CostPerBase: 12, MovType: "purchase_in"}, "uji"); err != nil {
		t.Fatalf("stok outlet: %v", err)
	}
	code, tr, msg := createTransfer(t, outlet, f.whA, f.whC, f.item, 1)
	if code != fiber.StatusCreated {
		t.Fatalf("outlet → Gudang Induk: %d %q", code, msg)
	}
	if code, msg := setTransferStatus(t, outlet, tr.ID, "approved", ""); code != fiber.StatusOK {
		t.Fatalf("outlet menyetujui kiriman dari gudangnya: %d %q", code, msg)
	}
	if code, msg := setTransferStatus(t, outlet, tr.ID, "sent", stockFlowPhoto); code != fiber.StatusOK {
		t.Fatalf("outlet mengirim dari gudangnya: %d %q", code, msg)
	}
	if code, _ := doJSON(t, outlet, "PUT", "/stock-transfers/"+tr.ID+"/items/"+f.item+"/received", map[string]interface{}{"received_qty_base": 1000}); code != fiber.StatusForbidden {
		t.Errorf("outlet mengisi jumlah diterima di Gudang Induk: %d, mau 403", code)
	}
	if code, _ := setTransferStatus(t, outlet, tr.ID, "received", ""); code != fiber.StatusForbidden {
		t.Errorf("outlet menerima atas nama Gudang Induk: %d, mau 403", code)
	}
	if code, msg := receiveTransferWithPhoto(t, central, tr.ID); code != fiber.StatusOK {
		t.Errorf("Gudang Induk menerima: %d %q", code, msg)
	}
	if got := ledgerQty(f.item, f.whC); got != 11000 {
		t.Errorf("stok Gudang Induk %.2f, mau 11000", got)
	}
}

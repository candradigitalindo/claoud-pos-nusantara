package handlers

// Uji regresi peninjauan akhir lewat HTTP: pengaju yang mengirim harga final
// dan nomor invoice di body — saat membuat maupun mengubah pengajuan pending —
// tidak pernah tersimpan (T-39). Hanya pembelian yang mengisinya setelah
// pengajuan disetujui.

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func TestRequesterFinalPriceIgnoredOverHTTP(t *testing.T) {
	requireHandlerDB(t)
	unit := newHandlerTestUnit(t)
	role := "uji_pengaju_t39_" + strings.ToLower(services.NewULID()[18:])
	for _, p := range []string{"procurement.requests.view", "procurement.requests.submit"} {
		if _, err := database.DB.Exec(`INSERT INTO role_permissions (role, permission) VALUES ($1, $2)`, role, p); err != nil {
			t.Fatalf("izin uji: %v", err)
		}
	}
	t.Cleanup(func() { database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, role) })
	app := purchaseTestApp(map[string]interface{}{
		"admin_id": services.NewULID(), "admin_username": "pengaju_t39", "admin_role": role, "admin_scope": "all",
	})

	body := func(final float64) []map[string]interface{} {
		return []map[string]interface{}{{
			"name": "Servis", "final_total": final * 2,
			"items": []map[string]interface{}{{
				"name": "Cuci AC uji T-39", "qty": 2, "unit": "unit", "hps_price": 150000,
				"final_price": final, "final_subtotal": final * 2,
			}},
		}}
	}
	code, res := doJSON(t, app, "POST", "/pr", map[string]interface{}{
		"request_type": "jasa", "work_unit_id": unit, "invoice_number": "INV-PENGAJU-1",
		"vendor_name": "CV Saran Pengaju", "items": body(120000),
	})
	if code != fiber.StatusCreated {
		t.Fatalf("create: %d %q", code, res.Error)
	}
	raw, _ := json.Marshal(res.Data)
	var pr models.PurchaseRequest
	json.Unmarshal(raw, &pr)

	stored := func(stage string) {
		t.Helper()
		var totalFinal, totalAmount float64
		var invoice string
		var itemsJSON []byte
		if err := database.DB.QueryRow(`SELECT total_final, total_amount, invoice_number, items FROM purchase_requests WHERE id = $1`, pr.ID).
			Scan(&totalFinal, &totalAmount, &invoice, &itemsJSON); err != nil {
			t.Fatalf("%s: baca pengajuan: %v", stage, err)
		}
		var items []models.PurchaseRequestItem
		json.Unmarshal(itemsJSON, &items)
		if totalFinal != 0 || totalAmount != 300000 || invoice != "" ||
			len(items) != 1 || items[0].Items[0].FinalPrice != 0 || items[0].Items[0].FinalSubtotal != 0 || items[0].FinalTotal != 0 {
			t.Errorf("%s: tersimpan total_final=%v total_amount=%v invoice=%q items=%s — harga final & invoice pengaju harus diabaikan",
				stage, totalFinal, totalAmount, invoice, itemsJSON)
		}
	}
	stored("POST pengajuan")

	code, res = doJSON(t, app, "PUT", "/pr/"+pr.ID+"/items", map[string]interface{}{
		"items": body(110000), "invoice_number": "INV-PENGAJU-2",
	})
	if code != fiber.StatusOK {
		t.Fatalf("ubah pending: %d %q", code, res.Error)
	}
	stored("PUT item pending")
}

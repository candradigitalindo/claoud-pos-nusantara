package routes

import (
	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"strings"
	"testing"
)

func TestFinanceEntryRoutesSeparateDuties(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	viewer, _ := ruteAkun(t, "fv"+suffix, []string{"finance.entries.view"})
	writer, _ := ruteAkun(t, "fw"+suffix, []string{"finance.entries.view", "finance.entries.create"})
	poster, _ := ruteAkun(t, "fp"+suffix, []string{"finance.entries.view", "finance.entries.post"})
	reverser, _ := ruteAkun(t, "fr"+suffix, []string{"finance.entries.view", "finance.entries.void"})
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	base := "/api/v1/admin/finance"
	input := models.FinanceEntryInput{RequestKey: services.NewULID(), Direction: "in", AccountCode: "4-200", Amount: 50000, OccurredAt: "2026-09-15", Counterparty: "Pelanggan uji", Description: "Pemasukan uji rute", Method: "cash", ProofURL: "/uploads/2026-09/test.png"}
	if code, _ := rutePanggil(t, app, "POST", base+"/entries", viewer, input); code != 403 {
		t.Fatalf("viewer create: %d", code)
	}
	code, res := rutePanggil(t, app, "POST", base+"/entries", writer, input)
	if code != 200 {
		t.Fatalf("writer create: %d %s", code, res.Error)
	}
	var entry models.FinanceEntry
	raw, _ := json.Marshal(res.Data)
	json.Unmarshal(raw, &entry)
	t.Cleanup(func() {
		ruteExec(`DELETE FROM finance_entry_events WHERE entry_id=$1`, entry.ID)
		ruteExec(`DELETE FROM finance_entries WHERE id=$1`, entry.ID)
	})
	actionURL := base + "/entries/" + entry.ID + "/actions"
	post := map[string]interface{}{"action": "post", "version": entry.Version}
	for _, token := range []string{viewer, writer, reverser} {
		if code, _ := rutePanggil(t, app, "POST", actionURL, token, post); code != 403 {
			t.Fatalf("unauthorized post: %d", code)
		}
	}
	if code, res = rutePanggil(t, app, "POST", actionURL, poster, post); code != 200 {
		t.Fatalf("post: %d %s", code, res.Error)
	}
	if code, _ = rutePanggil(t, app, "POST", actionURL, poster, post); code != 409 {
		t.Fatalf("repeat post: %d", code)
	}
	void := map[string]interface{}{"action": "void", "version": entry.Version + 1, "reason": "Salah pencatatan"}
	if code, _ = rutePanggil(t, app, "POST", actionURL, poster, void); code != 403 {
		t.Fatalf("poster void: %d", code)
	}
	if code, res = rutePanggil(t, app, "POST", actionURL, reverser, void); code != 200 {
		t.Fatalf("void: %d %s", code, res.Error)
	}
	for _, path := range []string{"/lookups", "/accounts", "/entries?date_from=2026-09-01&date_to=2026-09-30", "/entries/" + entry.ID, "/operations?date_from=2026-09-01&date_to=2026-09-30"} {
		if code, res = rutePanggil(t, app, "GET", base+path, viewer, nil); code != 200 {
			t.Fatalf("view %s: %d %s", path, code, res.Error)
		}
	}
}

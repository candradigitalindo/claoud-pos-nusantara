package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"errors"
	"testing"
)

func TestPOSClosedShiftRejectsStaleOpenAndDuplicateClose(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	opened := models.PushCashierShiftRequest{LocalID: NewULID(), OpenedBy: "Kasir awal", OpenedAt: "2026-10-21T02:00:00Z", OpeningCash: 100000, Status: "open"}
	id, err := SaveCashierShift(outlet, opened)
	if err != nil {
		t.Fatal(err)
	}
	closed := opened
	closed.ClosedAt, closed.ClosedBy, closed.Status = "2026-10-21T15:00:00Z", "Kasir akhir", "closed"
	closed.ClosingCash, closed.ClosingCard, closed.ClosingQris, closed.ClosingTransfer = 250000, 20000, 30000, 40000
	closed.CarryOverCash, closed.Notes = 250000, "Tutup harian"
	closed.Report = map[string]any{
		"sales_total": 240000, "expected_cash": 250000,
		"by_day":    []map[string]any{{"date": "2026-10-21", "sales_total": 240000}},
		"by_method": map[string]any{"cash": map[string]any{"count": 3, "total": 150000}},
	}
	if got, err := SaveCashierShift(outlet, closed); err != nil || got != id {
		t.Fatalf("close identity: %q %v", got, err)
	}
	readAccepted := func() []byte {
		t.Helper()
		var data string
		if err := database.DB.QueryRow(`SELECT (to_jsonb(s)-'synced_at')::text FROM cloud_cashier_shifts s WHERE id=$1`, id).Scan(&data); err != nil {
			t.Fatal(err)
		}
		return []byte(data)
	}
	accepted := readAccepted()
	stale := opened
	stale.OpenedAt, stale.OpeningCash, stale.Report = "2026-10-20T02:00:00Z", 0, nil
	duplicate := closed
	duplicate.OpenedAt, duplicate.ClosedAt, duplicate.Report = "", "", nil
	duplicate.ClosingCash, duplicate.CarryOverCash, duplicate.Notes = 0, 0, "old incomplete retry"
	for _, retry := range []models.PushCashierShiftRequest{stale, duplicate, closed} {
		if got, err := SaveCashierShift(outlet, retry); err != nil || got != id {
			t.Fatalf("retry identity: %q %v", got, err)
		}
		assertJSONValueEqual(t, accepted, readAccepted())
	}
	restored, err := GetRestoreData(outlet)
	if err != nil || len(restored.OpenShifts) != 0 || len(restored.CashMovements) != 0 {
		t.Fatalf("closed shift resurrected in restore: %+v %v", restored, err)
	}
}

func TestPOSRestoreExcludesLegacyClosedAtShiftAndMovements(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	old := models.PushCashierShiftRequest{LocalID: NewULID(), OpenedBy: "Old", OpenedAt: "2026-10-20T02:00:00Z", OpeningCash: 100, Status: "open"}
	oldID, err := SaveCashierShift(outlet, old)
	if err != nil {
		t.Fatal(err)
	}
	// Preserve an inconsistent historical row until a subsequent sync touches
	// that row; the restore read itself must not modify production history.
	if _, err := database.DB.Exec(`UPDATE cloud_cashier_shifts SET closed_at='2026-10-20 15:00:00',closing_cash=500,report='{"sales_total":400,"by_day":[{"date":"2026-10-20","sales_total":400}]}' WHERE id=$1`, oldID); err != nil {
		t.Fatal(err)
	}
	active := models.PushCashierShiftRequest{LocalID: NewULID(), OpenedBy: "Current", OpenedAt: "2026-10-21T02:00:00Z", OpeningCash: 200, Status: "open"}
	if _, err := SaveCashierShift(outlet, active); err != nil {
		t.Fatal(err)
	}
	for _, shift := range []models.PushCashierShiftRequest{old, active} {
		if _, err := SaveCashMovement(outlet, models.PushCashMovementRequest{LocalID: NewULID(), ShiftID: shift.LocalID, MovementType: "in", Amount: 10, CreatedAt: shift.OpenedAt}); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := GetRestoreData(outlet)
	if err != nil || len(restored.OpenShifts) != 1 || restored.OpenShifts[0].LocalID != active.LocalID || len(restored.CashMovements) != 1 || restored.CashMovements[0].ShiftID != active.LocalID {
		t.Fatalf("restore included terminal legacy shift or its movements: %+v %v", restored, err)
	}
	if got, err := SaveCashierShift(outlet, old); err != nil || got != oldID {
		t.Fatalf("legacy retry identity: %q %v", got, err)
	}
	var status, closedAt, report string
	var cash float64
	if err := database.DB.QueryRow(`SELECT status,closed_at::text,closing_cash,report::text FROM cloud_cashier_shifts WHERE id=$1`, oldID).Scan(&status, &closedAt, &cash, &report); err != nil {
		t.Fatal(err)
	}
	if status != "closed" || closedAt != "2026-10-20 15:00:00" || cash != 500 {
		t.Fatalf("legacy closure changed: %q %q %v", status, closedAt, cash)
	}
	assertJSONValueEqual(t, []byte(`{"sales_total":400,"by_day":[{"date":"2026-10-20","sales_total":400}]}`), []byte(report))
}

func TestPOSClosingTimestampIsTerminalWithoutInventingDates(t *testing.T) {
	outlet := newPOSRestoreOutlet(t)
	req := models.PushCashierShiftRequest{LocalID: NewULID(), OpenedBy: "Kasir", OpenedAt: "2026-10-21T02:00:00Z", ClosedAt: "not-a-time", Status: "open"}
	var invalid ValidationError
	if _, err := SaveCashierShift(outlet, req); !errors.As(err, &invalid) {
		t.Fatalf("invalid close timestamp accepted: %v", err)
	}
	req.ClosedAt = "2026-10-21T15:00:00Z"
	id, err := SaveCashierShift(outlet, req)
	if err != nil {
		t.Fatal(err)
	}
	var status string
	if err := database.DB.QueryRow(`SELECT status FROM cloud_cashier_shifts WHERE id=$1`, id).Scan(&status); err != nil || status != "closed" {
		t.Fatalf("closing timestamp did not make shift terminal: %q %v", status, err)
	}
}

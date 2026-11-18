package services

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

func TestPurchasingWorkbookSeparatesVendorsAndRetainsActualQuantities(t *testing.T) {
	vendorA, vendorB, master := "vendor-a", "vendor-b", "master"
	eight, zero := 8, 0
	pr := func(number, vendorID, name string, sub models.PurchaseSubItem) models.PurchaseRequest {
		return models.PurchaseRequest{
			RequestNumber: number, VendorID: &vendorID, VendorName: name,
			RequestType: RequestTypeBahan, Status: "approved", CreatedAt: "2026-11-09T18:00:00Z",
			Items: []models.PurchaseRequestItem{{Name: "Kebutuhan", Items: []models.PurchaseSubItem{sub}}},
		}
	}
	first := pr("PR-CHILD-A", vendorA, "CV Sumber/Utama", models.PurchaseSubItem{
		Name: "Tepung", Qty: 10, PurchasedQty: &eight, Unit: "kotak", HpsPrice: 10000, FinalPrice: 11000,
	})
	first.ParentID, first.ParentNumber = &master, "PR-MASTER"
	second := pr("PR-CHILD-B", vendorB, "CV Sumber:Utama", models.PurchaseSubItem{
		Name: "Gula", Qty: 1, PurchasedQty: &zero, Unit: "kotak", HpsPrice: 5000, FinalPrice: 10000,
	})
	second.ParentID, second.ParentNumber = &master, "PR-MASTER"
	legacy := pr("PR-LEGACY", vendorA, first.VendorName, models.PurchaseSubItem{
		Name: "Garam", Qty: 2, Unit: "bungkus", HpsPrice: 5000, FinalPrice: 6000,
	})
	remaining := pr("PR-PARTIAL-MASTER", vendorA, first.VendorName, models.PurchaseSubItem{
		Name: "Minyak", Qty: 3, Unit: "liter", HpsPrice: 2000, FinalPrice: 2500,
	})
	remaining.SplitStatus = &master
	remaining.TotalFinal, remaining.TotalHps = 999999, 999999 // rolled-up totals must never be counted
	emptyMaster := models.PurchaseRequest{RequestNumber: "PR-MASTER", SplitStatus: &master, TotalFinal: 999999}
	data, err := buildPurchasingWorkbook([]models.PurchaseRequest{emptyMaster, first, second, legacy, remaining}, time.Date(2026, 11, 10, 8, 0, 0, 0, time.FixedZone("WIB", 7*3600)))
	if err != nil {
		t.Fatalf("build XLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("open XLSX: %v", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) != 3 {
		t.Fatalf("sheets = %v, want summary and two vendors", sheets)
	}
	assertCell := func(sheet, cell, want string) {
		t.Helper()
		got, err := f.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
		if err != nil || got != want {
			t.Errorf("%s!%s = %q (%v), want %q", sheet, cell, got, err, want)
		}
	}
	a, _ := f.GetCellValue("Ringkasan", "B7")
	b, _ := f.GetCellValue("Ringkasan", "B8")
	if a == b || strings.ContainsAny(a+b, `[]:*?/\`) {
		t.Fatalf("vendor sheet collision or invalid names: %q, %q", a, b)
	}
	assertCell(a, "A4", "PR-CHILD-A")
	assertCell(a, "B4", "PR-MASTER")
	assertCell(a, "C4", "10 Nov 2026") // local date, not UTC date
	assertCell(a, "I4", "10")
	assertCell(a, "J4", "8")
	assertCell(a, "N4", "100000")
	assertCell(a, "O4", "88000")
	assertCell(a, "J5", "2") // legacy quantity defaults to the request
	assertCell(a, "A6", "PR-PARTIAL-MASTER")
	assertCell(a, "O7", "107500")
	assertCell(b, "J4", "0") // explicit zero must not fall back to the request
	assertCell(b, "O4", "0")
	assertCell("Ringkasan", "C7", "3")
	assertCell("Ringkasan", "E9", "121000")
	assertCell("Ringkasan", "F9", "107500")
}

func TestPurchasingSheetNamesHandleUnicodeReservedNamesAndCollisions(t *testing.T) {
	used := map[string]bool{"ringkasan": true}
	for _, name := range []string{"Ringkasan", "ringkasan", "'[]:*?/\\'", "", strings.Repeat("供給者", 30), strings.Repeat("供給者", 30), strings.Repeat("🏪", 30), strings.Repeat("🏪", 30), "vendor", "VENDOR", "a\x00b\nc"} {
		before := len(used)
		got := purchasingSheetName(name, used)
		if !utf8.ValidString(got) || utf8.RuneCountInString(got) > 31 || strings.TrimSpace(got) == "" || strings.ContainsAny(got, "[]:*?/\\\x00\n") || len(used) != before+1 {
			t.Errorf("unsafe or duplicate sheet name %q from %q", got, name)
		}
		f := excelize.NewFile()
		if _, err := f.NewSheet(got); err != nil {
			t.Errorf("Excel rejects generated name %q: %v", got, err)
		}
		f.Close()
	}
}

func TestPurchasingWorkbookKeepsDistinctVendorIDsWithSameName(t *testing.T) {
	a, b := "vendor-one", "vendor-two"
	requests := []models.PurchaseRequest{
		{VendorID: &a, VendorName: "Vendor Sama", Items: []models.PurchaseRequestItem{{Name: "A", Items: []models.PurchaseSubItem{{Name: "Satu", Qty: 1}}}}},
		{VendorID: &b, VendorName: "Vendor Sama", Items: []models.PurchaseRequestItem{{Name: "B", Items: []models.PurchaseSubItem{{Name: "Dua", Qty: 2}}}}},
	}
	data, err := buildPurchasingWorkbook(requests, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if got := f.GetSheetList(); len(got) != 3 || got[1] == got[2] {
		t.Errorf("separate vendor IDs merged: %v", got)
	}
}

func TestPurchasingWorkbookEmptyFilterIsStillDownloadable(t *testing.T) {
	data, err := buildPurchasingWorkbook(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if sheets := f.GetSheetList(); len(sheets) != 1 || sheets[0] != "Ringkasan" {
		t.Errorf("empty export sheets = %v", sheets)
	}
	message, _ := f.GetCellValue("Ringkasan", "A7")
	if !strings.Contains(message, "Tidak ada item") {
		t.Errorf("empty export message = %q", message)
	}
}

func TestPurchasingExportRespectsTypeAndLocationScopes(t *testing.T) {
	requireDB(t)
	search := "Purchasing scope " + NewULID()
	outletA, outletB := recvOutlet(t), recvOutlet(t)
	unit := func(outletID string) string {
		t.Helper()
		id := NewULID()
		if _, err := database.DB.Exec(`INSERT INTO work_units (id, name, outlet_id) VALUES ($1, $2, $3)`, id, search, outletID); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { execScratch(`DELETE FROM work_units WHERE id = $1`, id) })
		return id
	}
	unitA, unitB := unit(outletA), unit(outletB)
	items := []models.PurchaseRequestItem{{Name: search, Items: []models.PurchaseSubItem{
		{Name: search + " Tepung", Qty: 10, Unit: "kotak", HpsPrice: 1000, FinalPrice: 1100},
		{Name: search + " Gula", Qty: 2, Unit: "kotak", HpsPrice: 2000, FinalPrice: 2500},
	}}}
	raw := recvApprovedPR(t, RequestTypeBahan, items)
	service := recvApprovedPR(t, RequestTypeJasa, []models.PurchaseRequestItem{{Name: search, Items: []models.PurchaseSubItem{
		{Name: search + " Servis", Qty: 1, Unit: "kali", HpsPrice: 100, FinalPrice: 120},
	}}})
	for _, row := range []struct{ id, outlet, unit string }{{raw.ID, outletA, unitA}, {service.ID, outletB, unitB}} {
		if _, err := database.DB.Exec(`UPDATE purchase_requests SET outlet_id = $1, work_unit_id = $2 WHERE id = $3`, row.outlet, row.unit, row.id); err != nil {
			t.Fatal(err)
		}
	}
	vendorAName, vendorBName, vendorCName := "Vendor A "+search, "Vendor B "+search, "Vendor C "+search
	vendorA, vendorB, vendorC := testVendorNamed(t, vendorAName), testVendorNamed(t, vendorBName), testVendorNamed(t, vendorCName)
	eight := 8
	items[0].Items[0].PurchasedQty = &eight
	if _, err := UpdatePurchaseItems(raw.ID, models.UpdatePurchaseItemsInput{Items: items, VendorID: &vendorA}); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdatePurchaseItems(service.ID, models.UpdatePurchaseItemsInput{Items: service.Items, VendorID: &vendorC}); err != nil {
		t.Fatal(err)
	}
	split, err := SplitPurchaseRequest(raw.ID, models.SplitPurchaseRequestInput{
		VendorID: vendorB, Items: []models.PurchaseRequestItem{{Name: search, Items: []models.PurchaseSubItem{{Name: items[0].Items[0].Name}}}},
	})
	if err != nil || len(split.Children) != 1 {
		t.Fatalf("split: %v (%+v)", err, split)
	}
	for _, tc := range []struct {
		name, status, requestType, parentID, workUnitID, total string
		outletScope, unitScope, allowedTypes                   []string
		vendors                                                []string
	}{
		{name: "unrestricted", total: "13920", vendors: []string{vendorAName, vendorBName, vendorCName}},
		{name: "no permitted types", allowedTypes: []string{}},
		{name: "raw materials only", allowedTypes: []string{RequestTypeBahan}, total: "13800", vendors: []string{vendorAName, vendorBName}},
		{name: "type filter", requestType: RequestTypeJasa, total: "120", vendors: []string{vendorCName}},
		{name: "empty location scope", outletScope: []string{}, unitScope: []string{}, allowedTypes: []string{RequestTypeBahan}},
		{name: "foreign outlet scope", outletScope: []string{outletB}, allowedTypes: []string{RequestTypeBahan}},
		{name: "foreign work unit scope", outletScope: []string{}, unitScope: []string{unitB}, allowedTypes: []string{RequestTypeBahan}},
		{name: "owned work unit scope", outletScope: []string{}, unitScope: []string{unitA}, allowedTypes: []string{RequestTypeBahan}, total: "13800", vendors: []string{vendorAName, vendorBName}},
		{name: "owned outlet scope", outletScope: []string{outletA}, allowedTypes: []string{RequestTypeBahan}, total: "13800", vendors: []string{vendorAName, vendorBName}},
		{name: "work unit filter", workUnitID: unitB, allowedTypes: []string{RequestTypeBahan}},
		{name: "status filter", status: "verified", allowedTypes: []string{RequestTypeBahan}},
		{name: "selected split children", parentID: raw.ID, allowedTypes: []string{RequestTypeBahan}, total: "8800", vendors: []string{vendorBName}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, filename, err := BuildPurchasingExcel("", tc.workUnitID, tc.status, tc.requestType, "", tc.parentID, search, tc.outletScope, tc.unitScope, tc.allowedTypes)
			if err != nil {
				t.Fatalf("export: %v", err)
			}
			if !strings.HasSuffix(filename, ".xlsx") {
				t.Errorf("filename = %q", filename)
			}
			f, err := excelize.OpenReader(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			if sheets := f.GetSheetList(); len(sheets) != 1+len(tc.vendors) {
				t.Fatalf("sheets = %v, want %d vendor sheets", sheets, len(tc.vendors))
			}
			for i, want := range tc.vendors {
				cell, _ := excelize.CoordinatesToCellName(1, i+7)
				got, _ := f.GetCellValue("Ringkasan", cell)
				if got != want {
					t.Errorf("vendor = %q, want %q", got, want)
				}
			}
			if tc.total != "" {
				cell, _ := excelize.CoordinatesToCellName(6, 7+len(tc.vendors))
				got, _ := f.GetCellValue("Ringkasan", cell, excelize.Options{RawCellValue: true})
				if got != tc.total {
					t.Errorf("total = %q, want %q (split items counted once)", got, tc.total)
				}
			}
		})
	}
}

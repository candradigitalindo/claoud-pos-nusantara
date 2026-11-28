package services

import (
	"bytes"
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/xuri/excelize/v2"
)

func TestAllPermissionsCoversNewFeatures(t *testing.T) {
	permSet := map[string]bool{}
	for _, p := range AllPermissions {
		permSet[p] = true
	}
	required := []string{
		"bundles.view", "bundles.claim", "bundles.manage",
		"logistics.rawmaterials.view", "logistics.rawmaterials.manage",
		"logistics.goods.view", "logistics.goods.manage",
		"logistics.deliveries.view", "logistics.deliveries.manage",
		"finance.payments.view", "finance.payments.approve", "finance.payments.pay",
		"procurement.rawmaterials.view", "procurement.rawmaterials.purchasing",
		"procurement.requests.view", "procurement.requests.purchasing",
	}
	for _, req := range required {
		if !permSet[req] {
			t.Errorf("AllPermissions kehilangan izin wajib %q", req)
		}
	}
	for _, retired := range []string{"procurement.requests.pickup", "procurement.rawmaterials.pickup"} {
		if permSet[retired] {
			t.Errorf("AllPermissions masih memuat izin lama %q yang sudah dihapus", retired)
		}
	}
}

func TestPurchasingGroupSeparationAndFinanceOrigin(t *testing.T) {
	requireDB(t)
	ensureBundleTestMigrations(t)

	// Buat 1 outlet, 1 gudang, dan 4 tagihan terpisah: bahan, barang, jasa, manual.
	outletID := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets (id, name, code, api_key, is_active) VALUES ($1, $2, $3, $4, true)`,
		outletID, "Test Outlet Group Split", "TOG-"+outletID[20:], "key-"+outletID); err != nil {
		t.Fatalf("insert outlet: %v", err)
	}
	defer database.DB.Exec(`DELETE FROM outlets WHERE id = $1`, outletID)

	makePR := func(reqType, name string, manual bool, final float64) string {
		id := NewULID()
		num := "PRG-" + id[20:]
		itemsJSON := `[{"name":"` + name + `","items":[{"name":"` + name + ` Item","qty":5,"unit":"pcs","hps_price":10000,"final_price":10000}]}]`
		_, err := database.DB.Exec(`
			INSERT INTO purchase_requests (
				id, request_number, request_type, outlet_id, requested_by, status,
				total_hps, total_final, total_amount, paid_amount, vendor_name,
				items, is_manual_payment, created_at, updated_at
			) VALUES ($1,$2,$3,$4,'Pengaju Test','payment_requested',$5,$5,$5,0,'Vendor Test',$6::jsonb,$7,NOW(),NOW())`,
			id, num, reqType, outletID, final, itemsJSON, manual)
		if err != nil {
			t.Fatalf("insert PR %s: %v", reqType, err)
		}
		return id
	}

	prBahan := makePR(RequestTypeBahan, "Beras Test Group", false, 50000)
	prBarang := makePR(RequestTypeBarang, "Kompor Test Group", false, 60000)
	prJasa := makePR(RequestTypeJasa, "Servis AC Test Group", false, 70000)
	prManual := makePR(RequestTypeJasa, "Listrik Manual Test Group", true, 80000)
	defer func() {
		database.DB.Exec(`DELETE FROM logistics_deliveries WHERE id IN (SELECT delivery_id FROM logistics_delivery_items WHERE purchase_request_id IN ($1,$2,$3,$4))`, prBahan, prBarang, prJasa, prManual)
		database.DB.Exec(`DELETE FROM purchase_requests WHERE id IN ($1,$2,$3,$4)`, prBahan, prBarang, prJasa, prManual)
	}()

	scope := []string{outletID}

	// 1. Filter type=barang_jasa memuat barang & jasa, tidak memuat bahan.
	resBJ, err := ListPurchaseRequestsExt("", "", PurchaseStatusCommittedFilter, PurchasingGroupBarangJasa, "", "all", true, "", scope, nil, nil, 1, 50, PurchaseListExtra{})
	if err != nil {
		t.Fatalf("ListPurchaseRequestsExt barang_jasa: %v", err)
	}
	seenBJ := map[string]bool{}
	for _, r := range resBJ.Requests {
		seenBJ[strings.TrimSpace(r.ID)] = true
		if r.RequestType == RequestTypeBahan {
			t.Fatalf("filter barang_jasa tidak boleh memuat bahan baku (%s)", r.RequestNumber)
		}
	}
	if !seenBJ[prBarang] || !seenBJ[prJasa] {
		t.Fatalf("filter barang_jasa harus memuat barang dan jasa, got=%v", seenBJ)
	}

	// 2. SourceGroup partition: bahan_baku, barang_jasa, manual tidak saling tumpang tindih.
	checkSource := func(src string, expectIDs ...string) {
		res, err := ListPurchaseRequestsExt("", "", PurchaseStatusCommittedFilter, "", "", "all", true, "", scope, nil, nil, 1, 50, PurchaseListExtra{SourceGroup: src})
		if err != nil {
			t.Fatalf("SourceGroup %s: %v", src, err)
		}
		if len(res.Requests) != len(expectIDs) {
			t.Fatalf("SourceGroup %s: expect %d rows, got %d", src, len(expectIDs), len(res.Requests))
		}
		got := map[string]bool{}
		for _, r := range res.Requests {
			got[strings.TrimSpace(r.ID)] = true
		}
		for _, eid := range expectIDs {
			if !got[eid] {
				t.Fatalf("SourceGroup %s: missing expected PR %s", src, eid)
			}
		}
	}
	checkSource(PurchasingGroupBahanBaku, prBahan)
	checkSource(PurchasingGroupBarangJasa, prBarang, prJasa)
	checkSource("manual", prManual)

	// 3. PaymentStats GroupBreakdown akurat untuk tab Finance.
	ps, err := GetPaymentStats(scope, nil)
	if err != nil {
		t.Fatalf("GetPaymentStats: %v", err)
	}
	if ps.GroupBreakdown.BahanBaku.WaitingCount != 1 || ps.GroupBreakdown.BarangJasa.WaitingCount != 2 || ps.GroupBreakdown.Manual.WaitingCount != 1 {
		t.Fatalf("GroupBreakdown mismatch: %+v", ps.GroupBreakdown)
	}

	// 4. Dashboard Pengadaan terisolasi sesuai allowedTypes.
	dashRaw, err := GetProcurementDashboardFiltered("", scope, nil, []string{RequestTypeBahan})
	if err != nil {
		t.Fatalf("GetProcurementDashboardFiltered raw: %v", err)
	}
	if dashRaw.TypeSplit.Bahan != 1 || dashRaw.TypeSplit.Barang != 0 || dashRaw.TypeSplit.Jasa != 0 {
		t.Fatalf("Dashboard Bahan Baku bocor ke Barang/Jasa: %+v", dashRaw.TypeSplit)
	}
	dashGoods, err := GetProcurementDashboardFiltered("", scope, nil, []string{RequestTypeBarang, RequestTypeJasa})
	if err != nil {
		t.Fatalf("GetProcurementDashboardFiltered goods: %v", err)
	}
	if dashGoods.TypeSplit.Bahan != 0 || dashGoods.TypeSplit.Barang != 1 || dashGoods.TypeSplit.Jasa != 1 {
		t.Fatalf("Dashboard Barang & Jasa bocor ke Bahan Baku atau Manual: %+v", dashGoods.TypeSplit)
	}

	// 5. Excel Pembayaran mencantumkan kolom AA "Kelompok / Asal" dan AB "Pengaju".
	xlsBytes, _, err := BuildProcurementPaymentsExcelExt("", "", "", "", scope, nil, PurchaseListExtra{})
	if err != nil {
		t.Fatalf("BuildProcurementPaymentsExcelExt: %v", err)
	}
	xf, err := excelize.OpenReader(bytes.NewReader(xlsBytes))
	if err != nil {
		t.Fatalf("open excel: %v", err)
	}
	defer xf.Close()
	hAA, _ := xf.GetCellValue("Daftar Pembayaran", "AA1")
	hAB, _ := xf.GetCellValue("Daftar Pembayaran", "AB1")
	if hAA != "Kelompok / Asal" || hAB != "Pengaju" {
		t.Fatalf("Excel header AA/AB tidak sesuai: got %q, %q", hAA, hAB)
	}

	// 6. Logistik: tolak campur bahan + barang dalam 1 Surat Jalan, dan isolasi list/get per kelompok.
	disp, err := ListDispatchablePRItems(scope, nil, "", "", "", "")
	if err != nil {
		t.Fatalf("ListDispatchablePRItems: %v", err)
	}
	var keyBahan, keyBarang string
	for _, it := range disp {
		if it.PurchaseRequestID == prBahan {
			keyBahan = it.PRItemKey
		}
		if it.PurchaseRequestID == prBarang {
			keyBarang = it.PRItemKey
		}
	}
	if keyBahan == "" || keyBarang == "" {
		t.Fatalf("missing dispatchable item keys: bahan=%q barang=%q", keyBahan, keyBarang)
	}
	actor := Actor{ID: "", Name: "Tester Logistik"}
	_, err = CreateLogisticsDelivery(models.CreateLogisticsDeliveryInput{
		DispatchMode:    "logistics",
		DestinationType: "warehouse",
		Items: []models.LogisticsDeliveryItemInput{
			{PurchaseRequestID: prBahan, PRItemKey: keyBahan, QtyPlanned: 2},
			{PurchaseRequestID: prBarang, PRItemKey: keyBarang, QtyPlanned: 1},
		},
	}, actor, scope, nil)
	if err == nil || !strings.Contains(err.Error(), "tidak boleh dicampur") {
		t.Fatalf("expected error rejecting mixed bahan+barang SJ, got: %v", err)
	}

	sjRaw, err := CreateLogisticsDelivery(models.CreateLogisticsDeliveryInput{
		DispatchMode:    "logistics",
		DestinationType: "warehouse",
		Items:           []models.LogisticsDeliveryItemInput{{PurchaseRequestID: prBahan, PRItemKey: keyBahan, QtyPlanned: 2}},
	}, actor, scope, []string{RequestTypeBahan})
	if err != nil {
		t.Fatalf("create raw SJ: %v", err)
	}
	if sjRaw.PurchasingGroup != PurchasingGroupBahanBaku {
		t.Fatalf("expected sjRaw group bahan_baku, got %q", sjRaw.PurchasingGroup)
	}

	sjGoods, err := CreateLogisticsDelivery(models.CreateLogisticsDeliveryInput{
		DispatchMode:    "logistics",
		DestinationType: "asset_desk",
		Items:           []models.LogisticsDeliveryItemInput{{PurchaseRequestID: prBarang, PRItemKey: keyBarang, QtyPlanned: 1}},
	}, actor, scope, []string{RequestTypeBarang})
	if err != nil {
		t.Fatalf("create goods SJ: %v", err)
	}
	if sjGoods.PurchasingGroup != PurchasingGroupBarangJasa {
		t.Fatalf("expected sjGoods group barang_jasa, got %q", sjGoods.PurchasingGroup)
	}

	rawOnly, rawTotal, err := ListLogisticsDeliveriesFiltered("", "", "", "", "", "", "", scope, []string{PurchasingGroupBahanBaku}, 1, 20)
	if err != nil {
		t.Fatalf("list raw SJ: %v", err)
	}
	if rawTotal != 1 || len(rawOnly) != 1 || rawOnly[0].ID != sjRaw.ID {
		t.Fatalf("isolasi list bahan_baku gagal: total=%d items=%+v", rawTotal, rawOnly)
	}

	if err := CheckLogisticsDeliveryGroupAccess(sjGoods.ID, []string{PurchasingGroupBahanBaku}); err == nil {
		t.Fatalf("akses lintas kelompok bahan_baku ke SJ barang_jasa seharusnya ditolak")
	}
	if err := CheckLogisticsDeliveryGroupAccess(sjRaw.ID, []string{PurchasingGroupBarangJasa}); err == nil {
		t.Fatalf("akses lintas kelompok barang_jasa ke SJ bahan_baku seharusnya ditolak")
	}

	// 7. Verifikasi hasil migrasi mig_purchasing_group_split_v1 pada role standar & role demo eksisting.
	rawPerms, err := GetRolePermissions("purchasing bahan baku")
	if err != nil {
		t.Fatalf("get perms purchasing bahan baku: %v", err)
	}
	if !HasPermission(rawPerms, "logistics.rawmaterials.manage") || HasPermission(rawPerms, "logistics.goods.view") || HasPermission(rawPerms, "procurement.requests.view") {
		t.Fatalf("role 'purchasing bahan baku' tercampur: %v", rawPerms)
	}
	goodsPerms, err := GetRolePermissions("purchasing barang & jasa")
	if err != nil {
		t.Fatalf("get perms purchasing barang & jasa: %v", err)
	}
	if !HasPermission(goodsPerms, "logistics.goods.manage") || HasPermission(goodsPerms, "logistics.rawmaterials.view") || HasPermission(goodsPerms, "procurement.rawmaterials.view") {
		t.Fatalf("role 'purchasing barang & jasa' tercampur: %v", goodsPerms)
	}
}

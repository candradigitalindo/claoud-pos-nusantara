package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func usageFixture(t *testing.T, qty float64) (string, string, string) {
	t.Helper()
	requireDB(t)
	if err := database.RunProjectUsageMigrations(); err != nil {
		t.Fatal(err)
	}
	p := newTestProject(t, 0)
	mid := NewULID()
	if _, err := database.DB.Exec(`INSERT INTO project_materials(id,project_id,name,unit,qty_received,unit_cost) VALUES($1,$2,'Semen uji','sak',$3,50000)`, mid, p.ID, qty); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM project_usage_reports WHERE project_id=$1`, p.ID)
		database.DB.Exec(`DELETE FROM project_materials WHERE project_id=$1`, p.ID)
		database.DB.Exec(`DELETE FROM assets WHERE project_id=$1`, p.ID)
		database.DB.Exec(`DELETE FROM projects WHERE id=$1`, p.ID)
	})
	if err := os.MkdirAll("uploads", 0755); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp("uploads", "usage-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	raw, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aJ9sAAAAASUVORK5CYII=")
	if err = os.WriteFile(filepath.Join(dir, "proof.png"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p.ID, mid, "/" + filepath.ToSlash(filepath.Join(dir, "proof.png"))
}

func usageInput(pid, mid, proof string, qty float64) models.ProjectUsageInput {
	return models.ProjectUsageInput{ProjectID: pid, Kind: "pemakaian", UsageDate: time.Now().In(GetTimezoneLocation()).Format("2006-01-02"), Location: "Dapur", WorkDescription: "Pengecoran lantai", Performer: "Pelaksana", PICName: "PIC lokasi", Items: []models.ProjectUsageItem{{MaterialID: mid, Qty: qty}}, Evidence: []string{proof}}
}

func usageBalance(t *testing.T, mid string, used, wasted, remaining float64) {
	t.Helper()
	m, err := GetProjectMaterial(mid)
	if err != nil {
		t.Fatal(err)
	}
	if m.QtyUsed != used || m.QtyWasted != wasted || m.Remaining != remaining {
		t.Fatalf("saldo %+v, want used=%v wasted=%v remaining=%v", m, used, wasted, remaining)
	}
}

func TestProjectUsageLifecycleEvidenceRevisionAndClosure(t *testing.T) {
	pid, mid, proof := usageFixture(t, 100)
	maker := Actor{ID: "maker", Name: "Petugas"}
	reviewer := Actor{ID: "reviewer", Name: "Atasan"}
	in := usageInput(pid, mid, proof, 60)
	in.Evidence = nil
	r, err := SaveProjectUsage("", in, maker)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 0, 0, 100)
	if _, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: r.Version}, maker, false); err == nil {
		t.Fatal("submitted without proof")
	}
	in = r.ProjectUsageInput
	in.Evidence = []string{proof}
	r, err = SaveProjectUsage(r.ID, in, maker)
	if err != nil {
		t.Fatal(err)
	}
	v := r.Version
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: v}, maker, false)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 60, 0, 40)
	if _, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: v}, maker, false); !errors.Is(err, ErrUsageConflict) {
		t.Fatalf("duplicate submit: %v", err)
	}
	if _, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "verify", Version: r.Version}, maker, true); !errors.Is(err, ErrUsageForbidden) {
		t.Fatalf("self approval: %v", err)
	}
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "revise", Version: r.Version, Note: "Perbaiki jumlah sesuai lapangan"}, reviewer, true)
	if err != nil {
		t.Fatal(err)
	}
	in = r.ProjectUsageInput
	in.Items[0].Qty = 55
	r, err = SaveProjectUsage(r.ID, in, maker)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 60, 0, 40)
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: r.Version}, maker, false)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 55, 0, 45)
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "verify", Version: r.Version}, reviewer, true)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 55, 0, 45)
	if len(r.Events) != 7 {
		t.Fatalf("events=%d", len(r.Events))
	}
	if r.Events[2].Snapshot.Items[0].Qty != 60 {
		t.Fatal("old submission snapshot changed")
	}
	if _, err = SaveProjectUsage(r.ID, r.ProjectUsageInput, maker); err == nil {
		t.Fatal("verified report editable")
	}
	if _, err = UpdateProject(pid, models.UpdateProjectRequest{Name: "Uji", Status: "selesai"}); err == nil {
		t.Fatal("project closed with stock remaining")
	}
	waste := usageInput(pid, mid, proof, 45)
	waste.Kind = "susut"
	waste.WorkDescription = "Rusak terkena hujan"
	w, err := SaveProjectUsage("", waste, maker)
	if err != nil {
		t.Fatal(err)
	}
	w, err = ActProjectUsage(w.ID, models.ProjectUsageAction{Action: "submit", Version: w.Version}, maker, false)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 55, 45, 0)
	if _, err = UpdateProject(pid, models.UpdateProjectRequest{Name: "Uji", Status: "selesai"}); err == nil {
		t.Fatal("project closed before verification")
	}
	if _, err = ActProjectUsage(w.ID, models.ProjectUsageAction{Action: "verify", Version: w.Version}, reviewer, true); err != nil {
		t.Fatal(err)
	}
	if _, err = UpdateProject(pid, models.UpdateProjectRequest{Name: "Uji", Status: "selesai"}); err != nil {
		t.Fatal(err)
	}
	if _, err = SaveProjectUsage("", usageInput(pid, mid, proof, 1), maker); err == nil {
		t.Fatal("new report in closed project")
	}
	if err = database.RunProjectUsageMigrations(); err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 55, 45, 0)
}

func TestProjectUsageConcurrentSubmissionsAndSettlement(t *testing.T) {
	pid, mid, proof := usageFixture(t, 10)
	a := Actor{ID: "maker", Name: "Petugas"}
	var reports []*models.ProjectUsageReport
	for i := 0; i < 2; i++ {
		r, err := SaveProjectUsage("", usageInput(pid, mid, proof, 7), a)
		if err != nil {
			t.Fatal(err)
		}
		reports = append(reports, r)
	}
	var wg sync.WaitGroup
	result := make(chan error, 2)
	start := make(chan struct{})
	for _, r := range reports {
		wg.Add(1)
		go func(r *models.ProjectUsageReport) {
			defer wg.Done()
			<-start
			_, err := ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: r.Version}, a, false)
			result <- err
		}(r)
	}
	close(start)
	wg.Wait()
	close(result)
	success := 0
	for err := range result {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("submissions succeeded=%d", success)
	}
	usageBalance(t, mid, 7, 0, 3)
	var events int
	if err := database.DB.QueryRow(`SELECT count(*) FROM project_material_logs WHERE material_id=$1 AND type='pemakaian'`, mid).Scan(&events); err != nil || events != 1 {
		t.Fatalf("duplicate ledger %d %v", events, err)
	}
	if _, err := SettleMaterialRemainder(mid, models.ProjectMaterialSettleRequest{Action: "aset", Qty: 0.5, PhotoURL: proof}, a.Name, false, nil); err == nil || !strings.Contains(err.Error(), "bulat") {
		t.Fatalf("fraction truncated: %v", err)
	}
	if _, err := RecordMaterialUsage(mid, models.ProjectMaterialUsageRequest{Qty: 1}, a.Name); err == nil {
		t.Fatal("legacy usage bypass")
	}
	if _, err := SettleMaterialRemainder(mid, models.ProjectMaterialSettleRequest{Action: "susut", Qty: 1, PhotoURL: proof, Notes: "rusak"}, a.Name, false, nil); err == nil {
		t.Fatal("waste bypass")
	}
	if err := validateReceivingDestination(models.ReceivingLine{}, "barang", pid, "habis"); err == nil {
		t.Fatal("project expense bypass")
	}

	// A settlement and usage submitted together cannot consume the same stock.
	pid2, mid2, proof2 := usageFixture(t, 10)
	target := newTestProject(t, 0)
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM project_materials WHERE project_id=$1`, target.ID)
		database.DB.Exec(`DELETE FROM projects WHERE id=$1`, target.ID)
	})
	r, err := SaveProjectUsage("", usageInput(pid2, mid2, proof2, 7), a)
	if err != nil {
		t.Fatal(err)
	}
	result = make(chan error, 2)
	start = make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, e := ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: r.Version}, a, false)
		result <- e
	}()
	go func() {
		defer wg.Done()
		<-start
		_, e := SettleMaterialRemainder(mid2, models.ProjectMaterialSettleRequest{Action: "pindah", Qty: 7, TargetProjectID: target.ID, PhotoURL: proof2}, a.Name, false, nil)
		result <- e
	}()
	close(start)
	wg.Wait()
	close(result)
	success = 0
	for e := range result {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("usage + settlement succeeded=%d", success)
	}
	m, _ := GetProjectMaterial(mid2)
	if m.Remaining != 3 {
		t.Fatalf("remaining=%v", m.Remaining)
	}
}

func TestProjectUsageCancellationAndValidation(t *testing.T) {
	pid, mid, proof := usageFixture(t, 10)
	a := Actor{ID: "maker", Name: "Petugas"}
	b := Actor{ID: "reviewer", Name: "Atasan"}
	in := usageInput(pid, mid, proof, 4)
	r, err := SaveProjectUsage("", in, a)
	if err != nil {
		t.Fatal(err)
	}
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "submit", Version: r.Version}, a, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "cancel", Version: r.Version, Note: "salah"}, a, false); err == nil {
		t.Fatal("unreviewed cancellation")
	}
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "revise", Version: r.Version, Note: "salah barang"}, b, true)
	if err != nil {
		t.Fatal(err)
	}
	r, err = ActProjectUsage(r.ID, models.ProjectUsageAction{Action: "cancel", Version: r.Version, Note: "barang belum dipakai"}, a, false)
	if err != nil {
		t.Fatal(err)
	}
	usageBalance(t, mid, 0, 0, 10)
	for _, q := range []float64{0, -1, 0.00001, 1e11} {
		bad := usageInput(pid, mid, proof, q)
		if _, err = SaveProjectUsage("", bad, a); err == nil {
			t.Fatalf("qty %v accepted", q)
		}
	}
	bad := usageInput(pid, mid, "/uploads/missing.png", 1)
	if _, err = SaveProjectUsage("", bad, a); err == nil {
		t.Fatal("missing evidence accepted")
	}
	bad = usageInput(pid, mid, "/uploads/../../etc/passwd.png", 1)
	if _, err = SaveProjectUsage("", bad, a); err == nil {
		t.Fatal("path traversal accepted")
	}
	bad = usageInput(pid, mid, proof, 1)
	bad.Items = append(bad.Items, bad.Items[0])
	if _, err = SaveProjectUsage("", bad, a); err == nil {
		t.Fatal("duplicate item accepted")
	}
	pid2, _, _ := usageFixture(t, 2)
	bad = usageInput(pid2, mid, proof, 1)
	if _, err = SaveProjectUsage("", bad, a); err == nil {
		t.Fatal("cross project material accepted")
	}
	if _, err = GetProjectUsageOverview(pid); err != nil {
		t.Fatal(err)
	}
}

func TestProjectUsageReceiptAssetLinkAndHandover(t *testing.T) {
	pid, mid, proof := usageFixture(t, 1)
	// Use the real receiving flow for a purchase containing both materials
	// and equipment; all references must survive without searching notes.
	outlet := recvOutlet(t)
	pr := recvApprovedPR(t, "barang", []models.PurchaseRequestItem{{Name: "Projek", Items: []models.PurchaseSubItem{
		{Name: "Semen konstruksi uji", Qty: 2, Unit: "sak", HpsPrice: 50000},
		{Name: "Bor konstruksi uji", Qty: 1, Unit: "unit", HpsPrice: 500000},
	}}})
	if _, err := database.DB.Exec(`UPDATE purchase_requests SET project_id=$1,outlet_id=$2 WHERE id=$3`, pid, outlet, pr.ID); err != nil {
		t.Fatal(err)
	}
	draft, err := BuildReceivingDraft(pr.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	lines := []models.ReceiveGoodsLine{}
	for _, line := range draft.Lines {
		dest := "material"
		if strings.Contains(line.Name, "Bor") {
			dest = "aset"
		}
		lines = append(lines, models.ReceiveGoodsLine{PRItemKey: line.PRItemKey, Qty: line.Qty, Destination: dest, TrackingMode: "tunggal"})
	}
	received, err := ReceiveGoods(pr.ID, models.ReceiveGoodsRequest{PhotoURL: proof, Lines: lines}, Actor{ID: "maker", Name: "Petugas"}, ReceiveAccess{Asset: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if received.AssetsCreated != 1 || received.MaterialLines != 1 {
		t.Fatalf("receipt %+v", received)
	}
	overview, err := GetProjectUsageOverview(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Assets) != 1 || len(overview.Materials) != 2 {
		t.Fatalf("overview %+v", overview)
	}
	_, err = CreateAssetHandover(models.AssetHandoverRequest{OutletID: outlet, PicName: "Budi", Location: "Gudang projek", PhotoURL: proof, Items: []models.AssetHandoverItemRequest{{AssetID: received.AssetIDs[0], Qty: 1}}}, "Petugas", nil)
	if err != nil {
		t.Fatal(err)
	}
	overview, err = GetProjectUsageOverview(pid)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Assets[0].PIC != "Budi" || len(overview.Assets[0].Evidence) != 1 || overview.Assets[0].Qty != 1 {
		t.Fatalf("asset custody %+v", overview.Assets)
	}
	usageBalance(t, mid, 0, 0, 1)
}

package services

// Uji alur RAB per baris: susun → tetapkan → belanja per baris → serapan per
// baris; plus pagar-pagarnya (belum ditetapkan, jenis tidak cocok, baris yang
// sudah dipakai tidak boleh dihapus).
//
// TestMain + requireDB ada di purchase_flow_test.go (paket yang sama).

import (
	"strings"
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func rabLine(items []models.ProjectRabItem, name string) *models.ProjectRabItem {
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func TestProjectRabFlow(t *testing.T) {
	requireDB(t)
	proj, err := CreateProject(models.CreateProjectRequest{Name: "Renovasi Teras (test RAB)", Status: "draft"}, "tester")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE project_id = $1", proj.ID)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	if proj.RabStatus != RabDraft || proj.RabItemCount != 0 || proj.Budget != 0 {
		t.Fatalf("projek baru: rab_status=%s items=%d budget=%v, mau draft/0/0", proj.RabStatus, proj.RabItemCount, proj.Budget)
	}

	// Belum ada RAB → pengajuan ditolak.
	_, err = CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{Name: "Material", Items: []models.PurchaseSubItem{{Name: "Semen", Qty: 1, HpsPrice: 60000}}}},
	})
	if err == nil || !strings.Contains(err.Error(), "belum ditetapkan") {
		t.Fatalf("PR sebelum RAB ditetapkan harus ditolak, dapat: %v", err)
	}

	// Susun RAB.
	items, err := SaveProjectRab(proj.ID, models.SaveProjectRabRequest{Items: []models.ProjectRabItemInput{
		{Section: "Pekerjaan Sipil", Name: "Semen", Kind: "barang", Unit: "sak", Qty: 100, UnitPrice: 60000},
		{Section: "Pekerjaan Sipil", Name: "Upah tukang", Kind: "jasa", Unit: "hari", Qty: 10, UnitPrice: 150000},
		{Section: "Lain-lain", Name: "Tak terduga", Kind: "umum", Unit: "ls", Qty: 1, UnitPrice: 500000},
	}})
	if err != nil {
		t.Fatalf("save RAB: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("jumlah baris = %d, mau 3", len(items))
	}
	p, _ := GetProject(proj.ID)
	if p.Budget != 8000000 {
		t.Errorf("budget = Σ subtotal = %v, mau 8000000", p.Budget)
	}
	if p.Status != "draft" || p.RabStatus != RabDraft {
		t.Errorf("sebelum ditetapkan: status=%s rab=%s, mau draft/draft", p.Status, p.RabStatus)
	}

	// Tetapkan.
	p, err = SetProjectRab(proj.ID, "manajer")
	if err != nil {
		t.Fatalf("set RAB: %v", err)
	}
	if p.RabStatus != RabSet || p.RabVersion != 1 || p.RabSetBy != "manajer" || p.Status != "berjalan" {
		t.Errorf("setelah ditetapkan: rab=%s v%d oleh %q status=%s", p.RabStatus, p.RabVersion, p.RabSetBy, p.Status)
	}
	if _, err := SaveProjectRab(proj.ID, models.SaveProjectRabRequest{Items: []models.ProjectRabItemInput{{Name: "x", Qty: 1}}}); err == nil {
		t.Errorf("RAB yang sudah ditetapkan tidak boleh diubah tanpa buka revisi")
	}

	semen, upah, umum := rabLine(items, "Semen"), rabLine(items, "Upah tukang"), rabLine(items, "Tak terduga")

	// Belanja barang menyerap baris Semen (HPS, belum ada harga final).
	pr1, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{Name: "Material tahap 1", Items: []models.PurchaseSubItem{
			{Name: "Semen", Qty: 20, Unit: "sak", HpsPrice: 60000, RabItemID: semen.ID},
		}}},
	})
	if err != nil {
		t.Fatalf("PR barang per baris RAB: %v", err)
	}
	if pr1.Items[0].Items[0].RabItemID != semen.ID {
		t.Errorf("rab_item_id tidak tersimpan di item pengajuan")
	}

	// Jasa tidak boleh menyerap baris barang.
	_, err = CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "jasa", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{Name: "Tukang", Items: []models.PurchaseSubItem{
			{Name: "Upah", Qty: 5, HpsPrice: 150000, RabItemID: semen.ID},
		}}},
	})
	if err == nil || !strings.Contains(err.Error(), "berjenis barang") {
		t.Errorf("jasa menyerap baris barang harus ditolak, dapat: %v", err)
	}

	// Baris 'umum' boleh diserap siapa saja; item tanpa rujukan = di luar RAB.
	_, err = CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "jasa", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{Name: "Jasa tahap 1", Items: []models.PurchaseSubItem{
			{Name: "Sewa scaffolding", Qty: 1, HpsPrice: 200000, RabItemID: umum.ID},
			{Name: "Konsumsi tukang", Qty: 1, HpsPrice: 75000},
		}}},
	})
	if err != nil {
		t.Fatalf("PR jasa (umum + di luar RAB): %v", err)
	}

	detail, err := GetProjectDetail(proj.ID, nil, nil)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	ls := rabLine(detail.RabItems, "Semen")
	if ls.Committed != 1200000 || ls.Estimated != 1200000 || ls.Remaining != 4800000 || ls.RequestCount != 1 {
		t.Errorf("serapan Semen: committed=%v est=%v sisa=%v n=%d, mau 1200000/1200000/4800000/1", ls.Committed, ls.Estimated, ls.Remaining, ls.RequestCount)
	}
	if lu := rabLine(detail.RabItems, "Tak terduga"); lu.Committed != 200000 || lu.Remaining != 300000 {
		t.Errorf("serapan Tak terduga: committed=%v sisa=%v, mau 200000/300000", lu.Committed, lu.Remaining)
	}
	if lj := rabLine(detail.RabItems, "Upah tukang"); lj.Committed != 0 || lj.Remaining != 1500000 {
		t.Errorf("baris jasa tak tersentuh: committed=%v sisa=%v", lj.Committed, lj.Remaining)
	}
	if detail.Project.OffRabCommitted != 75000 {
		t.Errorf("belanja di luar RAB = %v, mau 75000", detail.Project.OffRabCommitted)
	}
	// Rekap projek tetap menjumlahkan semuanya (1.200.000 + 200.000 + 75.000).
	if detail.Project.Committed != 1475000 {
		t.Errorf("komitmen projek = %v, mau 1475000", detail.Project.Committed)
	}
	_ = upah

	// Harga final masuk → serapan baris ikut memakai harga final.
	if _, err := UpdatePurchaseItems(pr1.ID, models.UpdatePurchaseItemsInput{Items: []models.PurchaseRequestItem{{
		Name: "Material tahap 1", Items: []models.PurchaseSubItem{
			{Name: "Semen", Qty: 20, Unit: "sak", HpsPrice: 60000, FinalPrice: 55000, RabItemID: semen.ID},
		}}}}); err != nil {
		t.Fatalf("isi harga final: %v", err)
	}
	detail, _ = GetProjectDetail(proj.ID, nil, nil)
	if ls := rabLine(detail.RabItems, "Semen"); ls.Committed != 1100000 || ls.Estimated != 0 {
		t.Errorf("setelah harga final: committed=%v est=%v, mau 1100000/0", ls.Committed, ls.Estimated)
	}

	// Revisi: baris yang sudah dipakai tidak boleh dihapus, tetapi boleh diubah.
	if _, err := ReopenProjectRab(proj.ID); err != nil {
		t.Fatalf("buka revisi: %v", err)
	}
	_, err = SaveProjectRab(proj.ID, models.SaveProjectRabRequest{Items: []models.ProjectRabItemInput{
		{ID: upah.ID, Section: "Pekerjaan Sipil", Name: "Upah tukang", Kind: "jasa", Unit: "hari", Qty: 10, UnitPrice: 150000},
	}})
	if err == nil || !strings.Contains(err.Error(), "sudah dipakai") {
		t.Errorf("menghapus baris yang sudah dipakai harus ditolak, dapat: %v", err)
	}
	items, err = SaveProjectRab(proj.ID, models.SaveProjectRabRequest{Items: []models.ProjectRabItemInput{
		{ID: semen.ID, Section: "Pekerjaan Sipil", Name: "Semen", Kind: "barang", Unit: "sak", Qty: 120, UnitPrice: 60000},
		{ID: upah.ID, Section: "Pekerjaan Sipil", Name: "Upah tukang", Kind: "jasa", Unit: "hari", Qty: 10, UnitPrice: 150000},
		{ID: umum.ID, Section: "Lain-lain", Name: "Tak terduga", Kind: "umum", Unit: "ls", Qty: 1, UnitPrice: 500000},
		{Section: "Lain-lain", Name: "Cat", Kind: "barang", Unit: "pail", Qty: 4, UnitPrice: 250000},
	}})
	if err != nil {
		t.Fatalf("revisi RAB: %v", err)
	}
	if len(items) != 4 {
		t.Errorf("jumlah baris setelah revisi = %d, mau 4", len(items))
	}
	p, err = SetProjectRab(proj.ID, "manajer")
	if err != nil {
		t.Fatalf("tetapkan ulang: %v", err)
	}
	if p.RabVersion != 2 || p.Budget != 10200000 {
		t.Errorf("setelah revisi: v%d budget=%v, mau v2/10200000", p.RabVersion, p.Budget)
	}
}

// Jalan pintas API: anggaran satu angka = satu baris 'umum' yang langsung
// ditetapkan, supaya pemanggil lama tetap bisa langsung mengajukan belanja.
func TestProjectLumpSumBudgetBecomesSetRab(t *testing.T) {
	requireDB(t)
	proj := newTestProject(t, 2500000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	if proj.RabStatus != RabSet || proj.RabItemCount != 1 || proj.Budget != 2500000 || proj.RabVersion != 1 {
		t.Fatalf("lump sum: rab=%s items=%d budget=%v v%d", proj.RabStatus, proj.RabItemCount, proj.Budget, proj.RabVersion)
	}
	rab, err := GetProjectRab(proj.ID, nil, nil)
	if err != nil {
		t.Fatalf("get rab: %v", err)
	}
	if len(rab.Items) != 1 || rab.Items[0].Kind != "umum" || rab.Items[0].Subtotal != 2500000 {
		t.Errorf("baris gelondongan: %+v", rab.Items)
	}
	// Rujukan ke baris projek lain ditolak.
	other := newTestProject(t, 100000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", other.ID)
	otherRab, _ := GetProjectRab(other.ID, nil, nil)
	_, err = CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{Name: "X", Items: []models.PurchaseSubItem{{Name: "Y", Qty: 1, HpsPrice: 1000, RabItemID: otherRab.Items[0].ID}}}},
	})
	if err == nil || !strings.Contains(err.Error(), "tidak ditemukan") {
		t.Errorf("rujukan baris projek lain harus ditolak, dapat: %v", err)
	}
}

package services

// Uji regresi rekap RAB projek. Yang dikunci di sini adalah dua hal yang paling
// mudah salah: (1) nilai pengadaan tidak boleh terhitung dua kali ketika
// belanjanya dipecah ke beberapa vendor, dan (2) pecahan vendor harus tetap
// membawa project_id induknya — kalau tidak, nilainya lenyap dari RAB begitu
// tim purchasing memecah vendor.
//
// TestMain + requireDB ada di purchase_flow_test.go (paket yang sama).

import (
	"testing"

	"cloud-pos/database"
	"cloud-pos/models"
)

func newTestProject(t *testing.T, budget float64) *models.Project {
	t.Helper()
	p, err := CreateProject(models.CreateProjectRequest{
		Name: "Renovasi Dapur (test)", Budget: budget, Status: "berjalan",
	}, "tester")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	return p
}

// Split vendor memindahkan item ke pecahan. Rekap projek harus tetap
// menjumlahkan nilai yang sama — bukan dua kali lipat.
func TestProjectSummaryNotDoubleCountedAfterSplit(t *testing.T) {
	requireDB(t)
	proj := newTestProject(t, 1000000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{
			Name: "Material",
			Items: []models.PurchaseSubItem{
				{Name: "Semen", Qty: 10, Unit: "sak", HpsPrice: 50000, FinalPrice: 50000},
				{Name: "Keramik", Qty: 20, Unit: "dus", HpsPrice: 10000, FinalPrice: 10000},
			},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)

	if pr.ProjectID == nil || *pr.ProjectID != proj.ID {
		t.Fatalf("project_id PR = %v, mau %s", pr.ProjectID, proj.ID)
	}

	// Sebelum split: komitmen = 500.000 + 200.000.
	before, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if before.Committed != 700000 {
		t.Fatalf("komitmen sebelum split = %v, mau 700000", before.Committed)
	}
	if before.RemainingBudget != 300000 {
		t.Errorf("sisa RAB = %v, mau 300000", before.RemainingBudget)
	}

	// Pecah Keramik ke vendor lain.
	master, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Keramik",
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Keramik", Qty: 20, HpsPrice: 10000, FinalPrice: 10000}},
		}},
	})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(master.Children) != 1 {
		t.Fatalf("jumlah pecahan = %d, mau 1", len(master.Children))
	}
	child := master.Children[0]
	if child.ProjectID == nil || *child.ProjectID != proj.ID {
		t.Fatalf("pecahan kehilangan projek: project_id = %v", child.ProjectID)
	}

	after, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project setelah split: %v", err)
	}
	if after.Committed != 700000 {
		t.Errorf("komitmen setelah split = %v, mau tetap 700000 (master sisa + pecahan)", after.Committed)
	}
	if after.RequestCount != 2 {
		t.Errorf("jumlah dokumen = %d, mau 2 (master sisa + 1 pecahan)", after.RequestCount)
	}

	// Detail harus menampilkan himpunan dokumen yang sama dengan yang dijumlahkan.
	detail, err := GetProjectDetail(proj.ID, nil, nil)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	var sum float64
	for _, r := range detail.Requests {
		sum += r.TotalFinal
	}
	if sum != after.Committed {
		t.Errorf("jumlah daftar belanja (%v) != komitmen rekap (%v) — rekap dan daftarnya memakai filter berbeda", sum, after.Committed)
	}
}

// Pengajuan yang belum diberi harga oleh purchasing tetap memakan RAB memakai
// HPS-nya, dan porsi itu dilaporkan terpisah sebagai Estimated. Kalau tidak,
// sisa RAB terlihat lega padahal belanjanya sudah antre.
func TestProjectCountsUnpricedRequestAtHps(t *testing.T) {
	requireDB(t)
	proj := newTestProject(t, 1000000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	// HPS terisi, harga final BELUM (final_price = 0) — persis kondisi
	// pengajuan yang baru disetujui dan menunggu tim purchasing.
	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Cat", Qty: 4, Unit: "pail", HpsPrice: 100000}},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", pr.ID)

	p, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if p.Committed != 400000 {
		t.Errorf("komitmen = %v, mau 400000 (HPS dipakai selama harga final belum diisi)", p.Committed)
	}
	if p.Estimated != 400000 {
		t.Errorf("estimated = %v, mau 400000 (seluruh komitmen masih tebakan)", p.Estimated)
	}
	if p.RemainingBudget != 600000 {
		t.Errorf("sisa RAB = %v, mau 600000", p.RemainingBudget)
	}
	// Belum ada kesepakatan harga → belum ada hutang.
	if p.Outstanding != 0 {
		t.Errorf("sisa hutang = %v, mau 0 — estimasi HPS bukan kewajiban", p.Outstanding)
	}

	// Purchasing mengisi harga final lebih murah: komitmen ikut turun dan
	// porsi estimasi hilang.
	if _, err := UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Cat", Qty: 4, Unit: "pail", HpsPrice: 100000, FinalPrice: 80000}},
		}},
	}); err != nil {
		t.Fatalf("isi harga final: %v", err)
	}

	after, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project setelah harga final: %v", err)
	}
	if after.Committed != 320000 {
		t.Errorf("komitmen setelah harga final = %v, mau 320000", after.Committed)
	}
	if after.Estimated != 0 {
		t.Errorf("estimated = %v, mau 0 setelah harga final masuk", after.Estimated)
	}
}

// Master yang seluruh itemnya sudah dipecah adalah cangkang bernilai 0 dan
// tidak boleh ikut menambah jumlah dokumen maupun nilai.
func TestProjectIgnoresFullySplitMaster(t *testing.T) {
	requireDB(t)
	proj := newTestProject(t, 500000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Pasir", Qty: 1, Unit: "truk", HpsPrice: 300000, FinalPrice: 300000}},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1 OR parent_id = $1", pr.ID)

	if _, err := SplitPurchaseRequest(pr.ID, models.SplitPurchaseRequestInput{
		VendorName: "CV Pasir",
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Pasir", Qty: 1, HpsPrice: 300000, FinalPrice: 300000}},
		}},
	}); err != nil {
		t.Fatalf("split: %v", err)
	}

	p, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if p.Committed != 300000 {
		t.Errorf("komitmen = %v, mau 300000 (master kosong tidak ikut dihitung)", p.Committed)
	}
	if p.RequestCount != 1 {
		t.Errorf("jumlah dokumen = %d, mau 1 (hanya pecahan)", p.RequestCount)
	}
}

// Pengajuan yang ditolak/dibatalkan tidak mengunci anggaran.
func TestProjectExcludesCancelledRequests(t *testing.T) {
	requireDB(t)
	proj := newTestProject(t, 200000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "jasa", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{
			Name:  "Upah",
			Items: []models.PurchaseSubItem{{Name: "Tukang", Qty: 1, Unit: "paket", HpsPrice: 150000, FinalPrice: 150000}},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", pr.ID)

	if _, err := UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{
		Action: "cancel", ActorName: "tester", RejectedReason: "batal uji",
	}); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	p, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if p.Committed != 0 {
		t.Errorf("komitmen = %v, mau 0 karena pengajuannya dibatalkan", p.Committed)
	}
	if p.RemainingBudget != 200000 {
		t.Errorf("sisa RAB = %v, mau 200000", p.RemainingBudget)
	}
}

// RAB terlampaui harus ditandai, dan projek yang masih dipakai tidak bisa dihapus.
func TestProjectOverBudgetAndDeleteGuard(t *testing.T) {
	requireDB(t)
	proj := newTestProject(t, 100000)
	defer database.DB.Exec("DELETE FROM projects WHERE id = $1", proj.ID)

	pr, err := CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: "barang", RequestedBy: "tester", ProjectID: proj.ID,
		Items: []models.PurchaseRequestItem{{
			Name:  "Material",
			Items: []models.PurchaseSubItem{{Name: "Besi", Qty: 1, Unit: "btg", HpsPrice: 250000, FinalPrice: 250000}},
		}},
	})
	if err != nil {
		t.Fatalf("create PR: %v", err)
	}
	defer database.DB.Exec("DELETE FROM purchase_requests WHERE id = $1", pr.ID)

	p, err := GetProject(proj.ID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if !p.OverBudget {
		t.Errorf("over_budget = false, mau true (komitmen %v > RAB %v)", p.Committed, p.Budget)
	}
	if p.RemainingBudget != -150000 {
		t.Errorf("sisa RAB = %v, mau -150000", p.RemainingBudget)
	}

	if err := DeleteProject(proj.ID); err == nil {
		t.Error("projek yang masih memayungi pengajuan berhasil dihapus — pengajuannya akan lepas tanpa jejak")
	}
}

package routes

// Uji regresi rute pengadaan untuk role keuangan murni (finance.payments.view
// + finance.payments.pay, tanpa izin procurement.*). Handler sudah dirancang
// untuk mereka — daftar exclude_masters halaman Pembayaran, detail, dan aksi
// pay — tetapi middleware rute dulu mewajibkan procurement.*.view, sehingga
// keuangan kena 403 sebelum aturan handler sempat berlaku dan tagihan tidak
// bisa dibayar sama sekali; rekening vendor untuk modal Bayar juga tertutup
// (ditemukan saat uji E2E 14 Okt 2026).
//
// Butuh Postgres uji (DB_NAME berakhiran _test) yang skemanya sudah dibuat
// RunMigrations; tanpa itu tes di-skip.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

const ruteRahasiaJWT = "rahasia-jwt-uji-rute-keuangan-0123456789"

var ruteDBReady bool

func TestMain(m *testing.M) {
	cfg := config.Load()
	// Hanya database uji (berakhiran _test) yang boleh disentuh.
	if !strings.HasSuffix(cfg.DBName, "_test") {
		cfg.DBHost, cfg.DBPort = "db-uji-dinonaktifkan.invalid", "1"
	}
	if err := database.Connect(cfg); err == nil {
		if err := database.RunAuthMigrations(); err == nil {
			ruteDBReady = true
		}
	}
	os.Exit(m.Run())
}

func requireRuteDB(t *testing.T) {
	t.Helper()
	if !ruteDBReady {
		t.Skip("butuh Postgres uji; set DB_HOST/DB_USER/DB_PASSWORD dan DB_NAME berakhiran _test")
	}
}

func ruteExec(q string, args ...interface{}) { database.DB.Exec(q, args...) }

// ruteAkun membuat role + akun dengan izin tertentu lalu login, mengembalikan JWT.
func ruteAkun(t *testing.T, suffix string, perms []string) (token, name string) {
	t.Helper()
	role := "ujr_" + suffix // cloud_admins.role VARCHAR(20)
	if _, err := services.CreateRole(models.CreateRoleRequest{Name: role, Permissions: perms, ScopeType: "all"}); err != nil {
		t.Fatalf("buat role %s: %v", role, err)
	}
	username := role
	name = "Keuangan Rute " + suffix
	admin, err := services.CreateAdmin(models.CreateAdminRequest{Username: username, Password: "rahasia-uji-123", Name: name, Role: role})
	if err != nil {
		ruteExec(`DELETE FROM role_permissions WHERE role = $1`, role)
		ruteExec(`DELETE FROM roles WHERE name = $1`, role)
		t.Fatalf("buat akun %s: %v", username, err)
	}
	t.Cleanup(func() {
		ruteExec(`DELETE FROM cloud_admins WHERE id = $1`, admin.ID)
		ruteExec(`DELETE FROM role_permissions WHERE role = $1`, role)
		ruteExec(`DELETE FROM roles WHERE name = $1`, role)
	})
	res, err := services.AdminLogin(models.AdminLoginRequest{Username: username, Password: "rahasia-uji-123"}, ruteRahasiaJWT)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return res.Token, name
}

func rutePanggil(t *testing.T, app *fiber.App, method, url, token string, body interface{}) (int, models.APIResponse) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, url, rd)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, 15000)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	data, _ := io.ReadAll(resp.Body)
	var out models.APIResponse
	json.Unmarshal(data, &out)
	return resp.StatusCode, out
}

// tagihanUji: pengajuan jasa pusat sampai 'payment_requested' lewat layanan.
func tagihanUji(t *testing.T, itemName string) string {
	t.Helper()
	items := []models.PurchaseRequestItem{{Name: "Servis Uji Rute", Items: []models.PurchaseSubItem{
		{Name: itemName, Qty: 2, Unit: "unit", HpsPrice: 150000, HpsSource: "uji"},
	}}}
	pr, err := services.CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: services.RequestTypeJasa, RequestedBy: "Pengaju Uji Rute", Items: items,
	})
	if err != nil {
		t.Fatalf("buat pengajuan: %v", err)
	}
	t.Cleanup(func() {
		ruteExec(`DELETE FROM payment_histories WHERE purchase_request_id = $1`, pr.ID)
		ruteExec(`DELETE FROM purchase_requests WHERE id = $1`, pr.ID)
	})
	if _, err := services.UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "verify", ActorName: "Verifikator Uji", Version: pr.Version}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if _, err := services.UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "approve", ActorName: "Penyetuju Uji"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	cur, err := services.GetPurchaseRequest(pr.ID)
	if err != nil {
		t.Fatalf("muat: %v", err)
	}
	cur.Items[0].Items[0].FinalPrice = 140000
	// Pembayaran wajib ke vendor terdaftar — rekeningnya dipotret saat bayar.
	vendor, err := services.CreateVendor(models.CreateVendorRequest{
		Name: "Vendor Uji Rute " + itemName, BankName: "BCA", AccountNumber: "9900112233", AccountHolder: "Vendor Uji Rute",
	})
	if err != nil {
		t.Fatalf("vendor uji: %v", err)
	}
	t.Cleanup(func() { ruteExec(`DELETE FROM vendors WHERE id = $1`, vendor.ID) })
	if _, err := services.UpdatePurchaseItems(pr.ID, models.UpdatePurchaseItemsInput{Items: cur.Items, VendorID: &vendor.ID}); err != nil {
		t.Fatalf("isi harga: %v", err)
	}
	if _, err := services.UpdatePurchaseStatus(pr.ID, models.UpdatePurchaseStatusInput{Action: "request_payment", ActorName: "Pembelian Uji"}); err != nil {
		t.Fatalf("ajukan bayar: %v", err)
	}
	return pr.ID
}

func TestKeuanganMurniBisaMembukaDanMembayarTagihan(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	itemName := "Zzz-Uji-Rute-" + suffix
	prID := tagihanUji(t, itemName)
	bayarTok, bayarNama := ruteAkun(t, "bayar_"+suffix, []string{"finance.payments.view", "finance.payments.pay"})
	lihatTok, _ := ruteAkun(t, "lihat_"+suffix, []string{"finance.payments.view"})

	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	base := "/api/v1/admin/purchase-requests"

	// Halaman Pembayaran: daftar exclude_masters memuat tagihan.
	code, res := rutePanggil(t, app, "GET", base+"?exclude_masters=true&parent_id=all&search="+itemName, bayarTok, nil)
	if code != fiber.StatusOK {
		t.Fatalf("daftar halaman Pembayaran: status %d (%s), mau 200", code, res.Error)
	}
	raw, _ := json.Marshal(res.Data)
	var list models.PurchaseRequestListResponse
	json.Unmarshal(raw, &list)
	if len(list.Requests) != 1 || list.Requests[0].ID != prID {
		t.Fatalf("daftar halaman Pembayaran: %d baris, mau tepat tagihan uji", len(list.Requests))
	}
	// Di luar halaman Pembayaran keuangan murni tidak melihat jenis apa pun.
	code, res = rutePanggil(t, app, "GET", base+"?search="+itemName, bayarTok, nil)
	raw, _ = json.Marshal(res.Data)
	list = models.PurchaseRequestListResponse{}
	json.Unmarshal(raw, &list)
	if code != fiber.StatusOK || len(list.Requests) != 0 {
		t.Errorf("daftar biasa: status %d, %d baris — mau 200 tanpa baris", code, len(list.Requests))
	}
	if code, res = rutePanggil(t, app, "GET", base+"/"+prID, bayarTok, nil); code != fiber.StatusOK {
		t.Errorf("detail: status %d (%s), mau 200", code, res.Error)
	}
	// Modal Bayar memuat rekening vendor tujuan (isi otomatis rekening tujuan).
	v, err := services.CreateVendor(models.CreateVendorRequest{Name: "Vendor Uji Rute " + suffix, BankName: "BCA",
		AccountNumber: "1234567890", AccountHolder: "Vendor Uji"})
	if err != nil {
		t.Fatalf("buat vendor: %v", err)
	}
	t.Cleanup(func() { ruteExec(`DELETE FROM vendors WHERE id = $1`, v.ID) })
	code, res = rutePanggil(t, app, "GET", "/api/v1/admin/vendors/"+v.ID, lihatTok, nil)
	raw, _ = json.Marshal(res.Data)
	var vend models.Vendor
	json.Unmarshal(raw, &vend)
	if code != fiber.StatusOK || vend.AccountNumber != "1234567890" {
		t.Errorf("rekening vendor untuk keuangan: status %d (%s) rekening %q, mau 200 dengan rekening", code, res.Error, vend.AccountNumber)
	}

	// Aksi pengadaan tetap tertutup; bayar butuh finance.payments.pay.
	if code, _ = rutePanggil(t, app, "PUT", base+"/"+prID+"/status", bayarTok, map[string]interface{}{"action": "cancel"}); code != fiber.StatusForbidden {
		t.Errorf("keuangan membatalkan: status %d, mau 403", code)
	}
	if code, _ = rutePanggil(t, app, "PUT", base+"/"+prID+"/status", lihatTok, map[string]interface{}{"action": "pay"}); code != fiber.StatusForbidden {
		t.Errorf("keuangan tanpa izin bayar membayar: status %d, mau 403", code)
	}
	code, res = rutePanggil(t, app, "PUT", base+"/"+prID+"/status", bayarTok, map[string]interface{}{
		"action": "pay", "payment_amount": 0, "actor_name": "Nama Palsu", "payment_notes": "uji rute",
	})
	if code != fiber.StatusOK {
		t.Fatalf("keuangan membayar: status %d (%s), mau 200", code, res.Error)
	}
	pr, err := services.GetPurchaseRequest(prID)
	if err != nil {
		t.Fatalf("muat: %v", err)
	}
	if pr.Status != "paid" || pr.PaidBy == nil || *pr.PaidBy != bayarNama || pr.PaidAmount != 280000 {
		t.Errorf("sesudah bayar: status=%s paid_by=%v paid=%v — mau paid oleh %q sebesar 280000", pr.Status, pr.PaidBy, pr.PaidAmount, bayarNama)
	}
}

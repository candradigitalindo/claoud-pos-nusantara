package routes

// Uji regresi temuan uji UI 14 Okt 2026 di halaman Pembayaran:
//   - role yang hanya diberi hak bayar (finance.payments.pay, tanpa kelola
//     rekening) kena 403 di daftar rekening bank, sehingga form bayar tidak
//     punya pilihan rekening asal dan pembayaran tidak bisa dicatat sama sekali;
//   - pilihan "Semua" di halaman Pembayaran ikut memuat pengajuan yang belum
//     disetujui owner, ditolak, dan batal (bernominal Rp 0) dari semua jenis.

import (
	"encoding/json"
	"strings"
	"testing"

	"cloud-pos/config"
	"cloud-pos/models"
	"cloud-pos/services"

	"github.com/gofiber/fiber/v2"
)

func TestHakBayarBisaMembacaRekeningAsal(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	bayarTok, _ := ruteAkun(t, "rkb_"+suffix, []string{"finance.payments.view", "finance.payments.pay"})
	lihatTok, _ := ruteAkun(t, "rkl_"+suffix, []string{"finance.payments.view"})

	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	url := "/api/v1/admin/bank-accounts"

	if code, res := rutePanggil(t, app, "GET", url, bayarTok, nil); code != fiber.StatusOK {
		t.Errorf("hak bayar membaca daftar rekening: status %d (%s), mau 200", code, res.Error)
	}
	if code, _ := rutePanggil(t, app, "GET", url, lihatTok, nil); code != fiber.StatusForbidden {
		t.Errorf("hanya lihat pembayaran membaca daftar rekening: status %d, mau 403", code)
	}
	// Mengelola rekening tetap butuh izin finance.bank.*.
	body := map[string]string{"bank_name": "BCA", "account_number": "000-uji", "account_holder": "Uji"}
	if code, _ := rutePanggil(t, app, "POST", url, bayarTok, body); code != fiber.StatusForbidden {
		t.Errorf("hak bayar menambah rekening: status %d, mau 403", code)
	}
}

func TestHalamanPembayaranHanyaMemuatPengajuanDisetujui(t *testing.T) {
	requireRuteDB(t)
	suffix := strings.ToLower(services.NewULID()[20:])
	marker := "Zzz-Uji-Bayar-Semua-" + suffix
	tagihanID := tagihanUji(t, marker+"-Tagihan")

	// Pengajuan pending berpenanda sama: belum disetujui owner, bukan tagihan.
	pending, err := services.CreatePurchaseRequest(models.CreatePurchaseRequestInput{
		RequestType: services.RequestTypeJasa, RequestedBy: "Pengaju Uji Rute",
		Items: []models.PurchaseRequestItem{{Name: "Servis Uji Rute", Items: []models.PurchaseSubItem{
			{Name: marker + "-Pending", Qty: 1, Unit: "unit", HpsPrice: 50000, HpsSource: "uji"},
		}}},
	})
	if err != nil {
		t.Fatalf("buat pengajuan pending: %v", err)
	}
	t.Cleanup(func() { ruteExec(`DELETE FROM purchase_requests WHERE id = $1`, pending.ID) })

	tok, _ := ruteAkun(t, "smb_"+suffix, []string{"finance.payments.view"})
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	base := "/api/v1/admin/purchase-requests?exclude_masters=true&parent_id=all&search=" + marker

	code, res := rutePanggil(t, app, "GET", base, tok, nil)
	if code != fiber.StatusOK {
		t.Fatalf("daftar halaman Pembayaran (Semua): status %d (%s), mau 200", code, res.Error)
	}
	raw, _ := json.Marshal(res.Data)
	var list models.PurchaseRequestListResponse
	json.Unmarshal(raw, &list)
	if len(list.Requests) != 1 || list.Requests[0].ID != tagihanID {
		ids := []string{}
		for _, r := range list.Requests {
			ids = append(ids, r.ID+"/"+r.Status)
		}
		t.Errorf("pilihan Semua memuat %v, mau hanya tagihan %s (pengajuan pending bukan tagihan)", ids, tagihanID)
	}

	if code, _ := rutePanggil(t, app, "GET", base+"&status=pending", tok, nil); code != fiber.StatusBadRequest {
		t.Errorf("halaman Pembayaran dengan status pending: status %d, mau 400", code)
	}
	if code, _ := rutePanggil(t, app, "GET", "/api/v1/admin/procurement-payments/export?status=verified", tok, nil); code != fiber.StatusBadRequest {
		t.Errorf("ekspor Pembayaran dengan status verified: status %d, mau 400", code)
	}
	// Filter Projek di halaman Pembayaran memuat daftar projek untuk keuangan
	// murni (dulu 403 sehingga pilihannya hanya "Semua Projek").
	if code, res := rutePanggil(t, app, "GET", "/api/v1/admin/projects", tok, nil); code != fiber.StatusOK {
		t.Errorf("daftar projek untuk filter halaman Pembayaran: status %d (%s), mau 200", code, res.Error)
	}
	// Filter status berkomitmen tetap berjalan seperti biasa.
	code, res = rutePanggil(t, app, "GET", base+"&status=payment_requested", tok, nil)
	raw, _ = json.Marshal(res.Data)
	list = models.PurchaseRequestListResponse{}
	json.Unmarshal(raw, &list)
	if code != fiber.StatusOK || len(list.Requests) != 1 {
		t.Errorf("filter Menunggu Pembayaran: status %d, %d baris — mau 200 dengan tepat 1 tagihan", code, len(list.Requests))
	}
}

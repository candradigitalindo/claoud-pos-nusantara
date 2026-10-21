package handlers

import (
	"bytes"
	"cloud-pos/models"
	"cloud-pos/services"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func uploadQualityPhoto(t *testing.T, app *fiber.App, warehouse string) (int, models.StockEvidence) {
	t.Helper()
	var imageData, body bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 40, 40))); err != nil {
		t.Fatal(err)
	}
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "bukti.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(imageData.Bytes())
	writer.WriteField("warehouse_id", warehouse)
	writer.WriteField("actor_id", "palsu")
	writer.WriteField("recorded_at", "1999-01-01T00:00:00Z")
	writer.Close()
	req := httptest.NewRequest("POST", "/stock-evidence", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var wrapped struct {
		Success bool                 `json:"success"`
		Data    models.StockEvidence `json:"data"`
		Error   string               `json:"error"`
	}
	if err = json.NewDecoder(res.Body).Decode(&wrapped); err != nil {
		t.Fatal(err)
	}
	if wrapped.Data.URL != "" {
		t.Cleanup(func() { p, _ := services.UploadFilePath(wrapped.Data.URL); os.Remove(p) })
	}
	return res.StatusCode, wrapped.Data
}

func receiveTransferWithPhoto(t *testing.T, app *fiber.App, id string) (int, string) {
	t.Helper()
	doc, err := services.GetStockTransfer(id)
	if err != nil {
		t.Fatal(err)
	}
	code, photo := uploadQualityPhoto(t, app, doc.ToWarehouseID)
	if code != 200 {
		t.Fatalf("unggah bukti: %d", code)
	}
	req := models.StockInspectionRequest{Version: doc.ReceiptVersion, EvidenceID: photo.ID}
	for _, it := range doc.Items {
		req.Items = append(req.Items, models.StockInspectionItem{ItemID: it.ItemID, Accepted: it.QtyBase - it.AcceptedQtyBase - it.RejectedQtyBase})
	}
	code, res := doJSON(t, app, "POST", "/stock-transfers/"+id+"/receive", req)
	return code, res.Error
}

func TestStockEvidenceHandlerScopeAndServerIdentity(t *testing.T) {
	requireHandlerDB(t)
	f := newStockFlowFixture(t)
	outlet, _ := stockFlowUsers(t, f, "stockquality.outlet.report")
	code, photo := uploadQualityPhoto(t, outlet, f.whA)
	if code != 200 || photo.ActorName != "kepala_dapur_a" || photo.RecordedAt == "1999-01-01T00:00:00Z" {
		t.Fatalf("identitas/waktu server: %d %+v", code, photo)
	}
	for _, wh := range []string{f.whB, f.whC} {
		if code, _ := uploadQualityPhoto(t, outlet, wh); code != 403 {
			t.Errorf("unggah lintas gudang: %d", code)
		}
	}
}

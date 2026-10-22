package routes

import (
	"bytes"
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func documentRequest(app *fiber.App, method, path, token string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp, data, err
}

func TestDocumentsUploadDownloadPermissionsAndCounter(t *testing.T) {
	requireRuteDB(t)
	if err := database.RunDocumentMigrations(); err != nil {
		t.Fatal(err)
	}
	oldDir := services.DocumentsDir
	services.DocumentsDir = t.TempDir()
	t.Cleanup(func() { services.DocumentsDir = oldDir })
	suffix := strings.ToLower(services.NewULID()[19:])
	number := "DOC/" + suffix
	t.Cleanup(func() { ruteExec(`DELETE FROM documents WHERE lower(document_number)=lower($1)`, number) })
	owner, _ := ruteAkun(t, suffix+"a", []string{"documents.view", "documents.create", "documents.download"})
	viewer, _ := ruteAkun(t, suffix+"v", []string{"documents.view"})
	denied, _ := ruteAkun(t, suffix+"n", nil)
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	request := func(method, path, token string) (*http.Response, []byte) {
		t.Helper()
		resp, data, err := documentRequest(app, method, path, token, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return resp, data
	}
	upload := func(token, num, about, name, content string) (*http.Response, []byte) {
		t.Helper()
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		form.WriteField("document_number", num)
		form.WriteField("about", about)
		form.WriteField("uploaded_by", "pemalsu")
		form.WriteField("download_count", "999")
		form.WriteField("uploaded_at", "1999-01-01T00:00:00Z")
		part, err := form.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte(content))
		form.Close()
		resp, data, err := documentRequest(app, "POST", "/api/v1/admin/documents", token, &body, form.FormDataContentType())
		if err != nil {
			t.Fatal(err)
		}
		return resp, data
	}
	for _, tc := range []struct {
		token  string
		status int
	}{{"", 401}, {denied, 403}} {
		if resp, data := request("GET", "/api/v1/admin/documents", tc.token); resp.StatusCode != tc.status {
			t.Fatalf("akses daftar: %d %s", resp.StatusCode, data)
		}
	}
	if resp, _ := upload(viewer, number, "SOP pembukaan outlet", "sop.pdf", "%PDF-1.7\nuji"); resp.StatusCode != 403 {
		t.Fatalf("viewer bisa unggah: %d", resp.StatusCode)
	}
	for _, tc := range []struct{ number, about, name, content string }{
		{"", "SOP", "sop.pdf", "%PDF-1.7\nuji"}, {number, " ", "sop.pdf", "%PDF-1.7\nuji"},
		{number, "SOP", "sop.exe", "program"}, {number, "SOP", "sop.pdf", "<html>palsu</html>"},
		{number, "SOP", "sop.txt", ""},
	} {
		if resp, data := upload(owner, tc.number, tc.about, tc.name, tc.content); resp.StatusCode != 400 {
			t.Fatalf("validasi %s: %d %s", tc.name, resp.StatusCode, data)
		}
	}
	const content = "%PDF-1.7\nDokumen uji\n%%EOF"
	start := time.Now().Add(-time.Second)
	resp, data := upload(owner, " "+number+" ", "SOP pembukaan outlet", `C:\folder\Panduan cabang.pdf`, content)
	if resp.StatusCode != 201 {
		t.Fatalf("unggah: %d %s", resp.StatusCode, data)
	}
	var result struct {
		Data models.Document `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	d := result.Data
	if d.DocumentNumber != number || d.FileName != "Panduan cabang.pdf" || d.DownloadCount != 0 || d.UploadedAt.Before(start) || d.UploadedBy != "ujr_"+suffix+"a" || d.FileSize != int64(len(content)) {
		t.Fatalf("metadata berasal dari server: %+v", d)
	}
	if resp, data := upload(owner, strings.ToLower(number), "Duplikat", "lain.pdf", content); resp.StatusCode != 409 {
		t.Fatalf("duplikat: %d %s", resp.StatusCode, data)
	}
	entries, err := os.ReadDir(services.DocumentsDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("berkas unggahan gagal tertinggal: %v %v", entries, err)
	}
	path := "/api/v1/admin/documents/" + d.ID + "/download"
	if resp, _ := request("POST", path, viewer); resp.StatusCode != 403 {
		t.Fatalf("viewer bisa unduh: %d", resp.StatusCode)
	}
	for _, method := range []string{"HEAD", "GET"} {
		if resp, _ := request(method, path, owner); resp.StatusCode < 400 {
			t.Fatalf("%s tidak boleh mengunduh", method)
		}
	}
	checkCount := func(want int64) {
		t.Helper()
		var got int64
		if err := database.DB.QueryRow(`SELECT download_count FROM documents WHERE id=$1`, d.ID).Scan(&got); err != nil || got != want {
			t.Fatalf("download_count=%d ingin %d: %v", got, want, err)
		}
	}
	resp, data = request("GET", "/api/v1/admin/documents?search="+suffix+"&limit=1", viewer)
	var list struct {
		Data struct {
			Items []models.Document `json:"items"`
			Total int               `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &list); err != nil || resp.StatusCode != 200 || list.Data.Total != 1 || len(list.Data.Items) != 1 {
		t.Fatalf("pencarian: %d %s %v", resp.StatusCode, data, err)
	}
	checkCount(0)
	resp, data = request("POST", path, owner)
	if resp.StatusCode != 200 || string(data) != content || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment;") || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("isi unduhan: %d %s %v", resp.StatusCode, data, resp.Header)
	}
	checkCount(1)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, b, err := documentRequest(app, "POST", path, owner, nil, "")
			if err == nil && (r.StatusCode != 200 || string(b) != content) {
				err = fmt.Errorf("unduh bersamaan: %d %s", r.StatusCode, b)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	checkCount(13)
	if err := os.Remove(filepath.Join(services.DocumentsDir, d.ID)); err != nil {
		t.Fatal(err)
	}
	if resp, _ := request("POST", path, owner); resp.StatusCode != 404 {
		t.Fatalf("berkas hilang: %d", resp.StatusCode)
	}
	if resp, _ := request("POST", "/api/v1/admin/documents/bad-id/download", owner); resp.StatusCode != 404 {
		t.Fatalf("ID tidak valid: %d", resp.StatusCode)
	}
	checkCount(13)
	if err := database.RunDocumentMigrations(); err != nil {
		t.Fatal(err)
	}
	checkCount(13)
}

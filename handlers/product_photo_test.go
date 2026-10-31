package handlers

import (
	"bytes"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func productPhotoFixture(t *testing.T) (*fiber.App, string, string) {
	t.Helper()
	requireHandlerDB(t)
	previousDir := productPhotoDir
	productPhotoDir = t.TempDir()
	t.Cleanup(func() { productPhotoDir = previousDir })
	outlet := services.NewULID()
	if _, err := database.DB.Exec(`INSERT INTO outlets(id,code,name,api_key) VALUES($1,$2,'Photo fixture',$3)`,
		outlet, "PHOTO"+outlet[11:], "photo-test-"+outlet); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, table := range []string{"cloud_products", "sync_logs", "pos_entity_ids"} {
			if _, err := database.DB.Exec(`DELETE FROM `+table+` WHERE outlet_id=$1`, outlet); err != nil {
				t.Error(err)
			}
		}
		if _, err := database.DB.Exec(`DELETE FROM outlets WHERE id=$1`, outlet); err != nil {
			t.Error(err)
		}
	})
	product, err := services.SaveProduct(outlet, models.PushProductRequest{LocalID: services.NewULID(), Name: "Menu foto", Price: 12000, UpdatedAt: "2020-01-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{outlet})
		c.Locals("outlet_id", outlet)
		return c.Next()
	})
	app.Post("/products/:id/photo", UploadProductPhoto)
	app.Delete("/products/:id/photo", DeleteProductPhoto)
	app.Get("/uploads/products/:name", ServeProductPhoto)
	app.Get("/uploads/products/*", ServeProductPhoto)
	app.Get("/catalog", GetProducts)
	app.Get("/updates", GetUpdates)
	return app, outlet, strings.TrimSpace(product)
}

func uploadMenuPhoto(t *testing.T, app *fiber.App, product, filename string, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/products/"+product+"/photo", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Data struct {
			PhotoURL string `json:"photo_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, result.Data.PhotoURL
}

func menuPhotoPNG(t *testing.T) []byte {
	t.Helper()
	var body bytes.Buffer
	if err := png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func assertProductPhotoGET(t *testing.T, app *fiber.App, photoURL string, status int, want []byte) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", photoURL, nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != status {
		t.Fatalf("photo GET %s: status=%d want=%d error=%v", photoURL, resp.StatusCode, status, err)
	}
	if status == 200 && (resp.Header.Get("Content-Type") != "image/png" || !bytes.Equal(body, want)) {
		t.Fatalf("photo GET %s: incorrect image type or content", photoURL)
	}
}

func assertCatalogPhoto(t *testing.T, app *fiber.App, localID, since, want string) {
	t.Helper()
	for _, endpoint := range []string{"/catalog", "/updates?since=" + url.QueryEscape(since)} {
		resp, err := app.Test(httptest.NewRequest("GET", endpoint, nil), -1)
		if err != nil {
			t.Fatal(err)
		}
		var root map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&root)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%s: status=%d error=%v body=%v", endpoint, resp.StatusCode, err, root)
		}
		data := root["data"]
		if strings.HasPrefix(endpoint, "/updates") {
			data = data.(map[string]interface{})["products"]
		}
		found := false
		for _, entry := range data.([]interface{}) {
			p := entry.(map[string]interface{})
			if p["local_id"] == localID {
				found = true
				photo, exists := p["photo_url"]
				if !exists || photo != want {
					t.Fatalf("%s photo_url=%v present=%v want %q", endpoint, photo, exists, want)
				}
			}
		}
		if !found {
			t.Fatalf("%s missing updated product %s", endpoint, localID)
		}
	}
}

func TestProductPhotoUploadReplaceDeleteSyncAndPOSOwnership(t *testing.T) {
	app, outlet, product := productPhotoFixture(t)
	since := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
	code, first := uploadMenuPhoto(t, app, product, "menu.png", menuPhotoPNG(t))
	if code != 200 || first == "" {
		t.Fatalf("upload: %d %q", code, first)
	}
	assertCatalogPhoto(t, app, product, since, first)
	assertProductPhotoGET(t, app, first, 200, menuPhotoPNG(t))
	code, replacement := uploadMenuPhoto(t, app, product, "menu.png", menuPhotoPNG(t))
	if code != 200 || replacement == first || replacement == "" {
		t.Fatalf("replacement must change cache URL: %d %q", code, replacement)
	}
	if _, err := os.Stat(filepath.Join(productPhotoDir, filepath.Base(first))); !os.IsNotExist(err) {
		t.Fatalf("replaced photo retained: %v", err)
	}
	assertCatalogPhoto(t, app, product, since, replacement)
	assertProductPhotoGET(t, app, first, 404, nil)
	assertProductPhotoGET(t, app, replacement, 200, menuPhotoPNG(t))
	// Photo ownership stays in cloud, even if an older/modified POS includes
	// an empty or forged photo_url while pushing an ordinary product edit.
	for _, photo := range []string{"", "/uploads/products/forged.png"} {
		body, _ := json.Marshal(map[string]interface{}{"local_id": product, "name": "Harga dari POS", "price": 15000, "photo_url": photo})
		var push models.PushProductRequest
		if err := json.Unmarshal(body, &push); err != nil {
			t.Fatal(err)
		}
		if _, err := services.SaveProduct(outlet, push); err != nil {
			t.Fatal(err)
		}
		assertCatalogPhoto(t, app, product, since, replacement)
	}
	resp, err := app.Test(httptest.NewRequest("DELETE", "/products/"+product+"/photo", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("delete status %d", resp.StatusCode)
	}
	assertCatalogPhoto(t, app, product, since, "")
	assertProductPhotoGET(t, app, replacement, 404, nil)
	if _, err := os.Stat(filepath.Join(productPhotoDir, filepath.Base(replacement))); !os.IsNotExist(err) {
		t.Fatalf("deleted photo retained: %v", err)
	}
}

func TestProductPhotoGETUsesCurrentFileAndRejectsTraversal(t *testing.T) {
	previousDir := productPhotoDir
	root := t.TempDir()
	productPhotoDir = filepath.Join(root, "products")
	t.Cleanup(func() { productPhotoDir = previousDir })
	if err := os.Mkdir(productPhotoDir, 0700); err != nil {
		t.Fatal(err)
	}
	pngData := menuPhotoPNG(t)
	for _, path := range []string{filepath.Join(productPhotoDir, "menu.png"), filepath.Join(productPhotoDir, "private.txt"), filepath.Join(root, "outside.png")} {
		if err := os.WriteFile(path, pngData, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "outside.png"), filepath.Join(productPhotoDir, "linked.png")); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/uploads/products/:name", ServeProductPhoto)
	app.Get("/uploads/products/*", ServeProductPhoto)
	// A later static handler must never get a missing or rejected photo path.
	app.Use("/uploads", func(c *fiber.Ctx) error { return c.SendStatus(418) })
	assertProductPhotoGET(t, app, "/uploads/products/menu.png", 200, pngData)
	if err := os.Remove(filepath.Join(productPhotoDir, "menu.png")); err != nil {
		t.Fatal(err)
	}
	assertProductPhotoGET(t, app, "/uploads/products/menu.png", 404, nil)
	for _, path := range []string{
		"/uploads/products/", "/uploads/products/private.txt", "/uploads/products/linked.png",
		"/uploads/products/nested/outside.png", "/uploads/products/..%2foutside.png",
		"/uploads/products/%2e%2e%5coutside.png",
	} {
		assertProductPhotoGET(t, app, path, 404, nil)
	}
}

func TestProductPhotoRejectsInvalidImageAndOtherOutlet(t *testing.T) {
	app, _, product := productPhotoFixture(t)
	if code, _ := uploadMenuPhoto(t, app, product, "not-image.png", []byte("not a PNG")); code != 400 {
		t.Fatalf("non-image accepted: %d", code)
	}
	if code, _ := uploadMenuPhoto(t, app, product, "script.html", menuPhotoPNG(t)); code != 400 {
		t.Fatalf("wrong extension accepted: %d", code)
	}
	foreign := fiber.New()
	foreign.Use(func(c *fiber.Ctx) error {
		c.Locals("admin_scope", "specific")
		c.Locals("admin_outlet_ids", []string{services.NewULID()})
		return c.Next()
	})
	foreign.Post("/products/:id/photo", UploadProductPhoto)
	foreign.Delete("/products/:id/photo", DeleteProductPhoto)
	if code, _ := uploadMenuPhoto(t, foreign, product, "menu.png", menuPhotoPNG(t)); code != 403 {
		t.Fatalf("cross-outlet upload accepted: %d", code)
	}
	resp, err := foreign.Test(httptest.NewRequest("DELETE", "/products/"+product+"/photo", nil), -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("cross-outlet delete accepted: %d", resp.StatusCode)
	}
	files, err := os.ReadDir(productPhotoDir)
	if err != nil || len(files) != 0 {
		t.Fatalf("rejected upload left files: %d %v", len(files), err)
	}
}

func TestProductPhotoDatabaseFailureCleansNewFileAndKeepsPrevious(t *testing.T) {
	app, _, product := productPhotoFixture(t)
	code, previous := uploadMenuPhoto(t, app, product, "old.png", menuPhotoPNG(t))
	if code != 200 {
		t.Fatalf("initial upload: %d", code)
	}
	constraint := "photo_failure_" + strings.ToLower(services.NewULID())
	// Synthetic-only, row-scoped fault injection. The existing photo remains
	// valid; any replacement for this test product fails after SaveFile.
	ddl := fmt.Sprintf(`ALTER TABLE cloud_products ADD CONSTRAINT %s CHECK (id <> '%s' OR photo_url = '%s') NOT VALID`, constraint, product, previous)
	if _, err := database.DB.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := database.DB.Exec(`ALTER TABLE cloud_products DROP CONSTRAINT ` + constraint); err != nil {
			t.Error(err)
		}
	})
	if code, _ := uploadMenuPhoto(t, app, product, "new.png", menuPhotoPNG(t)); code != 500 {
		t.Fatalf("injected database failure: %d", code)
	}
	files, err := os.ReadDir(productPhotoDir)
	if err != nil || len(files) != 1 || files[0].Name() != filepath.Base(previous) {
		t.Fatalf("failed replacement changed files: %v %v", files, err)
	}
	assertCatalogPhoto(t, app, product, "1970-01-01T00:00:00Z", previous)
}

func TestProductPhotoCleanupStaysWithinOwnedFiles(t *testing.T) {
	previousDir := productPhotoDir
	productPhotoDir = t.TempDir()
	t.Cleanup(func() { productPhotoDir = previousDir })
	id := services.NewULID()
	for _, name := range []string{id + "-owned.png", "other-product.png"} {
		if err := os.WriteFile(filepath.Join(productPhotoDir, name), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"/uploads/products/other-product.png", "/uploads/products/../other-product.png", "/uploads/other-product.png", "https://example.test/photo.png"} {
		removeProductPhoto(raw, id)
	}
	if _, err := os.Stat(filepath.Join(productPhotoDir, "other-product.png")); err != nil {
		t.Fatalf("cleanup removed unrelated file: %v", err)
	}
	removeProductPhoto("/uploads/products/"+id+"-owned.png", id)
	if _, err := os.Stat(filepath.Join(productPhotoDir, id+"-owned.png")); !os.IsNotExist(err) {
		t.Fatalf("owned photo not removed: %v", err)
	}
}

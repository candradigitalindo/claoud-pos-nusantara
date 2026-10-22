package routes

import (
	"bytes"
	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"
	"cloud-pos/services"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestProjectUsagePermissionsScopeAndExport(t *testing.T) {
	requireRuteDB(t)
	if err := database.RunProjectUsageMigrations(); err != nil {
		t.Fatal(err)
	}
	sfx := strings.ToLower(services.NewULID()[20:])
	creator, _ := ruteAkun(t, sfx+"c", []string{"assets.usage.create"})
	viewer, _ := ruteAkun(t, sfx+"v", []string{"assets.usage.view"})
	reviewer, _ := ruteAkun(t, sfx+"r", []string{"assets.usage.verify"})
	denied, _ := ruteAkun(t, sfx+"n", []string{"procurement.projects.manage"})
	scoped, _ := ruteAkun(t, sfx+"s", []string{"assets.usage.create", "assets.usage.verify"})
	if err := services.UpdateRoleScope("ujr_"+sfx+"s", models.UpdateRoleScopeRequest{ScopeType: "specific"}); err != nil {
		t.Fatal(err)
	}
	login, err := services.AdminLogin(models.AdminLoginRequest{Username: "ujr_" + sfx + "s", Password: "rahasia-uji-123"}, ruteRahasiaJWT)
	if err != nil {
		t.Fatal(err)
	}
	scoped = login.Token
	p, err := services.CreateProject(models.CreateProjectRequest{Name: "Projek uji akses " + sfx, Status: "berjalan"}, "tester")
	if err != nil {
		t.Fatal(err)
	}
	mid := services.NewULID()
	if _, err = database.DB.Exec(`INSERT INTO project_materials(id,project_id,name,unit,qty_received) VALUES($1,$2,'Cat','liter',10)`, mid, p.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ruteExec(`DELETE FROM project_usage_reports WHERE project_id=$1`, p.ID)
		ruteExec(`DELETE FROM project_materials WHERE project_id=$1`, p.ID)
		ruteExec(`DELETE FROM projects WHERE id=$1`, p.ID)
	})
	os.MkdirAll("uploads", 0755)
	dir, err := os.MkdirTemp("uploads", "usage-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	image, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aJ9sAAAAASUVORK5CYII=")
	os.WriteFile(filepath.Join(dir, "photo.png"), image, 0600)
	in := models.ProjectUsageInput{ProjectID: p.ID, Kind: "pemakaian", UsageDate: time.Now().Format("2006-01-02"), Location: "Dapur", WorkDescription: "Cat dinding", Performer: "Tukang", PICName: "PIC", Items: []models.ProjectUsageItem{{MaterialID: mid, Qty: 2}}, Evidence: []string{"/" + filepath.ToSlash(filepath.Join(dir, "photo.png"))}}
	app := fiber.New()
	Setup(app, &config.Config{JWTSecret: ruteRahasiaJWT})
	call := func(method, path, token string, input interface{}) (int, []byte) {
		t.Helper()
		b, _ := json.Marshal(input)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return resp.StatusCode, buf.Bytes()
	}
	for _, token := range []string{viewer, reviewer, denied, scoped} {
		if code, body := call("POST", "/api/v1/admin/project-usage", token, in); code != 403 {
			t.Fatalf("create should fail: %d %s", code, body)
		}
	}
	code, body := call("POST", "/api/v1/admin/project-usage", creator, in)
	if code != 200 {
		t.Fatalf("create %d %s", code, body)
	}
	var result struct {
		Data models.ProjectUsageReport `json:"data"`
	}
	json.Unmarshal(body, &result)
	r := result.Data
	if r.CreatedByID == "" || r.CreatedBy == "" || r.Version != 1 {
		t.Fatalf("actor/version missing: %+v", r)
	}
	for _, path := range []string{"/api/v1/admin/project-usage/projects", "/api/v1/admin/project-usage/projects/" + p.ID, "/api/v1/admin/project-usage/" + r.ID, "/api/v1/admin/project-usage?project_id=" + p.ID, "/api/v1/admin/projects/" + p.ID + "/materials"} {
		if c, b := call("GET", path, viewer, nil); c != 200 {
			t.Fatalf("view %s %d %s", path, c, b)
		}
	}
	for _, path := range []string{"/api/v1/admin/project-usage/projects/" + p.ID, "/api/v1/admin/project-usage/" + r.ID} {
		if c, b := call("GET", path, scoped, nil); c != 403 {
			t.Fatalf("scope %d %s", c, b)
		}
	}
	if c, b := call("GET", "/api/v1/admin/project-usage?project_id="+p.ID, scoped, nil); c != 200 || !bytes.Contains(b, []byte(`"total":0`)) {
		t.Fatalf("scoped list leaks %d %s", c, b)
	}
	action := models.ProjectUsageAction{Action: "submit", Version: r.Version}
	if c, b := call("POST", "/api/v1/admin/project-usage/"+r.ID+"/actions", viewer, action); c != 403 {
		t.Fatalf("viewer action %d %s", c, b)
	}
	code, body = call("POST", "/api/v1/admin/project-usage/"+r.ID+"/actions", creator, action)
	if code != 200 {
		t.Fatalf("submit %d %s", code, body)
	}
	json.Unmarshal(body, &result)
	r = result.Data
	action = models.ProjectUsageAction{Action: "verify", Version: r.Version}
	if c, b := call("POST", "/api/v1/admin/project-usage/"+r.ID+"/actions", creator, action); c != 403 {
		t.Fatalf("creator verifies %d %s", c, b)
	}
	if c, b := call("POST", "/api/v1/admin/project-usage/"+r.ID+"/actions", reviewer, action); c != 200 {
		t.Fatalf("reviewer %d %s", c, b)
	}
	if c, b := call("GET", "/api/v1/admin/project-usage-export?project_id="+p.ID, viewer, nil); c != 200 || !bytes.HasPrefix(b, []byte("PK")) {
		t.Fatalf("export %d %.100s", c, b)
	}
	if c, b := call("POST", "/api/v1/admin/projects/"+p.ID+"/materials/"+mid+"/usage", denied, models.ProjectMaterialUsageRequest{Qty: 1}); c != 403 {
		t.Fatalf("old permission bypass %d %s", c, b)
	}
}

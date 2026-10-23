package database

import (
	"cloud-pos/config"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestDocumentMigrationGrantsReadAccessToAllRoles(t *testing.T) {
	cfg := config.Load()
	if !strings.HasSuffix(cfg.DBName, "_test") {
		t.Skip("butuh Postgres uji dengan DB_NAME berakhiran _test")
	}
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}

	// Isolate the migration marker and permissions from other integration tests.
	schema := pq.QuoteIdentifier(fmt.Sprintf("documents_migration_%d", time.Now().UnixNano()))
	if _, err := db.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
	if _, err := db.Exec(`SET search_path TO ` + schema); err != nil {
		t.Fatal(err)
	}
	previousDB := DB
	DB = db
	defer func() { DB = previousDB }()
	if _, err := db.Exec(`
		CREATE TABLE roles (name VARCHAR(50) PRIMARY KEY);
		CREATE TABLE role_permissions (role VARCHAR(50), permission VARCHAR(50), UNIQUE(role,permission));
		CREATE TABLE app_settings (key VARCHAR(100) PRIMARY KEY, value TEXT NOT NULL DEFAULT '');
		INSERT INTO roles(name) VALUES ('admin'),('superadmin'),('manager'),('viewer'),
			('custom_empty'),('custom_uploader'),('finance_pencatat'),('finance_supervisor');
		INSERT INTO role_permissions(role,permission) VALUES
			('manager','dashboard'),('viewer','documents.view'),('custom_uploader','documents.create');
	`); err != nil {
		t.Fatal(err)
	}
	if err := RunDocumentMigrations(); err != nil {
		t.Fatal(err)
	}
	checkPermissions := func(role string, want []string) {
		t.Helper()
		rows, err := db.Query(`SELECT permission FROM role_permissions WHERE role=$1 ORDER BY permission`, role)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var permission string
			if err := rows.Scan(&permission); err != nil {
				t.Fatal(err)
			}
			got = append(got, permission)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("role %s: permissions=%v, want %v", role, got, want)
		}
	}
	for _, role := range []string{"admin", "superadmin", "custom_uploader"} {
		checkPermissions(role, []string{"documents.create", "documents.download", "documents.view"})
	}
	for _, role := range []string{"viewer", "custom_empty", "finance_pencatat", "finance_supervisor"} {
		checkPermissions(role, []string{"documents.download", "documents.view"})
	}
	checkPermissions("manager", []string{"dashboard", "documents.download", "documents.view"})

	// A later edit in the role editor must not be undone on the next startup.
	if _, err := db.Exec(`DELETE FROM role_permissions WHERE role='viewer' AND permission='documents.download'`); err != nil {
		t.Fatal(err)
	}
	if err := RunDocumentMigrations(); err != nil {
		t.Fatal(err)
	}
	checkPermissions("viewer", []string{"documents.view"})
	checkPermissions("custom_empty", []string{"documents.download", "documents.view"})
	checkPermissions("manager", []string{"dashboard", "documents.download", "documents.view"})
}

package services

import (
	"cloud-pos/database"
	"cloud-pos/models"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// AllPermissions returns the full list of available permission keys.
// CRUD structure: .view=Lihat, .create=Tambah, .update=Ubah, .delete=Hapus
// dashboard and reports are view-only; procurement and settings have custom granularity.
var AllPermissions = []string{
	"dashboard",

	// Master Data
	"outlets.view", "outlets.create", "outlets.update", "outlets.delete",
	"workunits.view", "workunits.create", "workunits.update", "workunits.delete",
	"appfiles.view", "appfiles.create", "appfiles.delete",
	"assets.view", "assets.create", "assets.update", "assets.delete",
	"assets.transfer.view", "assets.transfer.create", "assets.transfer.approve", "assets.transfer.receive",
	"assets.maintenance.view", "assets.maintenance.create",
	"assets.disposal.view", "assets.disposal.create", "assets.disposal.approve",
	"assets.opname.view", "assets.opname.create", "assets.opname.approve",
	"assets.report.view",

	// Produk
	"products.view", "products.create", "products.update", "products.delete",

	// Penjualan
	"reservations.view", "reservations.create", "reservations.update", "reservations.delete",
	"customers.view",

	// Laporan (view-only) + finance
	"reports.sales.view",
	"reports.product_sales.view",
	"reports.ledger.view",
	"reports.cashflow.view",
	"reports.pnl.view",
	"reports.balance.view",
	"reports.tax.view",
	"reports.void.view",
	// Titipan dipisah dari void: halaman sama (tab), izin berbeda.
	"reports.titipan.view",
	"reports.discount.view",
	// Analisa Bisnis (RGI): lintas outlet, tidak bisa di-scope per outlet.
	"reports.business_analysis.view",
	// Kinerja Markom — grafik IG/TikTok yang menempel di Analisa Bisnis.
	// .manage = mendaftarkan akun outlet, mengetik tambalan mingguan, dan
	// menarik paksa dari halaman publik; pekerjaan Markom, bukan pembaca laporan.
	"social.view", "social.manage",
	"cashier_shifts.view",
	// Rekonsiliasi shift dipisah dari laporan shift kasir.
	"shift_reconciliation.view",
	"finance.payments.view",
	// Mencairkan uang dipisah dari sekadar melihat halaman Pembayaran.
	"finance.payments.pay",
	"finance.bank.view", "finance.bank.create", "finance.bank.update", "finance.bank.delete",

	// Pengadaan
	"procurement.dashboard.view",
	"procurement.requests.view", "procurement.requests.submit", "procurement.requests.approve", "procurement.requests.purchasing",
	// Projek pembangunan/renovasi: payung RAB di atas pengadaan.
	// .manage = buat/ubah/hapus projek dan menetapkan RAB-nya.
	"procurement.projects.view", "procurement.projects.manage",
	"vendors.view", "vendors.create", "vendors.update", "vendors.delete",

	// Pengguna & Role
	"users.view", "users.create", "users.update", "users.delete",
	"roles.view", "roles.create", "roles.update", "roles.delete",
	"access_logs.view",

	// Gudang
	"warehouse_dashboard.view",
	"warehouses.view", "warehouses.create", "warehouses.update", "warehouses.delete",
	"stockitems.view", "stockitems.create", "stockitems.update", "stockitems.delete",
	"stocktransfers.view", "stocktransfers.create", "stocktransfers.update",
	"stockwastes.view", "stockwastes.create",
	"stockledger.view", "stockledger.adjust",
	"recipes.view", "recipes.create", "recipes.update", "recipes.delete",

	// PPIC — Fase 1: pengendalian (dashboard, par level/ROP, kedaluwarsa, opname)
	"ppic.dashboard.view",
	"ppic.planning.view", "ppic.planning.update",
	"ppic.expiry.view", "ppic.expiry.ack",
	"ppic.opname.view", "ppic.opname.create", "ppic.opname.approve",
	// PPIC — Fase 2: perencanaan (demand forecast + MRP)
	"ppic.forecast.view", "ppic.forecast.manage",
	"ppic.mrp.view", "ppic.mrp.run", "ppic.mrp.execute",
	// PPIC — Fase 3: produksi (MPS + work order) & laporan
	"ppic.production.view", "ppic.production.create", "ppic.production.approve",
	"ppic.workorders.view", "ppic.workorders.create", "ppic.workorders.execute",
	"ppic.reports.view", "ppic.reports.export",

	// Pengaturan
	"settings.company.view", "settings.company.update",
	"settings.timezone.view", "settings.timezone.update",
	"settings.tax.view", "settings.tax.update",
	"devices.view", // Monitoring perangkat (heartbeat tablet kasir)

	// CCTV — lihat live/putar ulang (view) + kelola kamera (create/update/delete)
	"cameras.view", "cameras.create", "cameras.update", "cameras.delete",
}

func AdminLogin(req models.AdminLoginRequest, jwtSecret string) (*models.AdminLoginResponse, error) {
	var admin models.CloudAdmin
	var passwordHash string
	var lastLogin sql.NullTime

	err := database.DB.QueryRow(
		`SELECT id, username, password_hash, name, role, is_active, last_login_at, created_at, updated_at
		FROM cloud_admins WHERE username = $1`,
		req.Username,
	).Scan(&admin.ID, &admin.Username, &passwordHash, &admin.Name, &admin.Role,
		&admin.IsActive, &lastLogin, &admin.CreatedAt, &admin.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("username atau password salah")
	}
	if err != nil {
		return nil, err
	}

	// Trim CHAR(26) whitespace
	admin.ID = strings.TrimSpace(admin.ID)

	if !admin.IsActive {
		return nil, fmt.Errorf("akun admin tidak aktif")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("username atau password salah")
	}

	if lastLogin.Valid {
		admin.LastLoginAt = &lastLogin.Time
	}

	// Update last_login_at
	if _, err := database.DB.Exec("UPDATE cloud_admins SET last_login_at = NOW() WHERE id = $1", admin.ID); err != nil {
		log.Printf("Failed to update last_login_at for admin %s: %v", admin.ID, err)
	}

	// Generate JWT token
	token, err := generateJWT(admin, jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat token: %w", err)
	}

	// Load role permissions
	perms, err := GetRolePermissions(admin.Role)
	if err != nil {
		perms = []string{} // fallback to empty
	}

	// superadmin gets all permissions
	if admin.Role == "superadmin" {
		perms = AllPermissions
	}

	// Load role scope
	scopeType, scopeOutletIDs := GetRoleScopeOutletIDs(admin.Role)
	if admin.Role == "superadmin" {
		scopeType = "all"
		scopeOutletIDs = nil
	}

	// Load redirect_to for the role
	redirectTo := "/"
	if admin.Role == "superadmin" {
		redirectTo = "/"
	} else {
		_ = database.DB.QueryRow(`SELECT COALESCE(redirect_to, '/') FROM roles WHERE name = $1`, admin.Role).Scan(&redirectTo)
	}

	return &models.AdminLoginResponse{
		Token:          token,
		Admin:          admin,
		Permissions:    perms,
		ScopeType:      scopeType,
		ScopeOutletIDs: scopeOutletIDs,
		RedirectTo:     redirectTo,
	}, nil
}

func generateJWT(admin models.CloudAdmin, secret string) (string, error) {
	// Load scope for JWT claims
	scopeType, scopeOutletIDs := GetRoleScopeOutletIDs(admin.Role)
	if admin.Role == "superadmin" {
		scopeType = "all"
		scopeOutletIDs = nil
	}

	claims := jwt.MapClaims{
		"sub":      admin.ID,
		"username": admin.Username,
		"name":     admin.Name,
		"role":     admin.Role,
		"scope":    scopeType,
		"iat":      time.Now().UTC().Unix(),
		"exp":      time.Now().UTC().Add(24 * time.Hour).Unix(),
	}
	if scopeType == "specific" && len(scopeOutletIDs) > 0 {
		claims["outlet_ids"] = scopeOutletIDs
	}
	if scopeType == "specific" {
		wuIDs := GetRoleScopeWorkUnitIDs(admin.Role)
		if len(wuIDs) > 0 {
			claims["work_unit_ids"] = wuIDs
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateJWT(tokenString, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func GetAdmins() ([]models.CloudAdmin, error) {
	rows, err := database.DB.Query(
		`SELECT id, username, name, role, is_active, last_login_at, created_at, updated_at
		FROM cloud_admins ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	admins := make([]models.CloudAdmin, 0)
	for rows.Next() {
		var a models.CloudAdmin
		var lastLogin sql.NullTime
		if err := rows.Scan(&a.ID, &a.Username, &a.Name, &a.Role,
			&a.IsActive, &lastLogin, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.ID = strings.TrimSpace(a.ID)
		if lastLogin.Valid {
			a.LastLoginAt = &lastLogin.Time
		}
		admins = append(admins, a)
	}
	return admins, rows.Err()
}

// GuardTargetAdmin menolak modifikasi akun ber-role superadmin oleh non-superadmin.
// Tanpa ini, pemegang users.update bisa mereset password / menonaktifkan / menghapus
// akun superadmin dan mengambil alih sistem.
func GuardTargetAdmin(adminID, callerRole string) error {
	var targetRole string
	err := database.DB.QueryRow(`SELECT role FROM cloud_admins WHERE id = $1`, adminID).Scan(&targetRole)
	if err == sql.ErrNoRows {
		return fmt.Errorf("admin tidak ditemukan")
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(targetRole) == "superadmin" && callerRole != "superadmin" {
		return fmt.Errorf("akun superadmin hanya dapat dikelola oleh superadmin")
	}
	return nil
}

func CreateAdmin(req models.CreateAdminRequest) (*models.CloudAdmin, error) {
	if req.Username == "" || req.Password == "" || req.Name == "" {
		return nil, fmt.Errorf("username, password, dan name wajib diisi")
	}
	if len(req.Password) < 6 {
		return nil, fmt.Errorf("password minimal 6 karakter")
	}
	if req.Role == "" {
		req.Role = "admin"
	}
	// Role wajib terdaftar di tabel roles. "superadmin" bukan baris tabel (string
	// bypass di middleware), jadi validasi ini sekaligus menutup pembuatan akun
	// superadmin lewat API oleh pemegang users.create.
	var roleExists bool
	if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, req.Role).Scan(&roleExists); err != nil {
		return nil, err
	}
	if !roleExists {
		return nil, fmt.Errorf("role '%s' tidak valid", req.Role)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	id := NewULID()
	admin := &models.CloudAdmin{ID: id, Username: req.Username, Name: req.Name, Role: req.Role, IsActive: true}

	err = database.DB.QueryRow(
		`INSERT INTO cloud_admins (id, username, password_hash, name, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`,
		id, req.Username, string(hash), req.Name, req.Role,
	).Scan(&admin.CreatedAt, &admin.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("username '%s' sudah digunakan", req.Username)
		}
		return nil, err
	}

	return admin, nil
}

func UpdateAdminRole(adminID string, req models.UpdateAdminRoleRequest) (*models.CloudAdmin, error) {
	var exists bool
	err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, req.Role).Scan(&exists)
	if err != nil || !exists {
		return nil, fmt.Errorf("role '%s' tidak valid", req.Role)
	}

	var admin models.CloudAdmin
	var lastLogin sql.NullTime
	err = database.DB.QueryRow(
		`UPDATE cloud_admins SET role = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, username, name, role, is_active, last_login_at, created_at, updated_at`,
		req.Role, adminID,
	).Scan(&admin.ID, &admin.Username, &admin.Name, &admin.Role,
		&admin.IsActive, &lastLogin, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin tidak ditemukan")
		}
		return nil, err
	}
	admin.ID = strings.TrimSpace(admin.ID)
	if lastLogin.Valid {
		admin.LastLoginAt = &lastLogin.Time
	}
	return &admin, nil
}

func GetRolePermissions(role string) ([]string, error) {
	rows, err := database.DB.Query(
		`SELECT permission FROM role_permissions WHERE role = $1 ORDER BY permission`,
		role,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	perms := make([]string, 0)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

func GetAllRolePermissions() (map[string][]string, error) {
	rows, err := database.DB.Query(
		`SELECT role, permission FROM role_permissions ORDER BY role, permission`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var role, perm string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, err
		}
		result[role] = append(result[role], perm)
	}
	return result, rows.Err()
}

func UpdateRolePermissions(role string, permissions []string) error {
	validPerms := make(map[string]bool)
	for _, p := range AllPermissions {
		validPerms[p] = true
	}
	for _, p := range permissions {
		if !validPerms[p] {
			return fmt.Errorf("permission '%s' tidak valid", p)
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM role_permissions WHERE role = $1`, role); err != nil {
		tx.Rollback()
		return err
	}

	for _, p := range permissions {
		if _, err := tx.Exec(
			`INSERT INTO role_permissions (role, permission) VALUES ($1, $2)`,
			role, p,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func ListRoles() ([]models.Role, error) {
	rows, err := database.DB.Query(
		`SELECT name, description, is_system, COALESCE(scope_type, 'all'), COALESCE(redirect_to, '/'), created_at FROM roles ORDER BY is_system DESC, name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var r models.Role
		if err := rows.Scan(&r.Name, &r.Description, &r.IsSystem, &r.ScopeType, &r.RedirectTo, &r.CreatedAt); err != nil {
			return nil, err
		}
		roles = append(roles, r)
	}
	return roles, rows.Err()
}

func CreateRole(req models.CreateRoleRequest) (*models.Role, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("nama role wajib diisi")
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if req.Name == "superadmin" {
		return nil, fmt.Errorf("nama role tidak diizinkan")
	}

	if req.ScopeType == "" {
		req.ScopeType = "all"
	}
	if req.ScopeType != "all" && req.ScopeType != "specific" {
		return nil, fmt.Errorf("scope_type harus 'all' atau 'specific'")
	}

	if req.RedirectTo == "" {
		req.RedirectTo = "/"
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}

	var role models.Role
	err = tx.QueryRow(
		`INSERT INTO roles (name, description, is_system, scope_type, redirect_to) VALUES ($1, $2, false, $3, $4) RETURNING name, description, is_system, scope_type, redirect_to, created_at`,
		req.Name, req.Description, req.ScopeType, req.RedirectTo,
	).Scan(&role.Name, &role.Description, &role.IsSystem, &role.ScopeType, &role.RedirectTo, &role.CreatedAt)
	if err != nil {
		tx.Rollback()
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			return nil, fmt.Errorf("role '%s' sudah ada", req.Name)
		}
		return nil, err
	}

	for _, p := range req.Permissions {
		validPerm := false
		for _, ap := range AllPermissions {
			if p == ap {
				validPerm = true
				break
			}
		}
		if !validPerm {
			tx.Rollback()
			return nil, fmt.Errorf("permission '%s' tidak valid", p)
		}
		if _, err := tx.Exec(
			`INSERT INTO role_permissions (role, permission) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			req.Name, p,
		); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Save work unit scope if specific
	if req.ScopeType == "specific" {
		for _, wuID := range req.WorkUnitIDs {
			if _, err := tx.Exec(
				`INSERT INTO role_work_unit_scope (role, work_unit_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				req.Name, wuID,
			); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	return &role, tx.Commit()
}

// GetRoleScopeOutletIDs returns the scope type and the outlet IDs a role can access.
// For "all" scope, returns ("all", nil). For "specific", returns ("specific", []outletIDs).
func GetRoleScopeOutletIDs(role string) (string, []string) {
	var scopeType string
	err := database.DB.QueryRow(
		`SELECT COALESCE(scope_type, 'all') FROM roles WHERE name = $1`, role,
	).Scan(&scopeType)
	if err != nil || scopeType != "specific" {
		return "all", nil
	}

	rows, err := database.DB.Query(
		`SELECT DISTINCT w.outlet_id FROM role_work_unit_scope s
		 JOIN work_units w ON w.id = s.work_unit_id
		 WHERE s.role = $1 AND w.outlet_id IS NOT NULL`, role,
	)
	if err != nil {
		return "all", nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	if len(ids) == 0 {
		return "specific", []string{}
	}
	return "specific", ids
}

// GetRoleScopeWorkUnitIDs returns the work unit IDs assigned to a role scope.
func GetRoleScopeWorkUnitIDs(role string) []string {
	rows, err := database.DB.Query(
		`SELECT work_unit_id FROM role_work_unit_scope WHERE role = $1`, role,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, strings.TrimSpace(id))
		}
	}
	return ids
}

// GetAllRoleScopes returns scope info for all roles.
func GetAllRoleScopes() (map[string]models.RoleScopeInfo, error) {
	result := make(map[string]models.RoleScopeInfo)

	// Get scope types
	rows, err := database.DB.Query(`SELECT name, COALESCE(scope_type, 'all') FROM roles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, scopeType string
		if err := rows.Scan(&name, &scopeType); err == nil {
			result[name] = models.RoleScopeInfo{ScopeType: scopeType}
		}
	}

	// Get work unit IDs per role
	wuRows, err := database.DB.Query(`SELECT role, work_unit_id FROM role_work_unit_scope ORDER BY role`)
	if err != nil {
		return result, nil // return what we have
	}
	defer wuRows.Close()
	for wuRows.Next() {
		var role, wuID string
		if err := wuRows.Scan(&role, &wuID); err == nil {
			info := result[role]
			info.ScopeType = result[role].ScopeType
			info.WorkUnitIDs = append(info.WorkUnitIDs, strings.TrimSpace(wuID))
			result[role] = info
		}
	}

	return result, nil
}

// UpdateRoleScope updates the scope configuration for a role.
func UpdateRoleScope(role string, req models.UpdateRoleScopeRequest) error {
	if req.ScopeType != "all" && req.ScopeType != "specific" {
		return fmt.Errorf("scope_type harus 'all' atau 'specific'")
	}

	// Check role isn't superadmin
	if role == "superadmin" {
		return fmt.Errorf("scope superadmin tidak dapat diubah")
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`UPDATE roles SET scope_type = $1 WHERE name = $2`, req.ScopeType, role); err != nil {
		tx.Rollback()
		return err
	}

	// Clear existing scope
	if _, err := tx.Exec(`DELETE FROM role_work_unit_scope WHERE role = $1`, role); err != nil {
		tx.Rollback()
		return err
	}

	// Insert new scope entries if specific
	if req.ScopeType == "specific" {
		for _, wuID := range req.WorkUnitIDs {
			if _, err := tx.Exec(
				`INSERT INTO role_work_unit_scope (role, work_unit_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
				role, wuID,
			); err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return tx.Commit()
}

// GetOutletIDsForScope returns the pq.Array-compatible value for SQL scope filtering.
// Returns nil (for "all" scope — no filtering needed) or the array of outlet IDs.
func GetOutletIDsForScope(scopeType string, outletIDs []string) interface{} {
	if scopeType != "specific" || len(outletIDs) == 0 {
		return nil
	}
	return pq.Array(outletIDs)
}

func DeleteRole(name string) error {
	var isSystem bool
	err := database.DB.QueryRow(`SELECT is_system FROM roles WHERE name = $1`, name).Scan(&isSystem)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("role tidak ditemukan")
		}
		return err
	}
	if isSystem {
		return fmt.Errorf("role bawaan sistem tidak dapat dihapus")
	}

	var count int
	err = database.DB.QueryRow(`SELECT COUNT(*) FROM cloud_admins WHERE role = $1`, name).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("tidak dapat menghapus role yang masih digunakan oleh %d pengguna", count)
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM role_permissions WHERE role = $1`, name); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM role_work_unit_scope WHERE role = $1`, name); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM roles WHERE name = $1`, name); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpdateRole updates a role's name, description, and redirect_to.
func UpdateRole(currentName string, req models.UpdateRoleRequest) (*models.Role, error) {
	var isSystem bool
	err := database.DB.QueryRow(`SELECT is_system FROM roles WHERE name = $1`, currentName).Scan(&isSystem)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("role tidak ditemukan")
		}
		return nil, err
	}

	redirectTo := strings.TrimSpace(req.RedirectTo)
	if redirectTo == "" {
		redirectTo = "/"
	}

	// System roles: only allow description and redirect_to changes, not name changes
	if isSystem {
		newName := strings.TrimSpace(req.Name)
		if newName != "" && newName != currentName {
			return nil, fmt.Errorf("nama role bawaan sistem tidak dapat diubah")
		}
		_, err = database.DB.Exec(`UPDATE roles SET description = $1, redirect_to = $2 WHERE name = $3`,
			strings.TrimSpace(req.Description), redirectTo, currentName)
		if err != nil {
			return nil, err
		}
		var role models.Role
		err = database.DB.QueryRow(`SELECT name, description, is_system, scope_type, redirect_to, created_at FROM roles WHERE name = $1`, currentName).
			Scan(&role.Name, &role.Description, &role.IsSystem, &role.ScopeType, &role.RedirectTo, &role.CreatedAt)
		if err != nil {
			return nil, err
		}
		return &role, nil
	}

	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		return nil, fmt.Errorf("nama role wajib diisi")
	}
	// "superadmin" bukan baris tabel roles, jadi cek duplikat di bawah tidak
	// menangkapnya — tanpa guard ini rename role custom → superadmin mem-promote
	// semua penggunanya jadi full bypass.
	if strings.ToLower(newName) == "superadmin" {
		return nil, fmt.Errorf("nama role tidak diizinkan")
	}

	// If renaming, check for conflicts
	if newName != currentName {
		var exists bool
		err = database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, newName).Scan(&exists)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, fmt.Errorf("role '%s' sudah ada", newName)
		}
	}

	tx, err := database.DB.Begin()
	if err != nil {
		return nil, err
	}

	// Update the role itself
	_, err = tx.Exec(`UPDATE roles SET name = $1, description = $2, redirect_to = $3 WHERE name = $4`,
		newName, strings.TrimSpace(req.Description), redirectTo, currentName)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// If name changed, cascade to related tables
	if newName != currentName {
		if _, err := tx.Exec(`UPDATE role_permissions SET role = $1 WHERE role = $2`, newName, currentName); err != nil {
			tx.Rollback()
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE role_work_unit_scope SET role = $1 WHERE role = $2`, newName, currentName); err != nil {
			tx.Rollback()
			return nil, err
		}
		if _, err := tx.Exec(`UPDATE cloud_admins SET role = $1 WHERE role = $2`, newName, currentName); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var role models.Role
	err = database.DB.QueryRow(`SELECT name, description, is_system, scope_type, redirect_to, created_at FROM roles WHERE name = $1`, newName).
		Scan(&role.Name, &role.Description, &role.IsSystem, &role.ScopeType, &role.RedirectTo, &role.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// ChangePassword verifies the current password and updates to the new one.
func ChangePassword(adminID string, req models.ChangePasswordRequest) error {
	var hash string
	err := database.DB.QueryRow(`SELECT password_hash FROM cloud_admins WHERE id = $1`, adminID).Scan(&hash)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user tidak ditemukan")
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.CurrentPassword)); err != nil {
		return fmt.Errorf("password lama salah")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = database.DB.Exec(`UPDATE cloud_admins SET password_hash = $1, updated_at = NOW() WHERE id = $2`, string(newHash), adminID)
	return err
}

// UpdateProfile updates the admin's display name.
func UpdateProfile(adminID string, req models.UpdateProfileRequest) (*models.CloudAdmin, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}

	_, err := database.DB.Exec(`UPDATE cloud_admins SET name = $1, updated_at = NOW() WHERE id = $2`, name, adminID)
	if err != nil {
		return nil, err
	}

	var admin models.CloudAdmin
	err = database.DB.QueryRow(
		`SELECT id, username, name, role, is_active, last_login_at, created_at, updated_at FROM cloud_admins WHERE id = $1`, adminID,
	).Scan(&admin.ID, &admin.Username, &admin.Name, &admin.Role, &admin.IsActive, &admin.LastLoginAt, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// UpdateAdmin updates an admin's name and role (by another admin).
func UpdateAdmin(adminID string, req models.UpdateAdminRequest) (*models.CloudAdmin, error) {
	name := strings.TrimSpace(req.Name)
	role := strings.TrimSpace(req.Role)
	if name == "" {
		return nil, fmt.Errorf("nama wajib diisi")
	}
	if role == "" {
		return nil, fmt.Errorf("role wajib diisi")
	}

	var exists bool
	if err := database.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE name = $1)`, role).Scan(&exists); err != nil || !exists {
		return nil, fmt.Errorf("role '%s' tidak valid", role)
	}

	var admin models.CloudAdmin
	var lastLogin sql.NullTime
	err := database.DB.QueryRow(
		`UPDATE cloud_admins SET name = $1, role = $2, updated_at = NOW()
		WHERE id = $3
		RETURNING id, username, name, role, is_active, last_login_at, created_at, updated_at`,
		name, role, adminID,
	).Scan(&admin.ID, &admin.Username, &admin.Name, &admin.Role,
		&admin.IsActive, &lastLogin, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin tidak ditemukan")
		}
		return nil, err
	}
	admin.ID = strings.TrimSpace(admin.ID)
	if lastLogin.Valid {
		admin.LastLoginAt = &lastLogin.Time
	}
	return &admin, nil
}

// ResetAdminPassword resets an admin's password (by another admin, no old password check).
func ResetAdminPassword(adminID string, req models.ResetAdminPasswordRequest) error {
	pw := strings.TrimSpace(req.NewPassword)
	if len(pw) < 6 {
		return fmt.Errorf("password minimal 6 karakter")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	res, err := database.DB.Exec(`UPDATE cloud_admins SET password_hash = $1, updated_at = NOW() WHERE id = $2`, string(hash), adminID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("admin tidak ditemukan")
	}
	return nil
}

// ToggleAdminActive toggles the is_active flag of an admin.
func ToggleAdminActive(adminID string) (*models.CloudAdmin, error) {
	var admin models.CloudAdmin
	var lastLogin sql.NullTime
	err := database.DB.QueryRow(
		`UPDATE cloud_admins SET is_active = NOT is_active, updated_at = NOW()
		WHERE id = $1
		RETURNING id, username, name, role, is_active, last_login_at, created_at, updated_at`,
		adminID,
	).Scan(&admin.ID, &admin.Username, &admin.Name, &admin.Role,
		&admin.IsActive, &lastLogin, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("admin tidak ditemukan")
		}
		return nil, err
	}
	admin.ID = strings.TrimSpace(admin.ID)
	if lastLogin.Valid {
		admin.LastLoginAt = &lastLogin.Time
	}
	return &admin, nil
}

// DeleteAdmin deletes an admin user.
func DeleteAdmin(adminID string) error {
	res, err := database.DB.Exec(`DELETE FROM cloud_admins WHERE id = $1`, adminID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("admin tidak ditemukan")
	}
	return nil
}

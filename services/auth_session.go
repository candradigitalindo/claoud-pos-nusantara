package services

// Sesi admin — pencabutan JWT (T-55) dan scope gagal-tertutup (T-56).
//
// JWT admin berumur 24 jam. Dulu role, scope, dan status aktif "dibakar" ke
// token tanpa jalan pencabutan: akun yang dinonaktifkan atau dipindah role
// tetap bisa bertindak sampai token kedaluwarsa, dan galat DB sesaat ketika
// login memberi scope "all" selama 24 jam. Sekarang setiap permintaan ber-JWT
// (AdminAuth dan stream SSE) dicocokkan dengan keadaan akun di DB:
//
//   - akun harus masih ada dan aktif;
//   - klaim "ver" harus sama dengan cloud_admins.token_version — token lama
//     tanpa klaim itu dianggap versi 0, jadi sesi yang ada tetap berjalan
//     setelah deploy selama versi akunnya belum dinaikkan;
//   - role di token harus sama dengan role akun sekarang;
//   - scope (outlet & unit kerja) dibaca dari role di DB, bukan dari token.
//
// Keadaan akun dan scope role di-cache AdminSessionCacheTTL per proses agar
// tidak ada query di setiap permintaan. Perubahan lewat layanan di paket ini
// membuang cache seketika; perubahan di luar itu (SQL langsung, instans lain)
// berlaku paling lambat setelah TTL. Galat DB tidak pernah di-cache dan selalu
// berarti TOLAK.

import (
	"cloud-pos/database"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// adminTokenType: nilai klaim "typ" JWT sesi admin. Token lain yang
	// ditandatangani secret yang sama — cookie relay CCTV ("cctv") — bukan
	// sesi admin dan ditolak. Token admin lama tidak punya klaim typ.
	adminTokenType = "admin"

	// AdminSessionCacheTTL: lama keadaan akun & scope role disimpan di memori.
	AdminSessionCacheTTL = 30 * time.Second

	// adminSessionCacheMax: di atas jumlah entri ini, entri basi disapu.
	adminSessionCacheMax = 1024
)

// Penolakan sesi admin. Middleware memetakan ErrAuthUnavailable ke 503
// (gagal-tertutup tanpa memaksa logout karena galat sementara) dan sisanya ke
// 401 (UI membersihkan sesi dan kembali ke halaman login).
var (
	ErrAuthTokenInvalid    = errors.New("token tidak valid atau sudah kedaluwarsa")
	ErrAuthSessionRevoked  = errors.New("sesi sudah tidak berlaku, silakan login ulang")
	ErrAuthAccountInactive = errors.New("akun tidak aktif")
	ErrAuthUnavailable     = errors.New("sesi tidak dapat diperiksa saat ini, coba lagi sebentar lagi")
)

// AdminSession: identitas dan cakupan admin yang sudah dicocokkan dengan DB.
type AdminSession struct {
	ID          string
	Username    string
	Role        string
	ScopeType   string   // "all" | "specific"
	OutletIDs   []string // specific: outlet yang boleh diakses (kosong = tidak ada)
	WorkUnitIDs []string // specific: unit kerja yang boleh diakses (kosong = tidak ada)
}

// RoleScope: cakupan satu role. Type selalu "all" atau "specific"; untuk
// specific kedua daftar tidak pernah nil (kosong = tidak ada akses).
type RoleScope struct {
	Type        string
	OutletIDs   []string
	WorkUnitIDs []string
}

// closedRoleScope: scope yang tidak membuka outlet atau unit kerja apa pun.
func closedRoleScope() RoleScope {
	return RoleScope{Type: "specific", OutletIDs: []string{}, WorkUnitIDs: []string{}}
}

// normalized memaksa bentuk baku: selain "all" berarti specific dengan daftar
// tidak nil. Nilai tak dikenal tidak pernah menjadi "semua".
func (s RoleScope) normalized() RoleScope {
	if s.Type == "all" {
		return RoleScope{Type: "all"}
	}
	out := RoleScope{Type: "specific", OutletIDs: []string{}, WorkUnitIDs: []string{}}
	out.OutletIDs = append(out.OutletIDs, s.OutletIDs...)
	out.WorkUnitIDs = append(out.WorkUnitIDs, s.WorkUnitIDs...)
	return out
}

// ResolveRoleScope membaca scope role dari DB secara gagal-tertutup (T-56):
//   - superadmin → all;
//   - role tidak terdaftar di tabel roles (sql.ErrNoRows) → specific tanpa
//     outlet/unit (tidak melihat data ber-outlet), bukan all;
//   - scope_type tak dikenal → specific;
//   - galat DB lain → error; pemanggil WAJIB menolak login/permintaan.
func ResolveRoleScope(role string) (RoleScope, error) {
	role = strings.TrimSpace(role)
	if role == "superadmin" {
		return RoleScope{Type: "all"}, nil
	}
	if database.DB == nil {
		return RoleScope{}, errors.New("database belum terhubung")
	}

	var scopeType string
	err := database.DB.QueryRow(
		`SELECT COALESCE(scope_type, 'all') FROM roles WHERE name = $1`, role,
	).Scan(&scopeType)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("[Auth] role %q tidak terdaftar di tabel roles — scope ditutup (tanpa outlet/unit)", role)
		return closedRoleScope(), nil
	}
	if err != nil {
		return RoleScope{}, fmt.Errorf("baca scope role %q: %w", role, err)
	}
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case "all":
		return RoleScope{Type: "all"}, nil
	case "specific":
	default:
		log.Printf("[Auth] scope_type %q pada role %q tidak dikenal — diperlakukan specific", scopeType, role)
	}

	outletIDs, err := scopeQueryIDs(
		`SELECT DISTINCT w.outlet_id FROM role_work_unit_scope s
		 JOIN work_units w ON w.id = s.work_unit_id
		 WHERE s.role = $1 AND w.outlet_id IS NOT NULL
		 ORDER BY w.outlet_id`, role)
	if err != nil {
		return RoleScope{}, fmt.Errorf("baca outlet scope role %q: %w", role, err)
	}
	wuIDs, err := scopeQueryIDs(
		`SELECT work_unit_id FROM role_work_unit_scope WHERE role = $1 ORDER BY work_unit_id`, role)
	if err != nil {
		return RoleScope{}, fmt.Errorf("baca unit kerja scope role %q: %w", role, err)
	}
	return RoleScope{Type: "specific", OutletIDs: outletIDs, WorkUnitIDs: wuIDs}, nil
}

// scopeQueryIDs menjalankan query satu kolom id. Galat scan/iterasi
// dikembalikan — tidak dilewati diam-diam.
func scopeQueryIDs(q string, args ...interface{}) ([]string, error) {
	rows, err := database.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, strings.TrimSpace(id))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// adminAccountState: kolom cloud_admins yang menentukan sah tidaknya sesi.
type adminAccountState struct {
	Found    bool // false = akun sudah dihapus
	Active   bool
	Version  int
	Role     string
	Username string
}

func loadAdminAccountStateDB(adminID string) (adminAccountState, error) {
	if database.DB == nil {
		return adminAccountState{}, errors.New("database belum terhubung")
	}
	var st adminAccountState
	err := database.DB.QueryRow(
		`SELECT username, role, COALESCE(is_active, false), token_version
		 FROM cloud_admins WHERE id = $1`, adminID,
	).Scan(&st.Username, &st.Role, &st.Active, &st.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return adminAccountState{}, nil
	}
	if err != nil {
		return adminAccountState{}, err
	}
	st.Found = true
	st.Username = strings.TrimSpace(st.Username)
	st.Role = strings.TrimSpace(st.Role)
	return st, nil
}

// Titik sambung untuk tes tanpa DB: pemuat keadaan akun, pemuat scope role,
// dan jam cache.
var (
	adminStateLoader = loadAdminAccountStateDB
	roleScopeLoader  = ResolveRoleScope
	sessionClock     = time.Now
)

type cachedAdminState struct {
	st adminAccountState
	at time.Time
}

type cachedRoleScope struct {
	sc RoleScope
	at time.Time
}

// adminSessionCache: cache keadaan akun (per id) dan scope (per role).
// gen naik pada setiap invalidasi; hasil muat yang dimulai sebelum invalidasi
// tidak disimpan, sehingga penonaktifan yang terjadi saat query berjalan tidak
// tertimpa keadaan lama.
type adminSessionCache struct {
	mu       sync.Mutex
	gen      uint64
	accounts map[string]cachedAdminState
	scopes   map[string]cachedRoleScope
}

var adminSessions = &adminSessionCache{
	accounts: map[string]cachedAdminState{},
	scopes:   map[string]cachedRoleScope{},
}

func sessionCacheFresh(now, at time.Time) bool {
	age := now.Sub(at)
	return age >= 0 && age < AdminSessionCacheTTL
}

func (c *adminSessionCache) account(adminID string) (adminAccountState, error) {
	now := sessionClock()
	c.mu.Lock()
	if e, ok := c.accounts[adminID]; ok && sessionCacheFresh(now, e.at) {
		c.mu.Unlock()
		return e.st, nil
	}
	gen := c.gen
	c.mu.Unlock()

	st, err := adminStateLoader(adminID)
	if err != nil {
		return adminAccountState{}, err // tidak di-cache: permintaan berikutnya mencoba lagi
	}

	c.mu.Lock()
	if c.gen == gen {
		if len(c.accounts) >= adminSessionCacheMax {
			for k, e := range c.accounts {
				if !sessionCacheFresh(now, e.at) {
					delete(c.accounts, k)
				}
			}
		}
		c.accounts[adminID] = cachedAdminState{st: st, at: now}
	}
	c.mu.Unlock()
	return st, nil
}

// roleScope mengembalikan salinan — pemanggil boleh mengubah slice-nya tanpa
// menyentuh isi cache.
func (c *adminSessionCache) roleScope(role string) (RoleScope, error) {
	now := sessionClock()
	c.mu.Lock()
	if e, ok := c.scopes[role]; ok && sessionCacheFresh(now, e.at) {
		c.mu.Unlock()
		return e.sc.normalized(), nil
	}
	gen := c.gen
	c.mu.Unlock()

	sc, err := roleScopeLoader(role)
	if err != nil {
		return RoleScope{}, err
	}
	sc = sc.normalized()

	c.mu.Lock()
	if c.gen == gen {
		if len(c.scopes) >= adminSessionCacheMax {
			for k, e := range c.scopes {
				if !sessionCacheFresh(now, e.at) {
					delete(c.scopes, k)
				}
			}
		}
		c.scopes[role] = cachedRoleScope{sc: sc, at: now}
	}
	c.mu.Unlock()
	return sc.normalized(), nil
}

// InvalidateAdminSession membuang keadaan satu akun dari cache, sehingga
// perubahan (nonaktif, versi token, role, hapus) berlaku pada permintaan
// berikutnya di proses ini.
func InvalidateAdminSession(adminID string) {
	adminSessions.mu.Lock()
	delete(adminSessions.accounts, strings.TrimSpace(adminID))
	adminSessions.gen++
	adminSessions.mu.Unlock()
}

// InvalidateAllAdminSessions membuang seluruh cache akun & scope role —
// untuk perubahan yang menyangkut satu role penuh (scope, rename).
func InvalidateAllAdminSessions() {
	adminSessions.mu.Lock()
	adminSessions.accounts = map[string]cachedAdminState{}
	adminSessions.scopes = map[string]cachedRoleScope{}
	adminSessions.gen++
	adminSessions.mu.Unlock()
}

// adminTokenClaims: bagian klaim JWT yang dicocokkan dengan DB.
type adminTokenClaims struct {
	sub     string
	role    string
	version int
}

func parseAdminTokenClaims(claims jwt.MapClaims) (adminTokenClaims, error) {
	if raw, ok := claims["typ"]; ok {
		if typ, _ := raw.(string); typ != adminTokenType {
			return adminTokenClaims{}, ErrAuthTokenInvalid
		}
	}
	sub, _ := claims["sub"].(string)
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return adminTokenClaims{}, ErrAuthTokenInvalid
	}
	ver, ok := claimTokenVersion(claims)
	if !ok {
		return adminTokenClaims{}, ErrAuthTokenInvalid
	}
	role, _ := claims["role"].(string)
	return adminTokenClaims{sub: sub, role: strings.TrimSpace(role), version: ver}, nil
}

// claimTokenVersion membaca klaim "ver". Token lama tanpa klaim = versi 0;
// klaim yang ada tetapi bukan bilangan bulat ≥ 0 = token tidak sah.
func claimTokenVersion(claims jwt.MapClaims) (int, bool) {
	raw, ok := claims["ver"]
	if !ok {
		return 0, true
	}
	switch v := raw.(type) {
	case float64:
		if v < 0 || v > math.MaxInt32 || v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case json.Number:
		n, err := v.Int64()
		if err != nil || n < 0 || n > math.MaxInt32 {
			return 0, false
		}
		return int(n), true
	}
	return 0, false
}

// matchAdminAccount: keputusan sah tidaknya token terhadap keadaan akun.
func matchAdminAccount(st adminAccountState, tc adminTokenClaims) error {
	switch {
	case !st.Found:
		return ErrAuthSessionRevoked // akun sudah dihapus
	case !st.Active:
		return ErrAuthAccountInactive
	case st.Version != tc.version:
		return ErrAuthSessionRevoked // password diganti/direset, role/scope berubah, dst.
	case st.Role != tc.role:
		return ErrAuthSessionRevoked // role berubah di luar jalur yang menaikkan versi
	}
	return nil
}

// AuthenticateAdminJWT memverifikasi JWT admin (tanda tangan, kedaluwarsa,
// jenis) lalu mencocokkannya dengan keadaan akun dan scope role di DB.
// Satu-satunya pintu validasi sesi ber-JWT — dipakai AdminAuth dan stream SSE.
func AuthenticateAdminJWT(tokenString, secret string) (*AdminSession, error) {
	claims, err := ValidateJWT(tokenString, secret)
	if err != nil {
		return nil, ErrAuthTokenInvalid
	}
	tc, err := parseAdminTokenClaims(claims)
	if err != nil {
		return nil, err
	}
	return authenticateAdminClaims(tc)
}

// authenticateAdminClaims dipakai sesi admin dan cookie relay CCTV setelah
// masing-masing memverifikasi tanda tangan dan jenis tokennya sendiri.
func authenticateAdminClaims(tc adminTokenClaims) (*AdminSession, error) {
	st, err := adminSessions.account(tc.sub)
	if err != nil {
		log.Printf("[Auth] periksa sesi akun %s gagal: %v", tc.sub, err)
		return nil, ErrAuthUnavailable
	}
	if err := matchAdminAccount(st, tc); err != nil {
		return nil, err
	}

	sc, err := adminSessions.roleScope(st.Role)
	if err != nil {
		log.Printf("[Auth] baca scope role %q gagal: %v", st.Role, err)
		return nil, ErrAuthUnavailable
	}
	return &AdminSession{
		ID:          tc.sub,
		Username:    st.Username,
		Role:        st.Role,
		ScopeType:   sc.Type,
		OutletIDs:   sc.OutletIDs,
		WorkUnitIDs: sc.WorkUnitIDs,
	}, nil
}

// authUnavailable: galat DB ketika login — detail dicatat di log, pemanggil
// menerima ErrAuthUnavailable (HTTP 503) tanpa isi galat mentah.
func authUnavailable(step string, err error) error {
	log.Printf("[Auth] login: %s gagal: %v", step, err)
	return fmt.Errorf("%w (%s)", ErrAuthUnavailable, step)
}

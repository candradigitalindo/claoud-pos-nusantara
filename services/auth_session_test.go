package services

// Uji sesi admin: pencabutan JWT lewat versi token (T-55) dan scope
// gagal-tertutup (T-56).
//
// Bagian pertama tanpa database — pemuat keadaan akun & scope diganti tiruan
// untuk menguji logika klaim dan cache. Bagian kedua butuh Postgres uji
// (DB_NAME berakhiran _test); tanpa itu di-skip.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud-pos/config"
	"cloud-pos/database"
	"cloud-pos/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
)

const sesiRahasiaUji = "rahasia-jwt-uji-sesi-0123456789abcdef"

func sesiTandaTangan(t *testing.T, claims jwt.MapClaims, secret string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("tanda tangan token: %v", err)
	}
	return tok
}

// sesiKlaimLama: bentuk token sebelum perbaikan — tanpa "typ" dan "ver".
func sesiKlaimLama(sub, role string) jwt.MapClaims {
	now := time.Now().UTC()
	return jwt.MapClaims{
		"sub": sub, "username": "uji", "name": "Uji", "role": role, "scope": "all",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
}

// sesiKlaimBaru: bentuk token sesudah perbaikan.
func sesiKlaimBaru(sub, role string, ver int) jwt.MapClaims {
	c := sesiKlaimLama(sub, role)
	c["typ"] = adminTokenType
	c["ver"] = ver
	return c
}

// sesiPasangTiruan mengganti pemuat DB dengan tiruan selama satu tes dan
// mengosongkan cache sebelum & sesudahnya.
func sesiPasangTiruan(t *testing.T, akun func(string) (adminAccountState, error), scope func(string) (RoleScope, error)) {
	t.Helper()
	asliAkun, asliScope, asliJam := adminStateLoader, roleScopeLoader, sessionClock
	adminStateLoader, roleScopeLoader = akun, scope
	InvalidateAllAdminSessions()
	t.Cleanup(func() {
		adminStateLoader, roleScopeLoader, sessionClock = asliAkun, asliScope, asliJam
		InvalidateAllAdminSessions()
	})
}

func TestClaimTokenVersion(t *testing.T) {
	kasus := []struct {
		nama  string
		klaim jwt.MapClaims
		ver   int
		ok    bool
	}{
		{"token lama tanpa ver = versi 0", jwt.MapClaims{}, 0, true},
		{"ver 0", jwt.MapClaims{"ver": float64(0)}, 0, true},
		{"ver 7", jwt.MapClaims{"ver": float64(7)}, 7, true},
		{"json.Number", jwt.MapClaims{"ver": json.Number("3")}, 3, true},
		{"pecahan ditolak", jwt.MapClaims{"ver": 1.5}, 0, false},
		{"negatif ditolak", jwt.MapClaims{"ver": float64(-1)}, 0, false},
		{"string ditolak", jwt.MapClaims{"ver": "1"}, 0, false},
		{"null ditolak", jwt.MapClaims{"ver": nil}, 0, false},
	}
	for _, k := range kasus {
		ver, ok := claimTokenVersion(k.klaim)
		if ok != k.ok || (ok && ver != k.ver) {
			t.Errorf("%s: dapat (%d, %v), mau (%d, %v)", k.nama, ver, ok, k.ver, k.ok)
		}
	}
}

// Keputusan terima/tolak token terhadap keadaan akun, tanpa DB.
func TestAuthenticateAdminJWTTanpaDB(t *testing.T) {
	akun := map[string]adminAccountState{
		"A0":  {Found: true, Active: true, Version: 0, Role: "manager", Username: "ani"},
		"A2":  {Found: true, Active: true, Version: 2, Role: "manager", Username: "budi"},
		"OFF": {Found: true, Active: false, Version: 1, Role: "manager", Username: "cici"},
		"RS":  {Found: true, Active: true, Version: 0, Role: "scope_rusak", Username: "dodi"},
	}
	var muatAkun int
	sesiPasangTiruan(t,
		func(id string) (adminAccountState, error) {
			muatAkun++
			if id == "DBERR" {
				return adminAccountState{}, errors.New("koneksi DB putus")
			}
			return akun[id], nil // id tak dikenal → Found=false (akun dihapus)
		},
		func(role string) (RoleScope, error) {
			if role == "scope_rusak" {
				return RoleScope{}, errors.New("galat DB uji saat baca scope")
			}
			return RoleScope{Type: "specific", OutletIDs: []string{"O1"}, WorkUnitIDs: []string{"W1"}}, nil
		})

	now := time.Now().UTC()
	kadaluarsa := sesiKlaimBaru("A2", "manager", 2)
	kadaluarsa["exp"] = now.Add(-time.Minute).Unix()
	tanpaExp := sesiKlaimBaru("A2", "manager", 2)
	delete(tanpaExp, "exp")
	cctv := jwt.MapClaims{"typ": "cctv", "sub": "A0", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix()}
	tanpaSub := sesiKlaimBaru("", "manager", 0)
	verString := sesiKlaimBaru("A2", "manager", 2)
	verString["ver"] = "2"

	kasus := []struct {
		nama     string
		token    string
		mau      error // nil = diterima
		bacaAkun bool  // apakah boleh sampai ke pemuat akun
	}{
		{"token lama tanpa ver diterima bila versi akun 0", sesiTandaTangan(t, sesiKlaimLama("A0", "manager"), sesiRahasiaUji), nil, true},
		{"token lama tanpa ver ditolak bila versi akun sudah naik", sesiTandaTangan(t, sesiKlaimLama("A2", "manager"), sesiRahasiaUji), ErrAuthSessionRevoked, true},
		{"versi sama diterima", sesiTandaTangan(t, sesiKlaimBaru("A2", "manager", 2), sesiRahasiaUji), nil, true},
		{"versi tertinggal ditolak", sesiTandaTangan(t, sesiKlaimBaru("A2", "manager", 1), sesiRahasiaUji), ErrAuthSessionRevoked, true},
		{"versi melompat ditolak", sesiTandaTangan(t, sesiKlaimBaru("A2", "manager", 3), sesiRahasiaUji), ErrAuthSessionRevoked, true},
		{"akun nonaktif ditolak", sesiTandaTangan(t, sesiKlaimBaru("OFF", "manager", 1), sesiRahasiaUji), ErrAuthAccountInactive, true},
		{"akun terhapus ditolak", sesiTandaTangan(t, sesiKlaimBaru("HILANG", "manager", 0), sesiRahasiaUji), ErrAuthSessionRevoked, true},
		{"role di token beda dengan DB ditolak", sesiTandaTangan(t, sesiKlaimBaru("A0", "admin", 0), sesiRahasiaUji), ErrAuthSessionRevoked, true},
		{"galat DB baca akun = tolak (503)", sesiTandaTangan(t, sesiKlaimBaru("DBERR", "manager", 0), sesiRahasiaUji), ErrAuthUnavailable, true},
		{"galat DB baca scope = tolak, bukan scope all", sesiTandaTangan(t, sesiKlaimBaru("RS", "scope_rusak", 0), sesiRahasiaUji), ErrAuthUnavailable, true},
		{"token CCTV (typ lain) bukan sesi admin", sesiTandaTangan(t, cctv, sesiRahasiaUji), ErrAuthTokenInvalid, false},
		{"tanpa sub ditolak", sesiTandaTangan(t, tanpaSub, sesiRahasiaUji), ErrAuthTokenInvalid, false},
		{"klaim ver bukan angka ditolak", sesiTandaTangan(t, verString, sesiRahasiaUji), ErrAuthTokenInvalid, false},
		{"kedaluwarsa ditolak", sesiTandaTangan(t, kadaluarsa, sesiRahasiaUji), ErrAuthTokenInvalid, false},
		{"tanpa exp ditolak", sesiTandaTangan(t, tanpaExp, sesiRahasiaUji), ErrAuthTokenInvalid, false},
		{"secret lain ditolak", sesiTandaTangan(t, sesiKlaimBaru("A2", "manager", 2), "secret-lain-yang-juga-panjang-0123"), ErrAuthTokenInvalid, false},
		{"bukan JWT ditolak", "bukan-jwt", ErrAuthTokenInvalid, false},
	}
	for _, k := range kasus {
		InvalidateAllAdminSessions()
		muatAkun = 0
		sess, err := AuthenticateAdminJWT(k.token, sesiRahasiaUji)
		if k.mau == nil {
			if err != nil {
				t.Errorf("%s: mau diterima, dapat %v", k.nama, err)
				continue
			}
			if sess.ScopeType != "specific" || len(sess.OutletIDs) != 1 || sess.OutletIDs[0] != "O1" || len(sess.WorkUnitIDs) != 1 {
				t.Errorf("%s: scope sesi = %+v, mau specific [O1]/[W1] dari DB", k.nama, sess)
			}
			if sess.Role != "manager" || sess.Username == "" {
				t.Errorf("%s: identitas sesi = %+v", k.nama, sess)
			}
		} else if !errors.Is(err, k.mau) {
			t.Errorf("%s: galat = %v, mau %v", k.nama, err, k.mau)
		}
		if !k.bacaAkun && muatAkun != 0 {
			t.Errorf("%s: token ditolak sebelum DB, tetapi pemuat akun terpanggil %d kali", k.nama, muatAkun)
		}
	}
}

// Scope dengan jenis tak dikenal tidak pernah menjadi "semua".
func TestScopeTakDikenalDitutup(t *testing.T) {
	sesiPasangTiruan(t,
		func(string) (adminAccountState, error) {
			return adminAccountState{Found: true, Active: true, Role: "manager", Username: "ani"}, nil
		},
		func(string) (RoleScope, error) { return RoleScope{Type: "outlet"}, nil })
	sess, err := AuthenticateAdminJWT(sesiTandaTangan(t, sesiKlaimLama("A", "manager"), sesiRahasiaUji), sesiRahasiaUji)
	if err != nil {
		t.Fatal(err)
	}
	if sess.ScopeType != "specific" || sess.OutletIDs == nil || len(sess.OutletIDs) != 0 || sess.WorkUnitIDs == nil {
		t.Fatalf("scope tak dikenal harus specific kosong, dapat %+v", sess)
	}
}

// Cache: dalam TTL tidak query ulang; lewat TTL dimuat ulang; invalidasi
// berlaku seketika.
func TestAdminSessionCacheTTLDanInvalidasi(t *testing.T) {
	var muat int
	aktif := true
	sesiPasangTiruan(t,
		func(string) (adminAccountState, error) {
			muat++
			return adminAccountState{Found: true, Active: aktif, Role: "manager", Username: "ani"}, nil
		},
		func(string) (RoleScope, error) { return RoleScope{Type: "all"}, nil })
	jam := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	sessionClock = func() time.Time { return jam }
	tok := sesiTandaTangan(t, sesiKlaimLama("A", "manager"), sesiRahasiaUji)

	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); err != nil || muat != 1 {
		t.Fatalf("muat pertama: err=%v muat=%d", err, muat)
	}
	// Akun dinonaktifkan di luar layanan (mis. SQL langsung): selama TTL
	// keadaan lama dari cache masih dipakai — itulah batas keterlambatannya.
	aktif = false
	jam = jam.Add(AdminSessionCacheTTL - time.Second)
	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); err != nil || muat != 1 {
		t.Fatalf("dalam TTL harus dari cache: err=%v muat=%d", err, muat)
	}
	// Lewat TTL: dimuat ulang dan tertolak.
	jam = jam.Add(2 * time.Second)
	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); !errors.Is(err, ErrAuthAccountInactive) || muat != 2 {
		t.Fatalf("lewat TTL harus dimuat ulang & ditolak: err=%v muat=%d", err, muat)
	}
	// Invalidasi (dipanggil layanan setelah mengubah akun) berlaku tanpa menunggu TTL.
	aktif = true
	InvalidateAdminSession("A")
	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); err != nil || muat != 3 {
		t.Fatalf("setelah invalidasi harus dimuat ulang: err=%v muat=%d", err, muat)
	}
}

// Galat DB tidak di-cache: permintaan berikutnya mencoba lagi.
func TestAdminSessionCacheGalatTidakDisimpan(t *testing.T) {
	var muat int
	sesiPasangTiruan(t,
		func(string) (adminAccountState, error) {
			muat++
			if muat == 1 {
				return adminAccountState{}, errors.New("galat sementara")
			}
			return adminAccountState{Found: true, Active: true, Role: "manager", Username: "ani"}, nil
		},
		func(string) (RoleScope, error) { return RoleScope{Type: "all"}, nil })
	tok := sesiTandaTangan(t, sesiKlaimLama("A", "manager"), sesiRahasiaUji)
	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("galat DB harus ErrAuthUnavailable, dapat %v", err)
	}
	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); err != nil || muat != 2 {
		t.Fatalf("galat tidak boleh di-cache: err=%v muat=%d", err, muat)
	}
}

// Invalidasi yang terjadi SELAMA query berjalan (mis. akun dinonaktifkan
// bersamaan) membuat hasil muat itu tidak disimpan — keadaan lama tidak
// menempel di cache selama TTL.
func TestAdminSessionCacheInvalidasiSaatMemuat(t *testing.T) {
	var muat int
	sesiPasangTiruan(t,
		func(id string) (adminAccountState, error) {
			muat++
			if muat == 1 {
				InvalidateAdminSession(id)
			}
			return adminAccountState{Found: true, Active: true, Role: "manager", Username: "ani"}, nil
		},
		func(string) (RoleScope, error) { return RoleScope{Type: "all"}, nil })
	tok := sesiTandaTangan(t, sesiKlaimLama("A", "manager"), sesiRahasiaUji)
	for i := 0; i < 2; i++ {
		if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); err != nil {
			t.Fatal(err)
		}
	}
	if muat != 2 {
		t.Fatalf("hasil muat yang tersalip invalidasi tidak boleh disimpan: muat=%d, mau 2", muat)
	}
}

// Scope role di-cache per role dan selalu dikembalikan sebagai salinan.
func TestRoleScopeCacheSalinan(t *testing.T) {
	var muatScope int
	sesiPasangTiruan(t,
		func(string) (adminAccountState, error) {
			return adminAccountState{Found: true, Active: true, Role: "manager", Username: "ani"}, nil
		},
		func(string) (RoleScope, error) {
			muatScope++
			return RoleScope{Type: "specific", OutletIDs: []string{"O1"}, WorkUnitIDs: []string{"W1"}}, nil
		})
	tok := sesiTandaTangan(t, sesiKlaimLama("A", "manager"), sesiRahasiaUji)
	s1, err := AuthenticateAdminJWT(tok, sesiRahasiaUji)
	if err != nil {
		t.Fatal(err)
	}
	s1.OutletIDs[0] = "DIUBAH"
	s2, err := AuthenticateAdminJWT(tok, sesiRahasiaUji)
	if err != nil {
		t.Fatal(err)
	}
	if muatScope != 1 {
		t.Fatalf("scope dalam TTL harus dari cache: muat=%d", muatScope)
	}
	if s2.OutletIDs[0] != "O1" {
		t.Fatalf("mengubah slice sesi tidak boleh mengubah cache: %v", s2.OutletIDs)
	}
	InvalidateAllAdminSessions()
	if _, err := AuthenticateAdminJWT(tok, sesiRahasiaUji); err != nil || muatScope != 2 {
		t.Fatalf("InvalidateAllAdminSessions harus membuang cache scope: err=%v muat=%d", err, muatScope)
	}
}

// ── Dengan Postgres uji ─────────────────────────────────────────────────────

var (
	sesiMigrasiOnce sync.Once
	sesiMigrasiErr  error
)

// sesiButuhDB melewati tes tanpa Postgres uji dan memastikan kolom
// token_version ada (skema uji bisa dibuat oleh binary lama).
func sesiButuhDB(t *testing.T) {
	t.Helper()
	if !strings.HasSuffix(os.Getenv("DB_NAME"), "_test") || database.DB == nil || database.DB.Ping() != nil {
		t.Skip("butuh Postgres uji; set DB_HOST/DB_USER/DB_PASSWORD dan DB_NAME berakhiran _test")
	}
	sesiMigrasiOnce.Do(func() { sesiMigrasiErr = database.RunAuthMigrations() })
	if sesiMigrasiErr != nil {
		t.Fatalf("migrasi auth: %v", sesiMigrasiErr)
	}
	InvalidateAllAdminSessions()
}

type sesiAkunUji struct {
	ID, Username, Password, Role string
}

// sesiBuatRole membuat role uji (dihapus saat tes selesai).
func sesiBuatRole(t *testing.T, scopeType string) string {
	t.Helper()
	role := "ujs_" + strings.ToLower(NewULID()[16:]) // ≤ 20 huruf (cloud_admins.role)
	if _, err := CreateRole(models.CreateRoleRequest{
		Name: role, Description: "uji sesi", ScopeType: scopeType, Permissions: []string{"dashboard"},
	}); err != nil {
		t.Fatalf("buat role uji: %v", err)
	}
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM role_work_unit_scope WHERE role = $1`, role)
		database.DB.Exec(`DELETE FROM role_permissions WHERE role = $1`, role)
		database.DB.Exec(`DELETE FROM roles WHERE name = $1`, role)
	})
	return role
}

// sesiBuatAkun membuat role + akun aktif untuk satu tes.
func sesiBuatAkun(t *testing.T, scopeType string) sesiAkunUji {
	t.Helper()
	role := sesiBuatRole(t, scopeType)
	a := sesiAkunUji{Username: "ujisesi_" + strings.ToLower(NewULID()[16:]), Password: "rahasia-uji-123", Role: role}
	admin, err := CreateAdmin(models.CreateAdminRequest{Username: a.Username, Password: a.Password, Name: "Uji Sesi", Role: role})
	if err != nil {
		t.Fatalf("buat akun uji: %v", err)
	}
	a.ID = admin.ID
	t.Cleanup(func() {
		database.DB.Exec(`DELETE FROM cloud_admins WHERE id = $1`, a.ID)
		InvalidateAllAdminSessions()
	})
	return a
}

func sesiLogin(t *testing.T, username, password string) *models.AdminLoginResponse {
	t.Helper()
	res, err := AdminLogin(models.AdminLoginRequest{Username: username, Password: password}, sesiRahasiaUji)
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return res
}

func sesiHarus(t *testing.T, langkah, token string, mau error) *AdminSession {
	t.Helper()
	sess, err := AuthenticateAdminJWT(token, sesiRahasiaUji)
	if mau == nil && err != nil {
		t.Fatalf("%s: token harus diterima, dapat %v", langkah, err)
	}
	if mau != nil && !errors.Is(err, mau) {
		t.Fatalf("%s: galat = %v, mau %v", langkah, err, mau)
	}
	return sess
}

// sesiConnectorGagal: koneksi Postgres sungguhan yang menolak query berisi
// pola tertentu — meniru galat DB sesaat tepat di satu langkah (mis. baca
// scope), sementara query lain tetap jalan.
type sesiConnectorGagal struct {
	dasar *pq.Connector
	pola  string
}

func (k sesiConnectorGagal) Connect(ctx context.Context) (driver.Conn, error) {
	c, err := k.dasar.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return sesiConnGagal{Conn: c, pola: k.pola}, nil
}

func (k sesiConnectorGagal) Driver() driver.Driver { return k.dasar.Driver() }

type sesiConnGagal struct {
	driver.Conn
	pola string
}

func (c sesiConnGagal) Prepare(q string) (driver.Stmt, error) {
	if strings.Contains(q, c.pola) {
		return nil, fmt.Errorf("galat DB uji pada query berisi %q", c.pola)
	}
	return c.Conn.Prepare(q)
}

// sesiPasangDBGagal mengganti database.DB selama sisa tes.
func sesiPasangDBGagal(t *testing.T, pola string) {
	t.Helper()
	dasar, err := pq.NewConnector(config.Load().DSN())
	if err != nil {
		t.Fatal(err)
	}
	gagal := sql.OpenDB(sesiConnectorGagal{dasar: dasar, pola: pola})
	asli := database.DB
	database.DB = gagal
	InvalidateAllAdminSessions()
	t.Cleanup(func() {
		database.DB = asli
		gagal.Close()
		InvalidateAllAdminSessions()
	})
}

func TestTokenLamaTanpaVersiDiterimaBilaVersiNolDB(t *testing.T) {
	sesiButuhDB(t)
	a := sesiBuatAkun(t, "all")
	lama := sesiTandaTangan(t, sesiKlaimLama(a.ID, a.Role), sesiRahasiaUji)
	sess := sesiHarus(t, "token lama, versi akun 0", lama, nil)
	if sess.ID != a.ID || sess.Role != a.Role || sess.ScopeType != "all" {
		t.Fatalf("sesi dari token lama = %+v", sess)
	}
	// Token baru membawa ver = token_version akun.
	baru := sesiLogin(t, a.Username, a.Password).Token
	klaim, err := ValidateJWT(baru, sesiRahasiaUji)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := claimTokenVersion(klaim); !ok || v != 0 || klaim["typ"] != adminTokenType {
		t.Fatalf("klaim token baru: ver=%v typ=%v", klaim["ver"], klaim["typ"])
	}
	sesiHarus(t, "token baru, versi 0", baru, nil)

	// Begitu versi akun naik, token lama tanpa ver ikut tertolak.
	if _, err := ToggleAdminActive(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ToggleAdminActive(a.ID); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "token lama setelah versi naik", lama, ErrAuthSessionRevoked)
}

func TestSesiDicabutSetelahPasswordDigantiDB(t *testing.T) {
	sesiButuhDB(t)
	a := sesiBuatAkun(t, "all")
	baru := sesiLogin(t, a.Username, a.Password).Token
	lama := sesiTandaTangan(t, sesiKlaimLama(a.ID, a.Role), sesiRahasiaUji)
	sesiHarus(t, "sebelum ganti password", baru, nil)

	if err := ChangePassword(a.ID, models.ChangePasswordRequest{CurrentPassword: a.Password, NewPassword: "rahasia-baru-456"}); err != nil {
		t.Fatalf("ganti password: %v", err)
	}
	sesiHarus(t, "token login setelah ganti password", baru, ErrAuthSessionRevoked)
	sesiHarus(t, "token lama setelah ganti password", lama, ErrAuthSessionRevoked)

	kedua := sesiLogin(t, a.Username, "rahasia-baru-456").Token
	sesiHarus(t, "login ulang dengan password baru", kedua, nil)

	// Reset oleh admin lain juga mencabut sesi.
	if err := ResetAdminPassword(a.ID, models.ResetAdminPasswordRequest{NewPassword: "direset-789"}); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	sesiHarus(t, "token setelah reset password", kedua, ErrAuthSessionRevoked)
	sesiHarus(t, "login dengan password hasil reset", sesiLogin(t, a.Username, "direset-789").Token, nil)
}

func TestAkunNonaktifDitolakDB(t *testing.T) {
	sesiButuhDB(t)
	a := sesiBuatAkun(t, "all")
	tok := sesiLogin(t, a.Username, a.Password).Token
	sesiHarus(t, "akun aktif", tok, nil)

	if adm, err := ToggleAdminActive(a.ID); err != nil || adm.IsActive {
		t.Fatalf("nonaktifkan: %v (aktif=%v)", err, adm != nil && adm.IsActive)
	}
	sesiHarus(t, "akun dinonaktifkan", tok, ErrAuthAccountInactive)
	if _, err := AdminLogin(models.AdminLoginRequest{Username: a.Username, Password: a.Password}, sesiRahasiaUji); err == nil {
		t.Fatal("akun nonaktif tidak boleh bisa login")
	}

	// Diaktifkan kembali: token lama tetap mati, login baru berlaku.
	if _, err := ToggleAdminActive(a.ID); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "token lama setelah diaktifkan kembali", tok, ErrAuthSessionRevoked)
	tok2 := sesiLogin(t, a.Username, a.Password).Token
	sesiHarus(t, "login setelah diaktifkan kembali", tok2, nil)

	// Nonaktif lewat SQL langsung (di luar layanan, tanpa invalidasi cache):
	// tertolak paling lambat setelah cache kedaluwarsa.
	if _, err := database.DB.Exec(`UPDATE cloud_admins SET is_active = false WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "dalam TTL cache", tok2, nil)
	asliJam := sessionClock
	sessionClock = func() time.Time { return time.Now().Add(AdminSessionCacheTTL + time.Second) }
	t.Cleanup(func() { sessionClock = asliJam })
	sesiHarus(t, "setelah cache kedaluwarsa", tok2, ErrAuthAccountInactive)

	// Akun dihapus → tertolak.
	sessionClock = asliJam
	if _, err := database.DB.Exec(`UPDATE cloud_admins SET is_active = true WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	InvalidateAdminSession(a.ID)
	sesiHarus(t, "aktif lagi", tok2, nil)
	if err := DeleteAdmin(a.ID); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "akun dihapus", tok2, ErrAuthSessionRevoked)
}

func TestRoleDanScopeBerubahMencabutSesiDB(t *testing.T) {
	sesiButuhDB(t)
	a := sesiBuatAkun(t, "all")
	tok := sesiLogin(t, a.Username, a.Password).Token

	// Ubah nama saja dan simpan ulang scope yang sama: sesi tetap berlaku.
	if _, err := UpdateAdmin(a.ID, models.UpdateAdminRequest{Name: "Nama Baru", Role: a.Role}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateRoleScope(a.Role, models.UpdateRoleScopeRequest{ScopeType: "all"}); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "tanpa perubahan role/scope", tok, nil)

	// Scope role berubah → sesi dicabut; login ulang mendapat scope baru.
	if err := UpdateRoleScope(a.Role, models.UpdateRoleScopeRequest{ScopeType: "specific"}); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "scope role berubah", tok, ErrAuthSessionRevoked)
	res := sesiLogin(t, a.Username, a.Password)
	if res.ScopeType != "specific" || res.ScopeOutletIDs == nil || len(res.ScopeOutletIDs) != 0 {
		t.Fatalf("login setelah scope specific tanpa unit: scope=%q outlets=%v", res.ScopeType, res.ScopeOutletIDs)
	}
	sess := sesiHarus(t, "token scope baru", res.Token, nil)
	if sess.ScopeType != "specific" || len(sess.OutletIDs) != 0 || sess.OutletIDs == nil {
		t.Fatalf("sesi scope specific tanpa unit harus kosong (tidak melihat outlet): %+v", sess)
	}
	if err := UpdateRoleScope(a.Role, models.UpdateRoleScopeRequest{ScopeType: "specific", WorkUnitIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "simpan ulang scope specific yang sama", res.Token, nil)

	// Role akun diganti → sesi dicabut.
	role2 := sesiBuatRole(t, "all")
	if _, err := UpdateAdminRole(a.ID, models.UpdateAdminRoleRequest{Role: role2}); err != nil {
		t.Fatal(err)
	}
	sesiHarus(t, "role akun diganti", res.Token, ErrAuthSessionRevoked)
	tok3 := sesiLogin(t, a.Username, a.Password).Token
	sesiHarus(t, "login dengan role baru", tok3, nil)
}

func TestResolveRoleScopeGagalTertutupDB(t *testing.T) {
	sesiButuhDB(t)
	// Role yang tidak terdaftar: dulu jatuh ke "all".
	tidakAda := "tak_terdaftar_" + strings.ToLower(NewULID()[18:])
	sc, err := ResolveRoleScope(tidakAda)
	if err != nil || sc.Type != "specific" || sc.OutletIDs == nil || len(sc.OutletIDs) != 0 || len(sc.WorkUnitIDs) != 0 {
		t.Fatalf("role tak terdaftar harus specific kosong: %+v err=%v", sc, err)
	}
	if st, ids := GetRoleScopeOutletIDs(tidakAda); st != "specific" || ids == nil || len(ids) != 0 {
		t.Fatalf("GetRoleScopeOutletIDs role tak terdaftar = (%q, %v)", st, ids)
	}

	semua := sesiBuatRole(t, "all")
	if sc, err := ResolveRoleScope(semua); err != nil || sc.Type != "all" {
		t.Fatalf("role scope all: %+v err=%v", sc, err)
	}
	khusus := sesiBuatRole(t, "specific")

	// Galat DB pada langkah mana pun → error, bukan "all".
	for _, pola := range []string{"scope_type", "role_work_unit_scope"} {
		t.Run(pola, func(t *testing.T) {
			sesiPasangDBGagal(t, pola)
			for _, role := range []string{semua, khusus} {
				if pola == "role_work_unit_scope" && role == semua {
					continue // role all tidak membaca unit kerja
				}
				if sc, err := ResolveRoleScope(role); err == nil {
					t.Fatalf("role %s: galat DB harus dikembalikan, dapat %+v", role, sc)
				}
				if st, ids := GetRoleScopeOutletIDs(role); st != "specific" || len(ids) != 0 {
					t.Fatalf("role %s: GetRoleScopeOutletIDs saat galat = (%q, %v), mau specific kosong", role, st, ids)
				}
			}
		})
	}
}

func TestLoginDanSesiDitolakBilaScopeGagalDB(t *testing.T) {
	sesiButuhDB(t)
	kasus := []struct{ scope, pola string }{
		{"all", "scope_type"},                // gagal di baca jenis scope
		{"specific", "role_work_unit_scope"}, // gagal di baca outlet/unit kerja
	}
	for _, k := range kasus {
		t.Run(k.scope+"/"+k.pola, func(t *testing.T) {
			a := sesiBuatAkun(t, k.scope)
			tok := sesiLogin(t, a.Username, a.Password).Token
			sesiHarus(t, "sebelum galat", tok, nil)

			sesiPasangDBGagal(t, k.pola)
			res, err := AdminLogin(models.AdminLoginRequest{Username: a.Username, Password: a.Password}, sesiRahasiaUji)
			if !errors.Is(err, ErrAuthUnavailable) || res != nil {
				t.Fatalf("login saat scope gagal dibaca harus ditolak ErrAuthUnavailable, dapat res=%v err=%v", res, err)
			}
			sesiHarus(t, "validasi sesi saat scope gagal dibaca", tok, ErrAuthUnavailable)
		})
	}
}

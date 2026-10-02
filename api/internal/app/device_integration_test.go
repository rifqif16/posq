package app_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/platform/database"
)

const cashierPIN = "135790"

type deviceCred struct {
	id     string
	secret string
	code   string
}

func registerDevice(t *testing.T, base string, owner tenantSession, name string) (deviceCred, *http.Response, map[string]any) {
	t.Helper()
	res, out := call(t, "POST", base+"/v1/devices", map[string]any{"store_id": owner.storeID.String(), "name": name}, owner.token)
	if res.StatusCode != http.StatusCreated {
		return deviceCred{}, res, out
	}
	return deviceCred{id: out["id"].(string), secret: out["secret"].(string), code: out["code"].(string)}, res, out
}

func mustDevice(t *testing.T, base string, owner tenantSession, name string) deviceCred {
	t.Helper()
	d, res, out := registerDevice(t, base, owner, name)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("daftar perangkat: %d %v", res.StatusCode, out)
	}
	return d
}

func cashierWithPIN(t *testing.T, base string, owner tenantSession, name, email, pin string) string {
	t.Helper()
	body := staffBody(name, email, "cashier", owner.storeID)
	if pin != "" {
		body["pin"] = pin
	}
	res, out := createStaffAs(t, base, owner.token, body)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("kasir %s: %d %v", name, res.StatusCode, out)
	}
	return out["id"].(string)
}

func pinLogin(t *testing.T, base string, d deviceCred, userID, pin string) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/auth/pin-login", map[string]any{"device_id": d.id, "device_secret": d.secret, "user_id": userID, "pin": pin}, "")
}

func pinUsers(t *testing.T, base string, d deviceCred) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/auth/pin-users", map[string]any{"device_id": d.id, "device_secret": d.secret}, "")
}

func deviceFromList(t *testing.T, base, token, id string) map[string]any {
	t.Helper()
	_, out := call(t, "GET", base+"/v1/devices", nil, token)
	for _, it := range out["items"].([]any) {
		if m := it.(map[string]any); m["id"] == id {
			return m
		}
	}
	t.Fatalf("perangkat %s tidak ada: %v", id, out)
	return nil
}

func pinLockState(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, userID string) (int, *time.Time) {
	t.Helper()
	var attempts int
	var until *time.Time
	err := database.WithTenantTx(context.Background(), pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT pin_failed_attempts, pin_locked_until FROM users WHERE id = $1`, userID).Scan(&attempts, &until)
	})
	if err != nil {
		t.Fatal(err)
	}
	return attempts, until
}

func TestDeviceRegistrationListRevokeAndLimit(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	d1, res, out := registerDevice(t, srv.URL, owner, "  Kasir Depan ")
	if res.StatusCode != http.StatusCreated || d1.code != "K01" || out["name"] != "Kasir Depan" || out["status"] != "active" ||
		len(d1.secret) < 40 || out["registered_by"] != "Sari" || out["last_seen_at"] != nil {
		t.Fatalf("buat: %d %v", res.StatusCode, out)
	}
	d2 := mustDevice(t, srv.URL, owner, "Kasir Belakang")
	d3 := mustDevice(t, srv.URL, owner, "Tablet Dapur")
	if d2.code != "K02" || d3.code != "K03" || d1.secret == d2.secret {
		t.Fatalf("kode berurutan dan rahasia unik: %v %v %v", d1, d2, d3)
	}

	_, list := call(t, "GET", srv.URL+"/v1/devices", nil, owner.token)
	items := list["items"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["code"] != "K01" {
		t.Fatalf("daftar: %v", items)
	}
	for _, it := range items {
		if _, leaked := it.(map[string]any)["secret"]; leaked {
			t.Fatal("rahasia tidak boleh muncul di daftar")
		}
	}

	if res, out := call(t, "POST", srv.URL+"/v1/devices", map[string]any{"store_id": owner.storeID.String(), "name": "Keempat"}, owner.token); res.StatusCode != http.StatusConflict || out["code"] != "DEVICE_LIMIT_REACHED" {
		t.Fatalf("batas 3 perangkat: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "POST", srv.URL+"/v1/devices/"+d2.id+"/revoke", nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("cabut: %d", res.StatusCode)
	}
	if res, _ := call(t, "POST", srv.URL+"/v1/devices/"+d2.id+"/revoke", nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("cabut ulang harus idempoten: %d", res.StatusCode)
	}
	if deviceFromList(t, srv.URL, owner.token, d2.id)["status"] != "revoked" {
		t.Fatal("status harus revoked")
	}
	d4 := mustDevice(t, srv.URL, owner, "Pengganti")
	if d4.code != "K04" {
		t.Fatalf("kode tidak dipakai ulang: %s", d4.code)
	}

	if res, _ := call(t, "POST", srv.URL+"/v1/devices/"+uuid.NewString()+"/revoke", nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("cabut acak: %d", res.StatusCode)
	}
	if res, _ := call(t, "POST", srv.URL+"/v1/devices/bukan-uuid/revoke", nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("cabut non-uuid: %d", res.StatusCode)
	}
}

func TestDeviceValidationAndAuthorization(t *testing.T) {
	srv, _ := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashierWithPIN(t, srv.URL, owner, "Kasir", em("kasir"), cashierPIN)
	_, cashierToken := loginAs(t, srv.URL, em("kasir"), staffPassword)

	for name, body := range map[string]map[string]any{
		"nama kosong":  {"store_id": owner.storeID.String(), "name": " "},
		"nama panjang": {"store_id": owner.storeID.String(), "name": strings.Repeat("a", 61)},
		"tanpa outlet": {"name": "X"},
	} {
		if res, out := call(t, "POST", srv.URL+"/v1/devices", body, owner.token); res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "VALIDATION_FAILED" {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	if res, out := call(t, "POST", srv.URL+"/v1/devices", map[string]any{"store_id": other.storeID.String(), "name": "Curang"}, owner.token); res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_STORE" {
		t.Fatalf("outlet tenant lain: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/devices", nil, cashierToken); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh melihat perangkat: %d", res.StatusCode)
	}
	if res, _ := call(t, "POST", srv.URL+"/v1/devices", map[string]any{"store_id": owner.storeID.String(), "name": "X"}, cashierToken); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh mendaftarkan: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/devices", nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}

	d := mustDevice(t, srv.URL, owner, "Kasir Depan")
	if res, _ := call(t, "POST", srv.URL+"/v1/devices/"+d.id+"/revoke", nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("cabut lintas tenant: %d", res.StatusCode)
	}
	if deviceFromList(t, srv.URL, owner.token, d.id)["status"] != "active" {
		t.Fatal("perangkat tenant lain tidak boleh berubah")
	}
}

func TestPinLoginFlow(t *testing.T) {
	srv, _ := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	cashierID := cashierWithPIN(t, srv.URL, owner, "Budi", em("budi"), cashierPIN)
	cashierWithPIN(t, srv.URL, owner, "Tanpa PIN", em("nopin"), "")
	adminBody := staffBody("Admin", em("admin"), "admin", owner.storeID)
	adminBody["pin"] = "246802"
	if res, out := createStaffAs(t, srv.URL, owner.token, adminBody); res.StatusCode != http.StatusCreated {
		t.Fatalf("admin: %d %v", res.StatusCode, out)
	}
	d := mustDevice(t, srv.URL, owner, "Kasir Depan")

	res, users := pinUsers(t, srv.URL, d)
	items := users["items"].([]any)
	if res.StatusCode != http.StatusOK || users["device_name"] != "Kasir Depan" || users["store_id"] != owner.storeID.String() ||
		len(items) != 1 || items[0].(map[string]any)["name"] != "Budi" || items[0].(map[string]any)["role"] != "cashier" {
		t.Fatalf("hanya kasir/dapur aktif ber-PIN yang tampil: %d %v", res.StatusCode, users)
	}

	res, out := pinLogin(t, srv.URL, d, cashierID, cashierPIN)
	if res.StatusCode != http.StatusOK || out["access_token"] == "" || out["user"].(map[string]any)["role"] != "cashier" {
		t.Fatalf("pin login: %d %v", res.StatusCode, out)
	}
	var refresh bool
	for _, c := range res.Cookies() {
		refresh = refresh || (c.Name == "posq_rt" && c.Value != "" && c.HttpOnly)
	}
	if !refresh {
		t.Fatal("cookie refresh HttpOnly harus terpasang")
	}
	token := out["access_token"].(string)
	if res, me := call(t, "GET", srv.URL+"/v1/me", nil, token); res.StatusCode != http.StatusOK || me["user"].(map[string]any)["name"] != "Budi" {
		t.Fatalf("token PIN harus valid: %d %v", res.StatusCode, me)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/staff", nil, token); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir lewat PIN tidak punya akses back-office: %d", res.StatusCode)
	}
	if deviceFromList(t, srv.URL, owner.token, d.id)["last_seen_at"] == nil {
		t.Fatal("last_seen_at harus terisi setelah login")
	}
}

func TestPinLoginRejections(t *testing.T) {
	srv, pool := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashierID := cashierWithPIN(t, srv.URL, owner, "Budi", em("budi"), cashierPIN)
	noPIN := cashierWithPIN(t, srv.URL, owner, "Tanpa PIN", em("nopin"), "")
	disabledID := cashierWithPIN(t, srv.URL, owner, "Nonaktif", em("off"), "864209")
	adminBody := staffBody("Admin", em("admin"), "admin", owner.storeID)
	adminBody["pin"] = "246802"
	_, admin := createStaffAs(t, srv.URL, owner.token, adminBody)
	d := mustDevice(t, srv.URL, owner, "Kasir Depan")
	otherDevice := mustDevice(t, srv.URL, other, "Tenant Lain")

	expect := func(name string, status int, code string, res *http.Response, out map[string]any) {
		t.Helper()
		if res.StatusCode != status || out["code"] != code {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	res, out := pinLogin(t, srv.URL, d, cashierID, "000111")
	expect("PIN salah", 401, "INVALID_CREDENTIALS", res, out)
	res, out = pinLogin(t, srv.URL, deviceCred{id: d.id, secret: "rahasia-salah"}, cashierID, cashierPIN)
	expect("rahasia perangkat salah", 401, "INVALID_CREDENTIALS", res, out)
	res, out = pinLogin(t, srv.URL, deviceCred{id: uuid.NewString(), secret: d.secret}, cashierID, cashierPIN)
	expect("perangkat tidak ada", 401, "INVALID_CREDENTIALS", res, out)
	res, out = pinLogin(t, srv.URL, d, noPIN, cashierPIN)
	expect("tanpa PIN", 401, "INVALID_CREDENTIALS", res, out)
	res, out = pinLogin(t, srv.URL, d, admin["id"].(string), "246802")
	expect("admin tidak boleh login PIN", 401, "INVALID_CREDENTIALS", res, out)
	res, out = pinLogin(t, srv.URL, d, uuid.NewString(), cashierPIN)
	expect("pengguna tidak ada", 401, "INVALID_CREDENTIALS", res, out)
	res, out = pinLogin(t, srv.URL, otherDevice, cashierID, cashierPIN)
	expect("perangkat tenant lain", 401, "INVALID_CREDENTIALS", res, out)

	disable := map[string]any{"name": "Nonaktif", "role": "cashier", "status": "disabled", "store_ids": []string{owner.storeID.String()}}
	if res, o := call(t, "PATCH", srv.URL+"/v1/staff/"+disabledID, disable, owner.token); res.StatusCode != http.StatusOK {
		t.Fatalf("nonaktifkan: %d %v", res.StatusCode, o)
	}
	res, out = pinLogin(t, srv.URL, d, disabledID, "864209")
	expect("akun nonaktif", 403, "ACCOUNT_DISABLED", res, out)

	if res, _ := call(t, "POST", srv.URL+"/v1/devices/"+d.id+"/revoke", nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("cabut: %d", res.StatusCode)
	}
	res, out = pinLogin(t, srv.URL, d, cashierID, cashierPIN)
	expect("perangkat dicabut", 401, "INVALID_CREDENTIALS", res, out)
	if res, o := pinUsers(t, srv.URL, d); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("daftar pengguna dari perangkat dicabut: %d %v", res.StatusCode, o)
	}
	if attempts, _ := pinLockState(t, pool, owner.tenantID, cashierID); attempts != 1 {
		t.Fatalf("hanya satu PIN salah yang dihitung, dapat %d", attempts)
	}
}

func TestPinLockoutAfterFiveFailures(t *testing.T) {
	srv, pool := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	cashierID := cashierWithPIN(t, srv.URL, owner, "Budi", em("budi"), cashierPIN)
	d := mustDevice(t, srv.URL, owner, "Kasir Depan")

	for i := 1; i <= 4; i++ {
		if res, out := pinLogin(t, srv.URL, d, cashierID, "000111"); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("percobaan %d: %d %v", i, res.StatusCode, out)
		}
	}
	res, out := pinLogin(t, srv.URL, d, cashierID, "000111")
	if res.StatusCode != http.StatusTooManyRequests || out["code"] != "PIN_LOCKED" || res.Header.Get("Retry-After") != "900" {
		t.Fatalf("percobaan kelima mengunci: %d %v %v", res.StatusCode, out, res.Header)
	}
	if res, out := pinLogin(t, srv.URL, d, cashierID, cashierPIN); res.StatusCode != http.StatusTooManyRequests || out["code"] != "PIN_LOCKED" {
		t.Fatalf("PIN benar pun ditolak saat terkunci: %d %v", res.StatusCode, out)
	}
	if _, until := pinLockState(t, pool, owner.tenantID, cashierID); until == nil || time.Until(*until) < 14*time.Minute {
		t.Fatalf("kunci harus ~15 menit: %v", until)
	}

	err := database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `UPDATE users SET pin_locked_until = now() - interval '1 minute' WHERE id = $1`, cashierID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if res, out := pinLogin(t, srv.URL, d, cashierID, cashierPIN); res.StatusCode != http.StatusOK {
		t.Fatalf("setelah kunci berakhir: %d %v", res.StatusCode, out)
	}
	if attempts, until := pinLockState(t, pool, owner.tenantID, cashierID); attempts != 0 || until != nil {
		t.Fatalf("login berhasil harus mereset: %d %v", attempts, until)
	}
}

func TestPinSuccessResetsFailureCounter(t *testing.T) {
	srv, pool := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	cashierID := cashierWithPIN(t, srv.URL, owner, "Budi", em("budi"), cashierPIN)
	d := mustDevice(t, srv.URL, owner, "Kasir Depan")

	for i := 0; i < 3; i++ {
		pinLogin(t, srv.URL, d, cashierID, "000111")
	}
	if attempts, _ := pinLockState(t, pool, owner.tenantID, cashierID); attempts != 3 {
		t.Fatalf("tiga kegagalan: %d", attempts)
	}
	if res, _ := pinLogin(t, srv.URL, d, cashierID, cashierPIN); res.StatusCode != http.StatusOK {
		t.Fatalf("login: %d", res.StatusCode)
	}
	for i := 1; i <= 4; i++ {
		if res, out := pinLogin(t, srv.URL, d, cashierID, "000111"); res.StatusCode != http.StatusUnauthorized {
			t.Fatalf("hitungan harus mulai dari nol, percobaan %d: %d %v", i, res.StatusCode, out)
		}
	}
}

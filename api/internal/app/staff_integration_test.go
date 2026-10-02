package app_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/platform/database"
)

const staffPassword = "password-aman-1"

func mails() func(string) string {
	suffix := uuid.NewString()[:8]
	return func(name string) string { return name + "-" + suffix + "@kedai.id" }
}

func staffBody(name, email, role string, stores ...uuid.UUID) map[string]any {
	ids := make([]string, len(stores))
	for i, s := range stores {
		ids[i] = s.String()
	}
	return map[string]any{"name": name, "email": email, "password": staffPassword, "role": role, "store_ids": ids}
}

func createStaffAs(t *testing.T, base, token string, body map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/staff", body, token)
}

func loginAs(t *testing.T, base, email, password string) (int, string) {
	t.Helper()
	res, out := call(t, "POST", base+"/v1/auth/login", map[string]any{"email": email, "password": password}, "")
	token, _ := out["access_token"].(string)
	return res.StatusCode, token
}

func staffList(t *testing.T, base, token string) map[string]any {
	t.Helper()
	res, out := call(t, "GET", base+"/v1/staff", nil, token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("daftar staf: %d %v", res.StatusCode, out)
	}
	return out
}

func memberByEmail(t *testing.T, list map[string]any, email string) map[string]any {
	t.Helper()
	for _, it := range list["items"].([]any) {
		if m := it.(map[string]any); m["email"] == email {
			return m
		}
	}
	t.Fatalf("staf %s tidak ada: %v", email, list["items"])
	return nil
}

func activeRefreshTokens(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, email string) int {
	t.Helper()
	var n int
	err := database.WithTenantTx(context.Background(), pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(context.Background(), `SELECT count(*) FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id
			WHERE u.email = $1 AND rt.revoked_at IS NULL`, email).Scan(&n)
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func addStore(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, code string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := database.WithTenantTx(context.Background(), pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `INSERT INTO stores (id, tenant_id, code, name) VALUES ($1,$2,$3,$4)`, id, tenantID, code, "Cabang "+code)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestStaffCreateListLoginAndPIN(t *testing.T) {
	srv, _ := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)

	res, created := createStaffAs(t, srv.URL, owner.token, staffBody("  Budi Kasir ", strings.ToUpper(em("kasir")[:1])+em("kasir")[1:], "cashier", owner.storeID))
	if res.StatusCode != http.StatusCreated || created["email"] != em("kasir") || created["name"] != "Budi Kasir" ||
		created["role"] != "cashier" || created["status"] != "active" || created["has_pin"] != false || created["manageable"] != true {
		t.Fatalf("create: %d %v", res.StatusCode, created)
	}
	if status, token := loginAs(t, srv.URL, em("kasir"), staffPassword); status != http.StatusOK || token == "" {
		t.Fatalf("kasir harus bisa login: %d", status)
	}

	list := staffList(t, srv.URL, owner.token)
	items := list["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["role"] != "owner" || items[0].(map[string]any)["manageable"] != false {
		t.Fatalf("owner harus pertama dan tidak dapat dikelola: %v", items)
	}
	if roles := fmt.Sprint(list["assignable_roles"]); roles != "[admin cashier kitchen]" {
		t.Fatalf("assignable_roles: %s", roles)
	}

	id := created["id"].(string)
	if res, _ := call(t, "PUT", srv.URL+"/v1/staff/"+id+"/pin", map[string]any{"pin": "135790"}, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("set PIN: %d", res.StatusCode)
	}
	if memberByEmail(t, staffList(t, srv.URL, owner.token), em("kasir"))["has_pin"] != true {
		t.Fatal("has_pin harus true")
	}
	if res, out := call(t, "PUT", srv.URL+"/v1/staff/"+id+"/pin", map[string]any{"pin": "111111"}, owner.token); res.StatusCode != http.StatusUnprocessableEntity || out["errors"] == nil {
		t.Fatalf("PIN lemah: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/staff/"+id+"/pin", nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("hapus PIN: %d", res.StatusCode)
	}
	if memberByEmail(t, staffList(t, srv.URL, owner.token), em("kasir"))["has_pin"] != false {
		t.Fatal("has_pin harus false")
	}

	_, stores := call(t, "GET", srv.URL+"/v1/stores", nil, owner.token)
	if s := stores["items"].([]any); len(s) != 1 || s[0].(map[string]any)["id"] != owner.storeID.String() {
		t.Fatalf("daftar outlet: %v", stores)
	}
}

func TestStaffValidationAndConflicts(t *testing.T) {
	srv, _ := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	foreignStore := other.storeID

	cases := map[string]struct {
		mutate func(map[string]any)
		status int
		code   string
		field  string
	}{
		"password pendek": {func(b map[string]any) { b["password"] = "pendek" }, 422, "VALIDATION_FAILED", "password"},
		"role owner":      {func(b map[string]any) { b["role"] = "owner" }, 422, "VALIDATION_FAILED", "role"},
		"tanpa outlet":    {func(b map[string]any) { b["store_ids"] = []string{} }, 422, "VALIDATION_FAILED", "store_ids"},
		"pin lemah":       {func(b map[string]any) { b["pin"] = "123456" }, 422, "VALIDATION_FAILED", "pin"},
		"email salah":     {func(b map[string]any) { b["email"] = "bukan-email" }, 422, "VALIDATION_FAILED", "email"},
		"outlet asing":    {func(b map[string]any) { b["store_ids"] = []string{foreignStore.String()} }, 422, "INVALID_STORE", "store_ids[0]"},
	}
	for name, c := range cases {
		body := staffBody("Budi", em("budi"), "cashier", owner.storeID)
		c.mutate(body)
		res, out := createStaffAs(t, srv.URL, owner.token, body)
		errs, _ := out["errors"].([]any)
		if res.StatusCode != c.status || out["code"] != c.code || len(errs) == 0 || errs[0].(map[string]any)["field"] != c.field {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	if n := len(staffList(t, srv.URL, owner.token)["items"].([]any)); n != 1 {
		t.Fatalf("validasi gagal tidak boleh membuat pengguna, ada %d", n)
	}

	if res, out := createStaffAs(t, srv.URL, other.token, staffBody("Dua", em("dup"), "cashier", other.storeID)); res.StatusCode != http.StatusCreated {
		t.Fatalf("seed: %d %v", res.StatusCode, out)
	}
	res, out := createStaffAs(t, srv.URL, owner.token, staffBody("Satu", em("dup"), "cashier", owner.storeID))
	if res.StatusCode != http.StatusConflict || out["code"] != "EMAIL_TAKEN" {
		t.Fatalf("email dipakai tenant lain: %d %v", res.StatusCode, out)
	}
}

func TestStaffRoleBoundaries(t *testing.T) {
	srv, pool := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	secondStore := addStore(t, pool, owner.tenantID, "CAB2")

	for _, e := range []struct{ name, email, role string }{
		{"Kasir", em("kasir"), "cashier"}, {"Admin Satu", em("admin1"), "admin"}, {"Admin Dua", em("admin2"), "admin"},
	} {
		if res, out := createStaffAs(t, srv.URL, owner.token, staffBody(e.name, e.email, e.role, owner.storeID)); res.StatusCode != http.StatusCreated {
			t.Fatalf("seed %s: %d %v", e.email, res.StatusCode, out)
		}
	}
	_, cashierToken := loginAs(t, srv.URL, em("kasir"), staffPassword)
	_, adminToken := loginAs(t, srv.URL, em("admin1"), staffPassword)

	if res, out := call(t, "GET", srv.URL+"/v1/staff", nil, cashierToken); res.StatusCode != http.StatusForbidden || out["code"] != "FORBIDDEN" {
		t.Fatalf("kasir tidak boleh melihat staf: %d %v", res.StatusCode, out)
	}
	if res, _ := createStaffAs(t, srv.URL, cashierToken, staffBody("X", em("x"), "cashier", owner.storeID)); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh membuat: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/staff", nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}

	list := staffList(t, srv.URL, adminToken)
	if fmt.Sprint(list["assignable_roles"]) != "[cashier]" || memberByEmail(t, list, em("kasir"))["manageable"] != true ||
		memberByEmail(t, list, em("admin2"))["manageable"] != false || memberByEmail(t, list, em("admin1"))["manageable"] != false {
		t.Fatalf("hak admin pada daftar: %v", list)
	}
	ownerID := list["items"].([]any)[0].(map[string]any)["id"].(string)
	admin2ID := memberByEmail(t, list, em("admin2"))["id"].(string)
	cashierID := memberByEmail(t, list, em("kasir"))["id"].(string)

	if res, out := createStaffAs(t, srv.URL, adminToken, staffBody("Baru", em("baru"), "admin", owner.storeID)); res.StatusCode != http.StatusForbidden || out["code"] != "ROLE_NOT_ALLOWED" {
		t.Fatalf("admin membuat admin: %d %v", res.StatusCode, out)
	}
	if res, _ := createStaffAs(t, srv.URL, adminToken, staffBody("Dapur", em("dapur"), "kitchen", owner.storeID)); res.StatusCode != http.StatusForbidden {
		t.Fatalf("admin membuat kitchen: %d", res.StatusCode)
	}
	change := map[string]any{"name": "Diubah", "role": "cashier", "status": "active", "store_ids": []string{owner.storeID.String()}}
	for _, id := range []string{ownerID, admin2ID} {
		if res, _ := call(t, "PATCH", srv.URL+"/v1/staff/"+id, change, adminToken); res.StatusCode != http.StatusForbidden {
			t.Fatalf("admin mengubah %s: %d", id, res.StatusCode)
		}
		if res, _ := call(t, "POST", srv.URL+"/v1/staff/"+id+"/password", map[string]any{"password": "password-baru-1"}, adminToken); res.StatusCode != http.StatusForbidden {
			t.Fatalf("admin reset password %s: %d", id, res.StatusCode)
		}
		if res, _ := call(t, "PUT", srv.URL+"/v1/staff/"+id+"/pin", map[string]any{"pin": "135790"}, adminToken); res.StatusCode != http.StatusForbidden {
			t.Fatalf("admin set PIN %s: %d", id, res.StatusCode)
		}
	}
	if res, _ := call(t, "PATCH", srv.URL+"/v1/staff/"+ownerID, change, owner.token); res.StatusCode != http.StatusForbidden {
		t.Fatalf("owner tidak dapat diubah lewat API staf: %d", res.StatusCode)
	}
	promote := map[string]any{"name": "Kasir", "role": "admin", "status": "active", "store_ids": []string{owner.storeID.String()}}
	if res, out := call(t, "PATCH", srv.URL+"/v1/staff/"+cashierID, promote, adminToken); res.StatusCode != http.StatusForbidden || out["code"] != "ROLE_NOT_ALLOWED" {
		t.Fatalf("admin menaikkan kasir: %d %v", res.StatusCode, out)
	}

	if res, out := call(t, "PATCH", srv.URL+"/v1/staff/"+cashierID, change, adminToken); res.StatusCode != http.StatusOK || out["name"] != "Diubah" {
		t.Fatalf("admin mengubah kasir: %d %v", res.StatusCode, out)
	}
	wide := map[string]any{"name": "Diubah", "role": "cashier", "status": "active", "store_ids": []string{owner.storeID.String(), secondStore.String()}}
	if res, out := call(t, "PATCH", srv.URL+"/v1/staff/"+cashierID, wide, adminToken); res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_STORE" {
		t.Fatalf("admin memberi outlet di luar miliknya: %d %v", res.StatusCode, out)
	}
	res, out := call(t, "PATCH", srv.URL+"/v1/staff/"+cashierID, wide, owner.token)
	if res.StatusCode != http.StatusOK || len(out["store_ids"].([]any)) != 2 {
		t.Fatalf("owner memberi dua outlet: %d %v", res.StatusCode, out)
	}
	narrow := map[string]any{"name": "Diubah", "role": "kitchen", "status": "active", "store_ids": []string{secondStore.String()}}
	res, out = call(t, "PATCH", srv.URL+"/v1/staff/"+cashierID, narrow, owner.token)
	if ids := out["store_ids"].([]any); res.StatusCode != http.StatusOK || out["role"] != "kitchen" || len(ids) != 1 || ids[0] != secondStore.String() {
		t.Fatalf("outlet dilepas dan role diganti: %d %v", res.StatusCode, out)
	}
}

func TestStaffDisableRevokesSessionsAndReactivate(t *testing.T) {
	srv, pool := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	res, created := createStaffAs(t, srv.URL, owner.token, staffBody("Kasir", em("kasir"), "cashier", owner.storeID))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("seed: %d %v", res.StatusCode, created)
	}
	id := created["id"].(string)
	if status, _ := loginAs(t, srv.URL, em("kasir"), staffPassword); status != http.StatusOK {
		t.Fatalf("login awal: %d", status)
	}
	if activeRefreshTokens(t, pool, owner.tenantID, em("kasir")) == 0 {
		t.Fatal("login harus membuat refresh token aktif")
	}

	body := map[string]any{"name": "Kasir", "role": "cashier", "status": "disabled", "store_ids": []string{owner.storeID.String()}}
	if res, out := call(t, "PATCH", srv.URL+"/v1/staff/"+id, body, owner.token); res.StatusCode != http.StatusOK || out["status"] != "disabled" {
		t.Fatalf("nonaktifkan: %d %v", res.StatusCode, out)
	}
	if n := activeRefreshTokens(t, pool, owner.tenantID, em("kasir")); n != 0 {
		t.Fatalf("semua sesi harus dicabut, tersisa %d", n)
	}
	if status, token := loginAs(t, srv.URL, em("kasir"), staffPassword); status == http.StatusOK || token != "" {
		t.Fatalf("akun nonaktif tidak boleh login: %d", status)
	}

	body["status"] = "active"
	if res, out := call(t, "PATCH", srv.URL+"/v1/staff/"+id, body, owner.token); res.StatusCode != http.StatusOK || out["status"] != "active" {
		t.Fatalf("aktifkan kembali: %d %v", res.StatusCode, out)
	}
	if status, _ := loginAs(t, srv.URL, em("kasir"), staffPassword); status != http.StatusOK {
		t.Fatalf("login setelah diaktifkan: %d", status)
	}
}

func TestStaffPasswordReset(t *testing.T) {
	srv, pool := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)
	_, created := createStaffAs(t, srv.URL, owner.token, staffBody("Kasir", em("kasir"), "cashier", owner.storeID))
	id := created["id"].(string)
	if status, _ := loginAs(t, srv.URL, em("kasir"), staffPassword); status != http.StatusOK {
		t.Fatalf("login awal: %d", status)
	}

	if res, out := call(t, "POST", srv.URL+"/v1/staff/"+id+"/password", map[string]any{"password": "pendek"}, owner.token); res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("password pendek: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "POST", srv.URL+"/v1/staff/"+id+"/password", map[string]any{"password": "password-baru-123"}, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("reset: %d", res.StatusCode)
	}
	if n := activeRefreshTokens(t, pool, owner.tenantID, em("kasir")); n != 0 {
		t.Fatalf("sesi lama harus dicabut, tersisa %d", n)
	}
	if status, _ := loginAs(t, srv.URL, em("kasir"), staffPassword); status != http.StatusUnauthorized {
		t.Fatalf("password lama harus gagal: %d", status)
	}
	if status, _ := loginAs(t, srv.URL, em("kasir"), "password-baru-123"); status != http.StatusOK {
		t.Fatalf("password baru harus berhasil: %d", status)
	}
}

func TestStaffPlanLimitCountsOnlyActiveUsers(t *testing.T) {
	srv, _ := setup(t)
	em := mails()
	owner := registerTenant(t, srv.URL)

	ids := make([]string, 0, 4)
	for i := 1; i <= 4; i++ {
		res, out := createStaffAs(t, srv.URL, owner.token, staffBody(fmt.Sprintf("Kasir %d", i), em(fmt.Sprintf("kasir%d", i)), "cashier", owner.storeID))
		if res.StatusCode != http.StatusCreated {
			t.Fatalf("kasir %d: %d %v", i, res.StatusCode, out)
		}
		ids = append(ids, out["id"].(string))
	}
	res, out := createStaffAs(t, srv.URL, owner.token, staffBody("Lebih", em("lebih"), "cashier", owner.storeID))
	if res.StatusCode != http.StatusConflict || out["code"] != "PLAN_LIMIT_REACHED" {
		t.Fatalf("batas paket trial 5 pengguna: %d %v", res.StatusCode, out)
	}

	patch := func(id, status string) (*http.Response, map[string]any) {
		return call(t, "PATCH", srv.URL+"/v1/staff/"+id, map[string]any{"name": "K", "role": "cashier", "status": status, "store_ids": []string{owner.storeID.String()}}, owner.token)
	}
	if res, _ := patch(ids[0], "disabled"); res.StatusCode != http.StatusOK {
		t.Fatalf("nonaktifkan: %d", res.StatusCode)
	}
	if res, out := createStaffAs(t, srv.URL, owner.token, staffBody("Pengganti", em("pengganti"), "cashier", owner.storeID)); res.StatusCode != http.StatusCreated {
		t.Fatalf("pengguna nonaktif tidak dihitung: %d %v", res.StatusCode, out)
	}
	if res, out := patch(ids[0], "active"); res.StatusCode != http.StatusConflict || out["code"] != "PLAN_LIMIT_REACHED" {
		t.Fatalf("mengaktifkan kembali melewati batas: %d %v", res.StatusCode, out)
	}
}

func TestStaffTenantIsolation(t *testing.T) {
	srv, _ := setup(t)
	em := mails()
	a := registerTenant(t, srv.URL)
	b := registerTenant(t, srv.URL)
	_, created := createStaffAs(t, srv.URL, a.token, staffBody("Kasir A", em("kasir.a"), "cashier", a.storeID))
	id := created["id"].(string)

	body := map[string]any{"name": "Curang", "role": "cashier", "status": "disabled", "store_ids": []string{b.storeID.String()}}
	if res, _ := call(t, "PATCH", srv.URL+"/v1/staff/"+id, body, b.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("patch lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "POST", srv.URL+"/v1/staff/"+id+"/password", map[string]any{"password": "password-baru-123"}, b.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("reset lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/staff/"+id+"/pin", nil, b.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("hapus PIN lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "PATCH", srv.URL+"/v1/staff/bukan-uuid", body, b.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("id non-uuid: %d", res.StatusCode)
	}
	if n := len(staffList(t, srv.URL, b.token)["items"].([]any)); n != 1 {
		t.Fatalf("tenant B hanya melihat dirinya: %d", n)
	}
	if m := memberByEmail(t, staffList(t, srv.URL, a.token), em("kasir.a")); m["status"] != "active" {
		t.Fatalf("data tenant A tidak boleh berubah: %v", m)
	}
}

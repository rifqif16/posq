package app_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/app"
	"github.com/rifqif16/posq/api/internal/auth/infrastructure"
	"github.com/rifqif16/posq/api/internal/platform/config"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

// Integration test memakai PostgreSQL nyata (role posq_app, RLS aktif).
// Jalankan: TEST_DATABASE_URL=<url posq_app> go test ./internal/app/...
func setup(t *testing.T) (*httptest.Server, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set; integration test dilewati")
	}
	ctx := context.Background()
	pool, err := database.NewPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	_, priv, _ := ed25519.GenerateKey(nil)
	cfg := config.Config{
		JWTPrivateKey: priv, AccessTTL: 15 * time.Minute, RefreshTTL: time.Hour, TrialDays: 14,
	}
	fast := infrastructure.NewArgon2Hasher(infrastructure.Argon2Params{MemoryKiB: 8, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32})
	h, err := app.NewRouter(app.Deps{Pool: pool, Config: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Hasher: fast})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, pool
}

func call(t *testing.T, method, url string, body any, bearer string, cookies ...*http.Cookie) (*http.Response, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res, out
}

func refreshCookie(res *http.Response) *http.Cookie {
	for _, c := range res.Cookies() {
		if c.Name == "posq_rt" {
			return c
		}
	}
	return nil
}

func regBody(email string) map[string]string {
	return map[string]string{"business_name": "Kedai Test", "owner_name": "Sari", "email": email, "password": "password-aman-1"}
}

func TestAuthFlowEndToEnd(t *testing.T) {
	srv, _ := setup(t)
	email := "e2e-" + uuid.NewString() + "@kedai.id"

	// register
	res, out := call(t, "POST", srv.URL+"/v1/auth/register", regBody(email), "")
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %v", res.StatusCode, out)
	}
	access := out["access_token"].(string)
	rt1 := refreshCookie(res)
	if rt1 == nil || !rt1.HttpOnly || rt1.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie refresh tidak aman: %+v", rt1)
	}
	tenant := out["tenant"].(map[string]any)
	if tenant["status"] != "trial" || tenant["trial_ends_at"] == nil {
		t.Fatalf("tenant harus trial: %v", tenant)
	}

	// duplicate email (case-insensitive)
	res, out = call(t, "POST", srv.URL+"/v1/auth/register", regBody(email), "")
	if res.StatusCode != http.StatusConflict || out["code"] != "EMAIL_TAKEN" {
		t.Fatalf("duplikat: %d %v", res.StatusCode, out)
	}

	// validation
	res, out = call(t, "POST", srv.URL+"/v1/auth/register", map[string]string{"email": "x"}, "")
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "VALIDATION_FAILED" {
		t.Fatalf("validasi: %d %v", res.StatusCode, out)
	}

	// /me
	res, out = call(t, "GET", srv.URL+"/v1/me", nil, access)
	if res.StatusCode != http.StatusOK || out["user"].(map[string]any)["role"] != "owner" {
		t.Fatalf("me: %d %v", res.StatusCode, out)
	}
	if res, _ = call(t, "GET", srv.URL+"/v1/me", nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("me tanpa token harus 401, dapat %d", res.StatusCode)
	}

	// login: salah vs benar
	res, out = call(t, "POST", srv.URL+"/v1/auth/login", map[string]string{"email": email, "password": "salah-salah-salah"}, "")
	if res.StatusCode != http.StatusUnauthorized || out["code"] != "INVALID_CREDENTIALS" {
		t.Fatalf("login salah: %d %v", res.StatusCode, out)
	}
	res, out = call(t, "POST", srv.URL+"/v1/auth/login", map[string]string{"email": email, "password": "password-aman-1"}, "")
	if res.StatusCode != http.StatusOK || out["access_token"] == nil {
		t.Fatalf("login benar: %d %v", res.StatusCode, out)
	}
	rt2 := refreshCookie(res)

	// refresh: rotasi
	res, out = call(t, "POST", srv.URL+"/v1/auth/refresh", nil, "", rt2)
	if res.StatusCode != http.StatusOK || out["access_token"] == nil {
		t.Fatalf("refresh: %d %v", res.StatusCode, out)
	}
	rt3 := refreshCookie(res)
	if rt3.Value == rt2.Value {
		t.Fatal("refresh token harus dirotasi")
	}

	// reuse token lama -> seluruh keluarga dicabut, termasuk rt3
	res, out = call(t, "POST", srv.URL+"/v1/auth/refresh", nil, "", rt2)
	if res.StatusCode != http.StatusUnauthorized || out["code"] != "INVALID_REFRESH_TOKEN" {
		t.Fatalf("reuse harus 401: %d %v", res.StatusCode, out)
	}
	if res, _ = call(t, "POST", srv.URL+"/v1/auth/refresh", nil, "", rt3); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("keluarga harus tercabut, dapat %d", res.StatusCode)
	}

	// logout mencabut sesi register (rt1)
	res, _ = call(t, "POST", srv.URL+"/v1/auth/logout", nil, "", rt1)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("logout: %d", res.StatusCode)
	}
	if res, _ = call(t, "POST", srv.URL+"/v1/auth/refresh", nil, "", rt1); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("setelah logout refresh harus 401, dapat %d", res.StatusCode)
	}
}

func TestRowLevelSecurityIsolatesTenants(t *testing.T) {
	srv, pool := setup(t)

	var tenants [2]uuid.UUID
	for i := range tenants {
		_, out := call(t, "POST", srv.URL+"/v1/auth/register", regBody("rls-"+uuid.NewString()+"@kedai.id"), "")
		tenants[i] = uuid.MustParse(out["tenant"].(map[string]any)["id"].(string))
	}
	ctx := context.Background()

	// Tanpa app.tenant_id, role aplikasi tidak melihat apa pun.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("tanpa tenant harus 0 baris, dapat %d err=%v", n, err)
	}

	// Dengan tenant A, hanya baris tenant A yang terlihat.
	err := database.WithTenantTx(ctx, pool, tenants[0], func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	})
	if err != nil || n != 1 {
		t.Fatalf("tenant A harus melihat 1 user, dapat %d err=%v", n, err)
	}

	// Menyisipkan baris untuk tenant lain harus ditolak policy WITH CHECK.
	err = database.WithTenantTx(ctx, pool, tenants[0], func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO stores (id, tenant_id, code, name) VALUES ($1,$2,'X','X')`, uuid.New(), tenants[1])
		return err
	})
	if err == nil {
		t.Fatal("insert lintas tenant harus ditolak RLS")
	}
}

package app_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/auth/infrastructure"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

type tenantSession struct {
	token    string
	tenantID uuid.UUID
	storeID  uuid.UUID
	email    string
}

func registerTenant(t *testing.T, base string) tenantSession {
	t.Helper()
	email := "cat-" + uuid.NewString() + "@kedai.id"
	res, out := call(t, "POST", base+"/v1/auth/register", regBody(email), "")
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register: %d %v", res.StatusCode, out)
	}
	return tenantSession{
		token:    out["access_token"].(string),
		tenantID: uuid.MustParse(out["tenant"].(map[string]any)["id"].(string)),
		storeID:  uuid.MustParse(out["store_ids"].([]any)[0].(string)),
		email:    email,
	}
}

// insertCashier membuat kasir langsung di DB (manajemen user belum ada) lalu login lewat API.
func insertCashier(t *testing.T, base string, pool *pgxpool.Pool, owner tenantSession) string {
	t.Helper()
	hasher := infrastructure.NewArgon2Hasher(infrastructure.Argon2Params{MemoryKiB: 8, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32})
	hash, err := hasher.Hash("password-aman-1")
	if err != nil {
		t.Fatal(err)
	}
	email := "kasir-" + uuid.NewString() + "@kedai.id"
	userID := uuid.New()
	err = database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(),
			`INSERT INTO users (id, tenant_id, email, password_hash, name) VALUES ($1,$2,$3,$4,'Kasir')`,
			userID, owner.tenantID, email, hash); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(),
			`INSERT INTO user_store_roles (user_id, store_id, tenant_id, role) VALUES ($1,$2,$3,'cashier')`,
			userID, owner.storeID, owner.tenantID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	res, out := call(t, "POST", base+"/v1/auth/login", map[string]string{"email": email, "password": "password-aman-1"}, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login kasir: %d %v", res.StatusCode, out)
	}
	return out["access_token"].(string)
}

func createCategory(t *testing.T, base, token string, body map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/categories", body, token)
}

func TestCategoriesCRUDAndHierarchy(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	res, parent := createCategory(t, srv.URL, owner.token, map[string]any{"name": "Minuman"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create induk: %d %v", res.StatusCode, parent)
	}
	parentID := parent["id"].(string)

	res, child := createCategory(t, srv.URL, owner.token, map[string]any{"name": "Kopi", "parent_id": parentID})
	if res.StatusCode != http.StatusCreated || child["parent_id"] != parentID {
		t.Fatalf("create anak: %d %v", res.StatusCode, child)
	}

	// level ke-3 ditolak
	res, out := createCategory(t, srv.URL, owner.token, map[string]any{"name": "Espresso", "parent_id": child["id"]})
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_PARENT" {
		t.Fatalf("level 3: %d %v", res.StatusCode, out)
	}
	// nama duplikat (case-insensitive) pada induk yang sama
	res, out = createCategory(t, srv.URL, owner.token, map[string]any{"name": "minuman"})
	if res.StatusCode != http.StatusConflict || out["code"] != "CATEGORY_NAME_TAKEN" {
		t.Fatalf("duplikat: %d %v", res.StatusCode, out)
	}
	// nama sama di induk berbeda diizinkan
	if res, out = createCategory(t, srv.URL, owner.token, map[string]any{"name": "Kopi"}); res.StatusCode != http.StatusCreated {
		t.Fatalf("nama sama beda induk: %d %v", res.StatusCode, out)
	}
	// validasi
	res, out = createCategory(t, srv.URL, owner.token, map[string]any{"name": "  "})
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "VALIDATION_FAILED" {
		t.Fatalf("validasi: %d %v", res.StatusCode, out)
	}

	res, list := call(t, "GET", srv.URL+"/v1/categories", nil, owner.token)
	if res.StatusCode != http.StatusOK || len(list["items"].([]any)) != 3 {
		t.Fatalf("list: %d %v", res.StatusCode, list)
	}

	// rename
	res, out = call(t, "PATCH", srv.URL+"/v1/categories/"+parentID, map[string]any{"name": "Minuman Dingin", "sort_order": 2}, owner.token)
	if res.StatusCode != http.StatusOK || out["name"] != "Minuman Dingin" {
		t.Fatalf("rename: %d %v", res.StatusCode, out)
	}

	// induk yang punya anak tidak bisa dihapus; anak dulu, lalu induk
	res, out = call(t, "DELETE", srv.URL+"/v1/categories/"+parentID, nil, owner.token)
	if res.StatusCode != http.StatusConflict || out["code"] != "CATEGORY_IN_USE" {
		t.Fatalf("hapus induk: %d %v", res.StatusCode, out)
	}
	if res, _ = call(t, "DELETE", srv.URL+"/v1/categories/"+child["id"].(string), nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("hapus anak: %d", res.StatusCode)
	}
	if res, _ = call(t, "DELETE", srv.URL+"/v1/categories/"+parentID, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("hapus induk setelah anak: %d", res.StatusCode)
	}
	// sudah terhapus: 404, dan induk terhapus tidak bisa dipakai sebagai induk
	if res, _ = call(t, "PATCH", srv.URL+"/v1/categories/"+parentID, map[string]any{"name": "X"}, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("patch terhapus: %d", res.StatusCode)
	}
	res, out = createCategory(t, srv.URL, owner.token, map[string]any{"name": "Baru", "parent_id": parentID})
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_PARENT" {
		t.Fatalf("induk terhapus: %d %v", res.StatusCode, out)
	}
	// nama bekas kategori terhapus boleh dipakai lagi
	if res, out = createCategory(t, srv.URL, owner.token, map[string]any{"name": "Minuman Dingin"}); res.StatusCode != http.StatusCreated {
		t.Fatalf("reuse nama: %d %v", res.StatusCode, out)
	}
	if res, _ = call(t, "PATCH", srv.URL+"/v1/categories/bukan-uuid", map[string]any{"name": "X"}, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("id non-uuid: %d", res.StatusCode)
	}
}

func TestCategoriesAuthorization(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)

	if res, _ := call(t, "GET", srv.URL+"/v1/categories", nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/categories", nil, cashier); res.StatusCode != http.StatusOK {
		t.Fatalf("kasir boleh baca: %d", res.StatusCode)
	}
	res, out := createCategory(t, srv.URL, cashier, map[string]any{"name": "Terlarang"})
	if res.StatusCode != http.StatusForbidden || out["code"] != "FORBIDDEN" {
		t.Fatalf("kasir tulis: %d %v", res.StatusCode, out)
	}
}

func TestCategoriesTenantIsolation(t *testing.T) {
	srv, _ := setup(t)
	a := registerTenant(t, srv.URL)
	b := registerTenant(t, srv.URL)

	_, cat := createCategory(t, srv.URL, a.token, map[string]any{"name": "Rahasia A"})
	id := cat["id"].(string)

	_, list := call(t, "GET", srv.URL+"/v1/categories", nil, b.token)
	if n := len(list["items"].([]any)); n != 0 {
		t.Fatalf("tenant B melihat %d kategori milik A", n)
	}
	if res, _ := call(t, "PATCH", srv.URL+"/v1/categories/"+id, map[string]any{"name": "Dibajak"}, b.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("patch lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/categories/"+id, nil, b.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("delete lintas tenant: %d", res.StatusCode)
	}
	res, out := createCategory(t, srv.URL, b.token, map[string]any{"name": "Anak", "parent_id": id})
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_PARENT" {
		t.Fatalf("induk lintas tenant: %d %v", res.StatusCode, out)
	}
}

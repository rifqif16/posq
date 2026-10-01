package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/platform/database"
)

// callH seperti call, tetapi mendukung header tambahan (mis. If-Match).
func callH(t *testing.T, method, url string, body any, bearer string, headers map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	req.Header.Set("Authorization", "Bearer "+bearer)
	for k, v := range headers {
		req.Header.Set(k, v)
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

func productBody(name, sku string, barcodes []string, cost, sell int, extra map[string]any) map[string]any {
	b := map[string]any{
		"name": name,
		"variants": []map[string]any{{
			"sku": sku, "barcodes": barcodes, "cost_price": cost, "sell_price": sell,
		}},
	}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func createProduct(t *testing.T, base, token string, body map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/products", body, token)
}

func firstVariant(p map[string]any) map[string]any {
	return p["variants"].([]any)[0].(map[string]any)
}

func TestProductsCreateValidateAndUniqueness(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	res, p := createProduct(t, srv.URL, owner.token, productBody("Es Kopi Susu", "KOPI-01", []string{"8999999000011"}, 8000, 15000, map[string]any{"kitchen_station": "bar"}))
	if res.StatusCode != http.StatusCreated || res.Header.Get("ETag") != `"1"` {
		t.Fatalf("create: %d etag=%q %v", res.StatusCode, res.Header.Get("ETag"), p)
	}
	v := firstVariant(p)
	if p["version"] != float64(1) || p["taxable"] != true || p["is_active"] != true || p["kitchen_station"] != "bar" ||
		v["sku"] != "KOPI-01" || v["cost_price"] != float64(8000) || v["is_default"] != true || v["name"] != "Default" {
		t.Fatalf("respons tidak sesuai: %v", p)
	}

	res, out := createProduct(t, srv.URL, owner.token, productBody("Lain", "kopi-01", nil, 0, 1000, nil))
	if res.StatusCode != http.StatusConflict || out["code"] != "SKU_TAKEN" {
		t.Fatalf("sku duplikat (case-insensitive): %d %v", res.StatusCode, out)
	}
	res, out = createProduct(t, srv.URL, owner.token, productBody("Lain", "", []string{"8999999000011"}, 0, 1000, nil))
	if res.StatusCode != http.StatusConflict || out["code"] != "BARCODE_TAKEN" {
		t.Fatalf("barcode duplikat: %d %v", res.StatusCode, out)
	}
	// gagal di tengah transaksi tidak meninggalkan produk setengah jadi
	_, list := call(t, "GET", srv.URL+"/v1/products", nil, owner.token)
	if n := len(list["items"].([]any)); n != 1 {
		t.Fatalf("harus 1 produk setelah dua gagal, dapat %d", n)
	}

	res, out = createProduct(t, srv.URL, owner.token, productBody("Tanpa SKU", "", nil, 0, 5000, nil))
	if res.StatusCode != http.StatusCreated || !regexp.MustCompile(`^SKU-[0-9A-F]{8}$`).MatchString(firstVariant(out)["sku"].(string)) {
		t.Fatalf("sku otomatis: %d %v", res.StatusCode, out)
	}

	for name, body := range map[string]map[string]any{
		"harga negatif": productBody("X", "", nil, 0, -1, nil),
		"nama kosong":   productBody(" ", "", nil, 0, 1, nil),
		"dua varian":    {"name": "X", "variants": []map[string]any{{"sell_price": 1}, {"sell_price": 2}}},
		"tanpa varian":  {"name": "X"},
	} {
		res, out = createProduct(t, srv.URL, owner.token, body)
		if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "VALIDATION_FAILED" {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
}

func TestProductsSearchAndPagination(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	for i, n := range []string{"Kopi Susu", "Teh Tarik", "100% Jus", "Air Mineral"} {
		code := []string{"111", "222", "333", "444"}[i]
		if res, out := createProduct(t, srv.URL, owner.token, productBody(n, "SKU"+code, []string{code}, 0, 1000, nil)); res.StatusCode != http.StatusCreated {
			t.Fatalf("seed %s: %d %v", n, res.StatusCode, out)
		}
	}
	names := func(q string) []string {
		_, l := call(t, "GET", srv.URL+"/v1/products?"+q, nil, owner.token)
		var out []string
		for _, it := range l["items"].([]any) {
			out = append(out, it.(map[string]any)["name"].(string))
		}
		return out
	}
	eq := func(got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("got %v want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v want %v", got, want)
			}
		}
	}
	eq(names(""), "100% Jus", "Air Mineral", "Kopi Susu", "Teh Tarik")
	eq(names("q=kopi"), "Kopi Susu")
	eq(names("q=222"), "Teh Tarik")    // barcode persis
	eq(names("q=sku111"), "Kopi Susu") // SKU sebagian, case-insensitive
	eq(names("q=%25"), "100% Jus")     // wildcard LIKE dinetralkan
	eq(names("q=_"))                   // garis bawah literal, tidak ada yang cocok

	// paginasi keyset: 2 + 2, tanpa duplikat
	_, p1 := call(t, "GET", srv.URL+"/v1/products?limit=2", nil, owner.token)
	cursor, _ := p1["next_cursor"].(string)
	if cursor == "" || len(p1["items"].([]any)) != 2 {
		t.Fatalf("halaman 1: %v", p1)
	}
	_, p2 := call(t, "GET", srv.URL+"/v1/products?limit=2&cursor="+cursor, nil, owner.token)
	if p2["next_cursor"] != nil || len(p2["items"].([]any)) != 2 {
		t.Fatalf("halaman 2: %v", p2)
	}
	if p1["items"].([]any)[1].(map[string]any)["id"] == p2["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("item duplikat antar halaman")
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/products?cursor=rusak!", nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("cursor rusak: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/products?category_id=x", nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("category_id rusak: %d", res.StatusCode)
	}
}

func TestProductsUpdateConcurrencyHistoryAndDelete(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)

	_, cat := createCategory(t, srv.URL, owner.token, map[string]any{"name": "Minuman"})
	catID := cat["id"].(string)
	_, p := createProduct(t, srv.URL, owner.token, productBody("Kopi", "KOPI", []string{"111"}, 8000, 15000, map[string]any{"category_id": catID}))
	id := p["id"].(string)
	url := srv.URL + "/v1/products/" + id

	upd := productBody("Kopi Hitam", "", []string{"222"}, 8000, 16000, map[string]any{"category_id": catID})
	// varian lama harus dirujuk lewat id; tanpa id dianggap varian baru
	upd["variants"].([]map[string]any)[0]["id"] = firstVariant(p)["id"]
	if res, out := callH(t, "PATCH", url, upd, owner.token, nil); res.StatusCode != http.StatusPreconditionRequired || out["code"] != "PRECONDITION_REQUIRED" {
		t.Fatalf("tanpa If-Match: %d %v", res.StatusCode, out)
	}
	if res, out := callH(t, "PATCH", url, upd, owner.token, map[string]string{"If-Match": `"9"`}); res.StatusCode != http.StatusPreconditionFailed || out["code"] != "VERSION_CONFLICT" {
		t.Fatalf("versi basi: %d %v", res.StatusCode, out)
	}
	res, out := callH(t, "PATCH", url, upd, owner.token, map[string]string{"If-Match": `"1"`})
	v := firstVariant(out)
	if res.StatusCode != http.StatusOK || out["version"] != float64(2) || out["name"] != "Kopi Hitam" ||
		v["sku"] != "KOPI" || v["sell_price"] != float64(16000) || len(v["barcodes"].([]any)) != 1 || v["barcodes"].([]any)[0] != "222" {
		t.Fatalf("update: %d %v", res.StatusCode, out)
	}
	// pemakaian ulang versi lama setelah update -> konflik
	if res, _ = callH(t, "PATCH", url, upd, owner.token, map[string]string{"If-Match": `"1"`}); res.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("versi lama: %d", res.StatusCode)
	}

	// riwayat harga: hanya sell_price yang berubah
	var fields []string
	err := database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(context.Background(), `SELECT field FROM product_price_history`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f string
			if err := rows.Scan(&f); err != nil {
				return err
			}
			fields = append(fields, f)
		}
		return rows.Err()
	})
	if err != nil || len(fields) != 1 || fields[0] != "sell_price" {
		t.Fatalf("riwayat harga: %v %v", fields, err)
	}

	// barcode lama ("111") bebas dipakai produk lain setelah diganti
	if res, out = createProduct(t, srv.URL, owner.token, productBody("Lain", "", []string{"111"}, 0, 1, nil)); res.StatusCode != http.StatusCreated {
		t.Fatalf("pakai ulang barcode: %d %v", res.StatusCode, out)
	}
	// kategori tidak valid (acak) ditolak
	bad := productBody("X", "", nil, 0, 1, map[string]any{"category_id": uuid.NewString()})
	if res, out = createProduct(t, srv.URL, owner.token, bad); res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_CATEGORY" {
		t.Fatalf("kategori acak: %d %v", res.StatusCode, out)
	}

	// kategori yang dipakai produk tidak bisa dihapus; setelah produk dihapus bisa
	if res, out = call(t, "DELETE", srv.URL+"/v1/categories/"+catID, nil, owner.token); res.StatusCode != http.StatusConflict || out["code"] != "CATEGORY_IN_USE" {
		t.Fatalf("hapus kategori terpakai: %d %v", res.StatusCode, out)
	}
	if res, _ = call(t, "DELETE", url, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete produk: %d", res.StatusCode)
	}
	if res, _ = call(t, "GET", url, nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("get terhapus: %d", res.StatusCode)
	}
	if res, _ = call(t, "DELETE", srv.URL+"/v1/categories/"+catID, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("hapus kategori setelah produk dihapus: %d", res.StatusCode)
	}
	// SKU dan barcode bekas produk terhapus bebas dipakai lagi
	if res, out = createProduct(t, srv.URL, owner.token, productBody("Baru", "KOPI", []string{"222"}, 0, 1, nil)); res.StatusCode != http.StatusCreated {
		t.Fatalf("pakai ulang sku/barcode: %d %v", res.StatusCode, out)
	}
	if res, _ = callH(t, "PATCH", url, upd, owner.token, map[string]string{"If-Match": `"2"`}); res.StatusCode != http.StatusNotFound {
		t.Fatalf("patch terhapus: %d", res.StatusCode)
	}
}

func TestProductsAuthorizationAndIsolation(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)

	_, p := createProduct(t, srv.URL, owner.token, productBody("Rahasia", "RHS", []string{"999"}, 5000, 9000, nil))
	id := p["id"].(string)

	// kasir: boleh baca, harga beli disembunyikan, tidak boleh menulis
	res, got := call(t, "GET", srv.URL+"/v1/products/"+id, nil, cashier)
	if res.StatusCode != http.StatusOK || firstVariant(got)["cost_price"] != nil || firstVariant(got)["sell_price"] != float64(9000) {
		t.Fatalf("kasir baca: %d %v", res.StatusCode, got)
	}
	_, list := call(t, "GET", srv.URL+"/v1/products", nil, cashier)
	if firstVariant(list["items"].([]any)[0].(map[string]any))["cost_price"] != nil {
		t.Fatal("harga beli bocor di list untuk kasir")
	}
	if res, out := createProduct(t, srv.URL, cashier, productBody("X", "", nil, 0, 1, nil)); res.StatusCode != http.StatusForbidden || out["code"] != "FORBIDDEN" {
		t.Fatalf("kasir tulis: %d %v", res.StatusCode, out)
	}
	if res, _ := callH(t, "PATCH", srv.URL+"/v1/products/"+id, productBody("X", "", nil, 0, 1, nil), cashier, map[string]string{"If-Match": `"1"`}); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir patch: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/products/"+id, nil, cashier); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir delete: %d", res.StatusCode)
	}

	// tenant lain tidak melihat/mengubah, dan SKU/barcode unik per tenant
	if res, _ := call(t, "GET", srv.URL+"/v1/products/"+id, nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("get lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/products/"+id, nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("delete lintas tenant: %d", res.StatusCode)
	}
	if res, _ := callH(t, "PATCH", srv.URL+"/v1/products/"+id, productBody("X", "", nil, 0, 1, nil), other.token, map[string]string{"If-Match": `"1"`}); res.StatusCode != http.StatusNotFound {
		t.Fatalf("patch lintas tenant: %d", res.StatusCode)
	}
	_, ol := call(t, "GET", srv.URL+"/v1/products", nil, other.token)
	if n := len(ol["items"].([]any)); n != 0 {
		t.Fatalf("tenant lain melihat %d produk", n)
	}
	if res, out := createProduct(t, srv.URL, other.token, productBody("Sama", "RHS", []string{"999"}, 0, 1, nil)); res.StatusCode != http.StatusCreated {
		t.Fatalf("sku/barcode sama di tenant lain harus boleh: %d %v", res.StatusCode, out)
	}
	bad := productBody("X", "", nil, 0, 1, map[string]any{"category_id": uuid.NewString()})
	if res, _ := createProduct(t, srv.URL, other.token, bad); res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("kategori milik tenant lain: %d", res.StatusCode)
	}
}

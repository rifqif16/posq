package app_test

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type vspec map[string]any

func variantsBody(name string, variants ...vspec) map[string]any {
	vs := make([]map[string]any, len(variants))
	for i, v := range variants {
		vs[i] = v
	}
	return map[string]any{"name": name, "variants": vs}
}

func variantsOf(p map[string]any) []map[string]any {
	var out []map[string]any
	for _, v := range p["variants"].([]any) {
		out = append(out, v.(map[string]any))
	}
	return out
}

func variantByName(t *testing.T, p map[string]any, name string) map[string]any {
	t.Helper()
	for _, v := range variantsOf(p) {
		if v["name"] == name {
			return v
		}
	}
	t.Fatalf("varian %q tidak ada di %v", name, p["variants"])
	return nil
}

func patchProduct(t *testing.T, base, token, id string, version int, body map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return callH(t, "PATCH", base+"/v1/products/"+id, body, token, map[string]string{"If-Match": `"` + string(rune('0'+version)) + `"`})
}

func countHistory(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, tenantID uuid.UUID) int {
	t.Helper()
	var n int
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(context.Background(), "SELECT set_config('app.tenant_id', $1, true)", tenantID.String()); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(context.Background(), `SELECT count(*) FROM product_price_history`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestVariantProductCreateAndValidation(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	res, p := createProduct(t, srv.URL, owner.token, variantsBody("Kopi",
		vspec{"name": "Small", "sku": "K-S", "barcodes": []string{"101"}, "cost_price": 5000, "sell_price": 10000},
		vspec{"name": "Medium", "barcodes": []string{"102"}, "sell_price": 13000},
		vspec{"name": "Large", "sku": "K-L", "sell_price": 15000, "is_active": false},
	))
	if res.StatusCode != http.StatusCreated || p["type"] != "variant" {
		t.Fatalf("create: %d %v", res.StatusCode, p)
	}
	vs := variantsOf(p)
	if len(vs) != 3 || vs[0]["name"] != "Small" || vs[0]["is_default"] != true || vs[1]["is_default"] != false {
		t.Fatalf("varian/urutan default: %v", vs)
	}
	if !regexp.MustCompile(`^SKU-[0-9A-F]{8}$`).MatchString(variantByName(t, p, "Medium")["sku"].(string)) {
		t.Fatalf("sku otomatis varian: %v", vs[1])
	}
	if variantByName(t, p, "Large")["is_active"] != false || variantByName(t, p, "Small")["is_active"] != true {
		t.Fatalf("is_active: %v", vs)
	}

	problems := map[string]struct {
		body  map[string]any
		field string
	}{
		"nama kembar": {variantsBody("X", vspec{"name": "A", "sell_price": 1}, vspec{"name": "a", "sell_price": 1}), "variants[1].name"},
		"nama kosong": {variantsBody("X", vspec{"name": "A", "sell_price": 1}, vspec{"sell_price": 1}), "variants[1].name"},
		"sku kembar":  {variantsBody("X", vspec{"name": "A", "sku": "Z", "sell_price": 1}, vspec{"name": "B", "sku": "z", "sell_price": 1}), "variants[1].sku"},
		"barcode kembar": {variantsBody("X", vspec{"name": "A", "barcodes": []string{"9"}, "sell_price": 1},
			vspec{"name": "B", "barcodes": []string{"9"}, "sell_price": 1}), "variants[1].barcodes"},
		"semua nonaktif": {variantsBody("X", vspec{"name": "A", "sell_price": 1, "is_active": false}, vspec{"name": "B", "sell_price": 1, "is_active": false}), "variants"},
	}
	for name, c := range problems {
		res, out := createProduct(t, srv.URL, owner.token, c.body)
		errs, _ := out["errors"].([]any)
		if res.StatusCode != http.StatusUnprocessableEntity || len(errs) != 1 || errs[0].(map[string]any)["field"] != c.field {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	many := make([]vspec, 21)
	for i := range many {
		many[i] = vspec{"name": "V" + string(rune('A'+i)), "sell_price": 1}
	}
	if res, _ := createProduct(t, srv.URL, owner.token, variantsBody("X", many...)); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("21 varian: %d", res.StatusCode)
	}

	// SKU/barcode bentrok dengan produk lain (lintas produk) -> 409 dan tidak ada produk baru
	res, out := createProduct(t, srv.URL, owner.token, variantsBody("Teh", vspec{"name": "Hot", "sku": "k-s", "sell_price": 1}, vspec{"name": "Ice", "sell_price": 1}))
	if res.StatusCode != http.StatusConflict || out["code"] != "SKU_TAKEN" {
		t.Fatalf("sku lintas produk: %d %v", res.StatusCode, out)
	}
	res, out = createProduct(t, srv.URL, owner.token, variantsBody("Teh", vspec{"name": "Hot", "sell_price": 1}, vspec{"name": "Ice", "barcodes": []string{"102"}, "sell_price": 1}))
	if res.StatusCode != http.StatusConflict || out["code"] != "BARCODE_TAKEN" {
		t.Fatalf("barcode lintas produk: %d %v", res.StatusCode, out)
	}
	_, list := call(t, "GET", srv.URL+"/v1/products", nil, owner.token)
	if n := len(list["items"].([]any)); n != 1 {
		t.Fatalf("harus 1 produk, dapat %d", n)
	}
	// pencarian menemukan produk lewat SKU/barcode varian mana pun
	for _, q := range []string{"K-L", "102"} {
		_, l := call(t, "GET", srv.URL+"/v1/products?q="+q, nil, owner.token)
		if len(l["items"].([]any)) != 1 {
			t.Errorf("q=%s harus menemukan produk", q)
		}
	}
}

func TestVariantProductUpdateSyncsVariants(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)

	_, p := createProduct(t, srv.URL, owner.token, variantsBody("Kopi",
		vspec{"name": "Small", "sku": "K-S", "barcodes": []string{"101"}, "sell_price": 10000},
		vspec{"name": "Medium", "sku": "K-M", "barcodes": []string{"102"}, "sell_price": 13000},
		vspec{"name": "Large", "sku": "K-L", "barcodes": []string{"103"}, "sell_price": 15000},
	))
	id := p["id"].(string)
	s, m := variantByName(t, p, "Small"), variantByName(t, p, "Medium")

	// ubah nama S, ubah harga M, hapus L (tidak dikirim), tambah XL tanpa SKU
	res, out := patchProduct(t, srv.URL, owner.token, id, 1, variantsBody("Kopi",
		vspec{"id": s["id"], "name": "Kecil", "barcodes": []string{"101"}, "sell_price": 10000},
		vspec{"id": m["id"], "name": "Medium", "barcodes": []string{"102"}, "sell_price": 14000},
		vspec{"name": "XL", "barcodes": []string{"104"}, "sell_price": 20000},
	))
	if res.StatusCode != http.StatusOK || out["version"] != float64(2) || out["type"] != "variant" {
		t.Fatalf("update: %d %v", res.StatusCode, out)
	}
	if len(variantsOf(out)) != 3 {
		t.Fatalf("harus 3 varian: %v", out["variants"])
	}
	if kecil := variantByName(t, out, "Kecil"); kecil["id"] != s["id"] || kecil["sku"] != "K-S" {
		t.Fatalf("varian lama harus mempertahankan id dan sku: %v", kecil)
	}
	if !regexp.MustCompile(`^SKU-[0-9A-F]{8}$`).MatchString(variantByName(t, out, "XL")["sku"].(string)) {
		t.Fatal("XL harus mendapat sku otomatis")
	}
	if n := countHistory(t, pool, owner.tenantID); n != 1 {
		t.Fatalf("riwayat harga harus 1 (hanya harga Medium), dapat %d", n)
	}
	// barcode & SKU varian L yang dihapus bebas dipakai lagi
	if res, o := createProduct(t, srv.URL, owner.token, productBody("Lain", "K-L", []string{"103"}, 0, 1, nil)); res.StatusCode != http.StatusCreated {
		t.Fatalf("pakai ulang sku/barcode L: %d %v", res.StatusCode, o)
	}

	// pertukaran nama dan SKU antar varian dalam satu update tidak boleh menabrak indeks unik
	k, md := variantByName(t, out, "Kecil"), variantByName(t, out, "Medium")
	xl := variantByName(t, out, "XL")
	res, out = patchProduct(t, srv.URL, owner.token, id, 2, variantsBody("Kopi",
		vspec{"id": k["id"], "name": "Medium", "sku": "K-M", "sell_price": 10000},
		vspec{"id": md["id"], "name": "Kecil", "sku": "K-S", "sell_price": 14000},
		vspec{"id": xl["id"], "name": "XL", "sell_price": 20000},
	))
	if res.StatusCode != http.StatusOK || variantByName(t, out, "Medium")["id"] != k["id"] || variantByName(t, out, "Medium")["sku"] != "K-M" {
		t.Fatalf("swap: %d %v", res.StatusCode, out)
	}

	// jadikan XL default dengan menaruhnya di urutan pertama
	res, out = patchProduct(t, srv.URL, owner.token, id, 3, variantsBody("Kopi",
		vspec{"id": xl["id"], "name": "XL", "sell_price": 20000},
		vspec{"id": k["id"], "name": "Medium", "sell_price": 10000},
		vspec{"id": md["id"], "name": "Kecil", "sell_price": 14000},
	))
	vs := variantsOf(out)
	if res.StatusCode != http.StatusOK || vs[0]["name"] != "XL" || vs[0]["is_default"] != true || vs[1]["is_default"] != false {
		t.Fatalf("default berpindah: %d %v", res.StatusCode, vs)
	}

	// id tak dikenal / milik produk lain ditolak
	_, other := createProduct(t, srv.URL, owner.token, productBody("Teh", "TEH", nil, 0, 1, nil))
	for name, vid := range map[string]any{"acak": uuid.NewString(), "produk lain": firstVariant(other)["id"]} {
		res, o := patchProduct(t, srv.URL, owner.token, id, 4, variantsBody("Kopi",
			vspec{"id": vid, "name": "A", "sell_price": 1}, vspec{"name": "B", "sell_price": 1}))
		errs, _ := o["errors"].([]any)
		if res.StatusCode != http.StatusUnprocessableEntity || o["code"] != "INVALID_VARIANT" || len(errs) != 1 || errs[0].(map[string]any)["field"] != "variants[0].id" {
			t.Errorf("id %s: %d %v", name, res.StatusCode, o)
		}
	}
}

func TestVariantProductConvertsBetweenSimpleAndVariant(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	_, p := createProduct(t, srv.URL, owner.token, productBody("Roti", "ROTI", []string{"555"}, 1000, 3000, nil))
	id, v0 := p["id"].(string), firstVariant(p)
	if p["type"] != "simple" {
		t.Fatalf("awal: %v", p["type"])
	}

	res, out := patchProduct(t, srv.URL, owner.token, id, 1, variantsBody("Roti",
		vspec{"id": v0["id"], "name": "Polos", "barcodes": []string{"555"}, "sell_price": 3000},
		vspec{"name": "Coklat", "sell_price": 4000},
	))
	if res.StatusCode != http.StatusOK || out["type"] != "variant" || len(variantsOf(out)) != 2 {
		t.Fatalf("simple -> variant: %d %v", res.StatusCode, out)
	}
	polos := variantByName(t, out, "Polos")
	if polos["id"] != v0["id"] || polos["sku"] != "ROTI" {
		t.Fatalf("varian asli dipertahankan: %v", polos)
	}

	res, out = patchProduct(t, srv.URL, owner.token, id, 2, variantsBody("Roti",
		vspec{"id": v0["id"], "barcodes": []string{"555"}, "sell_price": 3500},
	))
	if res.StatusCode != http.StatusOK || out["type"] != "simple" || len(variantsOf(out)) != 1 || firstVariant(out)["name"] != "Default" {
		t.Fatalf("variant -> simple: %d %v", res.StatusCode, out)
	}
}

func TestVariantsHiddenCostForCashier(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)

	createProduct(t, srv.URL, owner.token, variantsBody("Kopi",
		vspec{"name": "S", "cost_price": 5000, "sell_price": 10000}, vspec{"name": "L", "cost_price": 7000, "sell_price": 15000}))
	_, list := call(t, "GET", srv.URL+"/v1/products", nil, cashier)
	for _, v := range variantsOf(list["items"].([]any)[0].(map[string]any)) {
		if v["cost_price"] != nil || v["sell_price"] == nil {
			t.Fatalf("harga beli bocor / harga jual hilang: %v", v)
		}
	}
	_, ownerList := call(t, "GET", srv.URL+"/v1/products", nil, owner.token)
	if variantsOf(ownerList["items"].([]any)[0].(map[string]any))[0]["cost_price"] == nil {
		t.Fatal("owner harus melihat harga beli")
	}
}

package app_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func historyItems(out map[string]any) []map[string]any {
	var items []map[string]any
	for _, v := range out["items"].([]any) {
		items = append(items, v.(map[string]any))
	}
	return items
}

func historyURL(base, id, query string) string {
	url := base + "/v1/products/" + id + "/price-history"
	if query != "" {
		url += "?" + query
	}
	return url
}

func TestPriceHistoryListsChangesNewestFirst(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	_, p := createProduct(t, srv.URL, owner.token, productBody("Kopi", "KOPI", nil, 8000, 15000, nil))
	id := p["id"].(string)

	res, out := call(t, "GET", historyURL(srv.URL, id, ""), nil, owner.token)
	if res.StatusCode != http.StatusOK || out["next_cursor"] != nil {
		t.Fatalf("awal: %d %v", res.StatusCode, out)
	}
	if items, ok := out["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("produk tanpa perubahan harus items [] bukan null: %v", out["items"])
	}

	upd := func(version int, cost, sell int) {
		t.Helper()
		body := productBody("Kopi", "", nil, cost, sell, nil)
		body["variants"].([]map[string]any)[0]["id"] = firstVariant(p)["id"]
		if res, o := patchProduct(t, srv.URL, owner.token, id, version, body); res.StatusCode != http.StatusOK {
			t.Fatalf("update v%d: %d %v", version, res.StatusCode, o)
		}
	}
	upd(1, 9000, 16000)
	time.Sleep(5 * time.Millisecond)
	upd(2, 9000, 17000)

	_, out = call(t, "GET", historyURL(srv.URL, id, ""), nil, owner.token)
	items := historyItems(out)
	if len(items) != 3 {
		t.Fatalf("harus 3 perubahan (cost, sell, sell): %v", items)
	}
	first := items[0]
	if first["field"] != "sell_price" || first["old_value"] != float64(16000) || first["new_value"] != float64(17000) {
		t.Fatalf("terbaru harus 16000 -> 17000: %v", first)
	}
	if by := first["changed_by"].(map[string]any); by["name"] != "Sari" || by["id"] == "" {
		t.Fatalf("pelaku: %v", by)
	}
	if first["variant_name"] != "Default" || first["variant_id"] != firstVariant(p)["id"] {
		t.Fatalf("varian: %v", first)
	}
	var prev time.Time
	for i, it := range items {
		at, err := time.Parse(time.RFC3339Nano, it["at"].(string))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && at.After(prev) {
			t.Fatalf("urutan harus menurun: %v setelah %v", at, prev)
		}
		prev = at
	}
	fields := map[string]bool{}
	for _, it := range items[1:] {
		fields[fmt.Sprintf("%s:%v>%v", it["field"], it["old_value"], it["new_value"])] = true
	}
	if !fields["cost_price:8000>9000"] || !fields["sell_price:15000>16000"] {
		t.Fatalf("perubahan awal hilang: %v", fields)
	}
}

func TestPriceHistoryPagination(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	_, p := createProduct(t, srv.URL, owner.token, productBody("Kopi", "KOPI", nil, 0, 1000, nil))
	id := p["id"].(string)

	for i := 1; i <= 5; i++ {
		body := productBody("Kopi", "", nil, 0, 1000+i*100, nil)
		body["variants"].([]map[string]any)[0]["id"] = firstVariant(p)["id"]
		if res, o := patchProduct(t, srv.URL, owner.token, id, i, body); res.StatusCode != http.StatusOK {
			t.Fatalf("update %d: %d %v", i, res.StatusCode, o)
		}
		time.Sleep(2 * time.Millisecond)
	}

	seen := map[string]bool{}
	var newValues []float64
	cursor := ""
	for range 10 {
		query := "limit=2"
		if cursor != "" {
			query += "&cursor=" + cursor
		}
		_, out := call(t, "GET", historyURL(srv.URL, id, query), nil, owner.token)
		for _, it := range historyItems(out) {
			if seen[it["id"].(string)] {
				t.Fatalf("item duplikat antar halaman: %v", it["id"])
			}
			seen[it["id"].(string)] = true
			newValues = append(newValues, it["new_value"].(float64))
		}
		next, _ := out["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	want := []float64{1500, 1400, 1300, 1200, 1100}
	if len(newValues) != len(want) {
		t.Fatalf("harus %d baris, dapat %v", len(want), newValues)
	}
	for i := range want {
		if newValues[i] != want[i] {
			t.Fatalf("urutan lintas halaman salah: %v", newValues)
		}
	}
	if res, _ := call(t, "GET", historyURL(srv.URL, id, "cursor=rusak!"), nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("cursor rusak: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", historyURL(srv.URL, id, "limit=abc"), nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("limit rusak: %d", res.StatusCode)
	}
}

func TestPriceHistoryKeepsRemovedVariantsAndNames(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	_, p := createProduct(t, srv.URL, owner.token, variantsBody("Kopi",
		vspec{"name": "Small", "sku": "K-S", "sell_price": 10000},
		vspec{"name": "Large", "sku": "K-L", "sell_price": 15000},
	))
	id := p["id"].(string)
	s, l := variantByName(t, p, "Small"), variantByName(t, p, "Large")

	res, out := patchProduct(t, srv.URL, owner.token, id, 1, variantsBody("Kopi",
		vspec{"id": s["id"], "name": "Small", "sell_price": 10000},
		vspec{"id": l["id"], "name": "Large", "sell_price": 18000},
	))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("naikkan harga L: %d %v", res.StatusCode, out)
	}
	res, out = patchProduct(t, srv.URL, owner.token, id, 2, variantsBody("Kopi",
		vspec{"id": s["id"], "name": "Kecil", "sell_price": 10000},
	))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("hapus L dan ganti nama S: %d %v", res.StatusCode, out)
	}

	_, hist := call(t, "GET", historyURL(srv.URL, id, ""), nil, owner.token)
	items := historyItems(hist)
	if len(items) != 1 || items[0]["variant_name"] != "Large" || items[0]["new_value"] != float64(18000) {
		t.Fatalf("riwayat varian terhapus harus tetap muncul: %v", items)
	}
}

func TestPriceHistoryAuthorizationAndIsolation(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)

	_, p := createProduct(t, srv.URL, owner.token, productBody("Kopi", "KOPI", nil, 8000, 15000, nil))
	id := p["id"].(string)

	if res, _ := call(t, "GET", historyURL(srv.URL, id, ""), nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}
	if res, out := call(t, "GET", historyURL(srv.URL, id, ""), nil, cashier); res.StatusCode != http.StatusForbidden || out["code"] != "FORBIDDEN" {
		t.Fatalf("kasir tidak boleh melihat riwayat harga beli: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "GET", historyURL(srv.URL, id, ""), nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/products/bukan-uuid/price-history", nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("id non-uuid: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/products/"+id, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("hapus produk: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", historyURL(srv.URL, id, ""), nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("produk terhapus: %d", res.StatusCode)
	}
}

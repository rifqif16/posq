package app_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/platform/database"
)

func trackedProduct(t *testing.T, base string, owner tenantSession, name, sku string) string {
	t.Helper()
	res, p := createProduct(t, base, owner.token, productBody(name, sku, nil, 0, 1000, map[string]any{"track_stock": true}))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("produk %s: %d %v", name, res.StatusCode, p)
	}
	return firstVariant(p)["id"].(string)
}

func postMovement(t *testing.T, base string, owner tenantSession, variant, typ, qty, reason string) (*http.Response, map[string]any) {
	t.Helper()
	body := map[string]any{"store_id": owner.storeID.String(), "variant_id": variant, "type": typ, "qty_delta": qty}
	if reason != "" {
		body["reason"] = reason
	}
	return call(t, "POST", base+"/v1/inventory/movements", body, owner.token)
}

func postCounts(t *testing.T, base string, owner tenantSession, items ...map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/inventory/counts", map[string]any{"store_id": owner.storeID.String(), "items": items}, owner.token)
}

func levelOf(t *testing.T, base string, owner tenantSession, variant string) string {
	t.Helper()
	_, out := call(t, "GET", base+"/v1/inventory/levels?store_id="+owner.storeID.String()+"&limit=100", nil, owner.token)
	for _, it := range out["items"].([]any) {
		m := it.(map[string]any)
		if m["variant_id"] == variant {
			return m["qty_on_hand"].(string)
		}
	}
	t.Fatalf("varian %s tidak ada di daftar saldo", variant)
	return ""
}

func movementsURL(base string, owner tenantSession, query string) string {
	url := base + "/v1/inventory/movements?store_id=" + owner.storeID.String()
	if query != "" {
		url += "&" + query
	}
	return url
}

func TestInventoryMovementsUpdateLevelsAndHistory(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	kopi := trackedProduct(t, srv.URL, owner, "Biji Kopi", "BK")

	if got := levelOf(t, srv.URL, owner, kopi); got != "0.000" {
		t.Fatalf("saldo awal: %s", got)
	}

	body := map[string]any{"store_id": owner.storeID.String(), "variant_id": kopi, "type": "purchase_receive", "qty_delta": "10", "unit_cost": 90000}
	res, out := call(t, "POST", srv.URL+"/v1/inventory/movements", body, owner.token)
	if res.StatusCode != http.StatusCreated || out["qty_on_hand"] != "10.000" {
		t.Fatalf("receive: %d %v", res.StatusCode, out)
	}
	mv := out["movement"].(map[string]any)
	if mv["type"] != "purchase_receive" || mv["qty_delta"] != "10.000" || mv["unit_cost"] != float64(90000) ||
		mv["created_by"].(map[string]any)["name"] != "Sari" || mv["product_name"] != "Biji Kopi" || mv["sku"] != "BK" {
		t.Fatalf("movement: %v", mv)
	}

	if res, out = postMovement(t, srv.URL, owner, kopi, "waste", "-2.5", "tumpah"); res.StatusCode != http.StatusCreated || out["qty_on_hand"] != "7.500" {
		t.Fatalf("waste: %d %v", res.StatusCode, out)
	}
	if res, out = postMovement(t, srv.URL, owner, kopi, "adjustment", "-0.25", "selisih timbangan"); out["qty_on_hand"] != "7.250" {
		t.Fatalf("adjustment: %d %v", res.StatusCode, out)
	}
	if got := levelOf(t, srv.URL, owner, kopi); got != "7.250" {
		t.Fatalf("saldo akhir: %s", got)
	}

	_, hist := call(t, "GET", movementsURL(srv.URL, owner, ""), nil, owner.token)
	items := hist["items"].([]any)
	if len(items) != 3 || items[0].(map[string]any)["type"] != "adjustment" || items[2].(map[string]any)["type"] != "purchase_receive" {
		t.Fatalf("riwayat terbaru dulu: %v", items)
	}
	if items[1].(map[string]any)["reason"] != "tumpah" {
		t.Fatalf("alasan: %v", items[1])
	}
	_, onlyWaste := call(t, "GET", movementsURL(srv.URL, owner, "type=waste"), nil, owner.token)
	if n := len(onlyWaste["items"].([]any)); n != 1 {
		t.Fatalf("filter type: %d", n)
	}
	_, other := call(t, "GET", movementsURL(srv.URL, owner, "variant_id="+uuid.NewString()), nil, owner.token)
	if n := len(other["items"].([]any)); n != 0 {
		t.Fatalf("filter varian lain: %d", n)
	}
}

func TestInventoryMovementValidationAndTracking(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	kopi := trackedProduct(t, srv.URL, owner, "Biji Kopi", "BK")
	_, untracked := createProduct(t, srv.URL, owner.token, productBody("Roti", "ROTI", nil, 0, 1000, nil))

	cases := map[string]struct {
		typ, qty, reason, field string
	}{
		"nol":                  {"purchase_receive", "0", "", "qty_delta"},
		"masuk negatif":        {"purchase_receive", "-1", "", "qty_delta"},
		"waste positif":        {"waste", "1", "x", "qty_delta"},
		"4 desimal":            {"adjustment", "1.2345", "x", "qty_delta"},
		"bukan angka":          {"adjustment", "banyak", "x", "qty_delta"},
		"alasan waste wajib":   {"waste", "-1", "", "reason"},
		"alasan koreksi wajib": {"adjustment", "1", " ", "reason"},
		"tipe opname ditolak":  {"opname", "1", "x", "type"},
	}
	for name, c := range cases {
		res, out := postMovement(t, srv.URL, owner, kopi, c.typ, c.qty, c.reason)
		errs, _ := out["errors"].([]any)
		if res.StatusCode != http.StatusUnprocessableEntity || len(errs) == 0 || errs[0].(map[string]any)["field"] != c.field {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	if got := levelOf(t, srv.URL, owner, kopi); got != "0.000" {
		t.Fatalf("validasi gagal tidak boleh mengubah saldo: %s", got)
	}

	res, out := postMovement(t, srv.URL, owner, firstVariant(untracked)["id"].(string), "purchase_receive", "5", "")
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "STOCK_NOT_TRACKED" {
		t.Fatalf("produk tidak melacak stok: %d %v", res.StatusCode, out)
	}
	res, out = postMovement(t, srv.URL, owner, uuid.NewString(), "purchase_receive", "5", "")
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_VARIANT" {
		t.Fatalf("varian acak: %d %v", res.StatusCode, out)
	}
	res, out = call(t, "POST", srv.URL+"/v1/inventory/movements", map[string]any{"variant_id": kopi, "type": "purchase_receive", "qty_delta": "1"}, owner.token)
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("tanpa store_id: %d %v", res.StatusCode, out)
	}
	if res, _ = call(t, "GET", srv.URL+"/v1/inventory/levels", nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("levels tanpa store_id: %d", res.StatusCode)
	}
	if res, _ = call(t, "GET", movementsURL(srv.URL, owner, "type=ngawur"), nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("type ngawur: %d", res.StatusCode)
	}
}

func TestInventoryCountsApplyDifferencesAtomically(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	a := trackedProduct(t, srv.URL, owner, "Gula", "GULA")
	b := trackedProduct(t, srv.URL, owner, "Susu", "SUSU")
	postMovement(t, srv.URL, owner, a, "purchase_receive", "10", "")
	postMovement(t, srv.URL, owner, b, "purchase_receive", "4", "")

	res, out := postCounts(t, srv.URL, owner,
		map[string]any{"variant_id": a, "counted_qty": "8.5"},
		map[string]any{"variant_id": b, "counted_qty": "4"},
	)
	if res.StatusCode != http.StatusOK || out["adjusted"] != float64(1) {
		t.Fatalf("opname: %d %v", res.StatusCode, out)
	}
	results := out["results"].([]any)
	first := results[0].(map[string]any)
	if first["variant_id"] != a || first["before"] != "10.000" || first["counted"] != "8.500" || first["delta"] != "-1.500" {
		t.Fatalf("hasil A (urutan sesuai input): %v", first)
	}
	if results[1].(map[string]any)["delta"] != "0.000" {
		t.Fatalf("hasil B: %v", results[1])
	}
	if levelOf(t, srv.URL, owner, a) != "8.500" || levelOf(t, srv.URL, owner, b) != "4.000" {
		t.Fatal("saldo setelah opname salah")
	}
	_, hist := call(t, "GET", movementsURL(srv.URL, owner, "type=opname"), nil, owner.token)
	items := hist["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["reason"] != "Stok opname" || items[0].(map[string]any)["qty_delta"] != "-1.500" {
		t.Fatalf("hanya selisih nol yang dilewati: %v", items)
	}

	res, out = postCounts(t, srv.URL, owner,
		map[string]any{"variant_id": a, "counted_qty": "1"},
		map[string]any{"variant_id": uuid.NewString(), "counted_qty": "1"},
	)
	errs, _ := out["errors"].([]any)
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_VARIANT" || len(errs) != 1 || errs[0].(map[string]any)["field"] != "items[1].variant_id" {
		t.Fatalf("varian acak: %d %v", res.StatusCode, out)
	}
	if levelOf(t, srv.URL, owner, a) != "8.500" {
		t.Fatal("batch gagal harus membatalkan item yang valid juga")
	}

	for name, items := range map[string][]map[string]any{
		"duplikat": {{"variant_id": a, "counted_qty": "1"}, {"variant_id": a, "counted_qty": "2"}},
		"negatif":  {{"variant_id": a, "counted_qty": "-1"}},
		"kosong":   {},
	} {
		if res, out := postCounts(t, srv.URL, owner, items...); res.StatusCode != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
}

func TestInventoryConcurrentReceivesAreAtomic(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	kopi := trackedProduct(t, srv.URL, owner, "Biji Kopi", "BK")

	const workers = 20
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if res, out := postMovement(t, srv.URL, owner, kopi, "purchase_receive", "1", ""); res.StatusCode != http.StatusCreated {
				t.Errorf("receive: %d %v", res.StatusCode, out)
			}
		}()
	}
	wg.Wait()

	if got := levelOf(t, srv.URL, owner, kopi); got != "20.000" {
		t.Fatalf("saldo harus 20, dapat %s", got)
	}
	var sum, level string
	err := database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
		if err := tx.QueryRow(context.Background(), `SELECT COALESCE(SUM(qty_delta),0)::text FROM stock_movements WHERE variant_id = $1`, kopi).Scan(&sum); err != nil {
			return err
		}
		return tx.QueryRow(context.Background(), `SELECT qty_on_hand::text FROM stock_levels WHERE variant_id = $1`, kopi).Scan(&level)
	})
	if err != nil || sum != "20.000" || level != "20.000" {
		t.Fatalf("saldo harus sama dengan jumlah ledger: sum=%s level=%s err=%v", sum, level, err)
	}
}

func TestInventoryLedgerIsAppendOnly(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	kopi := trackedProduct(t, srv.URL, owner, "Biji Kopi", "BK")
	postMovement(t, srv.URL, owner, kopi, "purchase_receive", "3", "")

	for name, stmt := range map[string]string{
		"update": `UPDATE stock_movements SET reason = 'dipalsukan'`,
		"delete": `DELETE FROM stock_movements`,
	} {
		err := database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
			_, err := tx.Exec(context.Background(), stmt)
			return err
		})
		if err == nil {
			t.Errorf("%s pada ledger harus ditolak", name)
		}
	}
	_, hist := call(t, "GET", movementsURL(srv.URL, owner, ""), nil, owner.token)
	if len(hist["items"].([]any)) != 1 {
		t.Fatal("ledger harus tetap utuh")
	}
}

func TestInventoryAuthorizationStoresAndIsolation(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)
	kopi := trackedProduct(t, srv.URL, owner, "Biji Kopi", "BK")

	if res, _ := call(t, "GET", srv.URL+"/v1/inventory/levels?store_id="+owner.storeID.String(), nil, cashier); res.StatusCode != http.StatusOK {
		t.Fatalf("kasir boleh melihat saldo: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", movementsURL(srv.URL, owner, ""), nil, cashier); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh melihat riwayat: %d", res.StatusCode)
	}
	asCashier := owner
	asCashier.token = cashier
	if res, out := postMovement(t, srv.URL, asCashier, kopi, "purchase_receive", "1", ""); res.StatusCode != http.StatusForbidden || out["code"] != "FORBIDDEN" {
		t.Fatalf("kasir tidak boleh mencatat pergerakan: %d %v", res.StatusCode, out)
	}
	if res, _ := postCounts(t, srv.URL, asCashier, map[string]any{"variant_id": kopi, "counted_qty": "1"}); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh opname: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/inventory/levels?store_id="+owner.storeID.String(), nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}

	intruder := owner
	intruder.token = other.token
	if res, out := postMovement(t, srv.URL, intruder, kopi, "purchase_receive", "1", ""); res.StatusCode != http.StatusNotFound || out["code"] != "STORE_NOT_FOUND" {
		t.Fatalf("tenant lain memakai outlet kita: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/inventory/levels?store_id="+owner.storeID.String(), nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("saldo outlet lintas tenant: %d", res.StatusCode)
	}
	ownStore := other
	if res, out := postMovement(t, srv.URL, ownStore, kopi, "purchase_receive", "1", ""); res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_VARIANT" {
		t.Fatalf("varian tenant lain di outlet sendiri: %d %v", res.StatusCode, out)
	}

	secondStore := uuid.New()
	err := database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `INSERT INTO stores (id, tenant_id, code, name) VALUES ($1,$2,'CAB2','Cabang 2')`, secondStore, owner.tenantID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	notMember := owner
	notMember.storeID = secondStore
	if res, out := postMovement(t, srv.URL, notMember, kopi, "purchase_receive", "1", ""); res.StatusCode != http.StatusNotFound || out["code"] != "STORE_NOT_FOUND" {
		t.Fatalf("outlet tanpa keanggotaan: %d %v", res.StatusCode, out)
	}
}

func TestInventoryLevelsSearchAndPagination(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	_, untracked := createProduct(t, srv.URL, owner.token, productBody("Tidak Dilacak", "TDL", nil, 0, 1, nil))
	for i := 1; i <= 5; i++ {
		trackedProduct(t, srv.URL, owner, fmt.Sprintf("Bahan %d", i), fmt.Sprintf("B%d", i))
	}
	store := owner.storeID.String()

	seen := map[string]bool{}
	var names []string
	cursor := ""
	for page := 0; page < 10; page++ {
		url := srv.URL + "/v1/inventory/levels?store_id=" + store + "&limit=2"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		_, out := call(t, "GET", url, nil, owner.token)
		for _, it := range out["items"].([]any) {
			m := it.(map[string]any)
			if seen[m["variant_id"].(string)] {
				t.Fatalf("duplikat antar halaman: %v", m)
			}
			seen[m["variant_id"].(string)] = true
			names = append(names, m["product_name"].(string))
		}
		next, _ := out["next_cursor"].(string)
		if next == "" {
			break
		}
		cursor = next
	}
	want := []string{"Bahan 1", "Bahan 2", "Bahan 3", "Bahan 4", "Bahan 5"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("urutan/isi: %v (produk yang tidak dilacak tidak boleh muncul)", names)
	}
	if seen[firstVariant(untracked)["id"].(string)] {
		t.Fatal("produk tanpa pelacakan stok muncul")
	}

	_, found := call(t, "GET", srv.URL+"/v1/inventory/levels?store_id="+store+"&q=b3", nil, owner.token)
	if items := found["items"].([]any); len(items) != 1 || items[0].(map[string]any)["product_name"] != "Bahan 3" {
		t.Fatalf("cari SKU: %v", found["items"])
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/inventory/levels?store_id="+store+"&cursor=rusak!", nil, owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("cursor rusak: %d", res.StatusCode)
	}
}

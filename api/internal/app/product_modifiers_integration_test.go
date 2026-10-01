package app_test

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/platform/database"
)

func idsOf(p map[string]any) []string {
	raw, ok := p["modifier_group_ids"].([]any)
	if !ok {
		return nil // null atau hilang
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i] = v.(string)
	}
	return out
}

func sameIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if got == nil || len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func seedGroup(t *testing.T, base, token, name string) string {
	t.Helper()
	res, g := createGroup(t, base, token, groupBody(name, 0, 1, vspec{"name": "A"}, vspec{"name": "B"}))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("seed grup %s: %d %v", name, res.StatusCode, g)
	}
	return g["id"].(string)
}

// updateBody membangun body PATCH untuk produk sederhana yang sudah ada (varian dirujuk lewat id).
func updateBody(p map[string]any, groups any) map[string]any {
	b := productBody(p["name"].(string), "", nil, 0, 1000, map[string]any{"modifier_group_ids": groups})
	b["variants"].([]map[string]any)[0]["id"] = firstVariant(p)["id"]
	return b
}

func TestProductModifierLinkingCreateGetUpdate(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	sugar := seedGroup(t, srv.URL, owner.token, "Level Gula")
	topping := seedGroup(t, srv.URL, owner.token, "Topping")

	// urutan penautan dipertahankan (topping dulu, baru gula)
	res, p := createProduct(t, srv.URL, owner.token, productBody("Es Kopi", "", nil, 0, 15000, map[string]any{"modifier_group_ids": []string{topping, sugar}}))
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %v", res.StatusCode, p)
	}
	sameIDs(t, idsOf(p), topping, sugar)
	id := p["id"].(string)

	_, got := call(t, "GET", srv.URL+"/v1/products/"+id, nil, owner.token)
	sameIDs(t, idsOf(got), topping, sugar)
	_, list := call(t, "GET", srv.URL+"/v1/products", nil, owner.token)
	sameIDs(t, idsOf(list["items"].([]any)[0].(map[string]any)), topping, sugar)

	// produk tanpa penautan mengembalikan array kosong, bukan null
	_, plain := createProduct(t, srv.URL, owner.token, productBody("Air", "", nil, 0, 3000, nil))
	if raw, ok := plain["modifier_group_ids"].([]any); !ok || len(raw) != 0 {
		t.Fatalf("harus [] bukan null: %v", plain["modifier_group_ids"])
	}

	// PATCH mengganti seluruh penautan, termasuk urutan
	res, upd := callH(t, "PATCH", srv.URL+"/v1/products/"+id, updateBody(p, []string{sugar, topping}), owner.token, map[string]string{"If-Match": `"1"`})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update urutan: %d %v", res.StatusCode, upd)
	}
	sameIDs(t, idsOf(upd), sugar, topping)
	res, upd = callH(t, "PATCH", srv.URL+"/v1/products/"+id, updateBody(p, []string{sugar}), owner.token, map[string]string{"If-Match": `"2"`})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update hapus grup: %d %v", res.StatusCode, upd)
	}
	sameIDs(t, idsOf(upd), sugar)

	// field dihilangkan = tanpa grup (PATCH mengganti seluruh field yang dapat diubah)
	omitted := productBody("Es Kopi", "", nil, 0, 1000, nil)
	omitted["variants"].([]map[string]any)[0]["id"] = firstVariant(p)["id"]
	res, upd = callH(t, "PATCH", srv.URL+"/v1/products/"+id, omitted, owner.token, map[string]string{"If-Match": `"3"`})
	if res.StatusCode != http.StatusOK || len(idsOf(upd)) != 0 || idsOf(upd) == nil {
		t.Fatalf("hilang = kosong: %d %v", res.StatusCode, upd["modifier_group_ids"])
	}
}

func TestProductModifierLinkingValidation(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	mine := seedGroup(t, srv.URL, owner.token, "Level Gula")
	foreign := seedGroup(t, srv.URL, other.token, "Punya Orang")

	cases := map[string]struct {
		groups []string
		status int
		code   string
		field  string
	}{
		"id acak":          {[]string{mine, uuid.NewString()}, http.StatusUnprocessableEntity, "INVALID_MODIFIER_GROUP", "modifier_group_ids[1]"},
		"grup tenant lain": {[]string{foreign}, http.StatusUnprocessableEntity, "INVALID_MODIFIER_GROUP", "modifier_group_ids[0]"},
		"duplikat":         {[]string{mine, mine}, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "modifier_group_ids[1]"},
	}
	for name, c := range cases {
		res, out := createProduct(t, srv.URL, owner.token, productBody("X", "", nil, 0, 1, map[string]any{"modifier_group_ids": c.groups}))
		errs, _ := out["errors"].([]any)
		if res.StatusCode != c.status || out["code"] != c.code || len(errs) != 1 || errs[0].(map[string]any)["field"] != c.field {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	// lebih dari 10 grup
	many := make([]string, 11)
	for i := range many {
		many[i] = uuid.NewString()
	}
	if res, out := createProduct(t, srv.URL, owner.token, productBody("X", "", nil, 0, 1, map[string]any{"modifier_group_ids": many})); res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "VALIDATION_FAILED" {
		t.Errorf("11 grup: %d %v", res.StatusCode, out)
	}
	// kegagalan penautan membatalkan seluruh transaksi: tidak ada produk tersimpan
	_, list := call(t, "GET", srv.URL+"/v1/products", nil, owner.token)
	if n := len(list["items"].([]any)); n != 0 {
		t.Fatalf("tidak boleh ada produk setengah jadi, dapat %d", n)
	}

	// kegagalan saat update tidak mengubah penautan lama
	_, p := createProduct(t, srv.URL, owner.token, productBody("Es Kopi", "", nil, 0, 1000, map[string]any{"modifier_group_ids": []string{mine}}))
	res, out := callH(t, "PATCH", srv.URL+"/v1/products/"+p["id"].(string), updateBody(p, []string{uuid.NewString()}), owner.token, map[string]string{"If-Match": `"1"`})
	if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "INVALID_MODIFIER_GROUP" {
		t.Fatalf("update dengan grup acak: %d %v", res.StatusCode, out)
	}
	_, still := call(t, "GET", srv.URL+"/v1/products/"+p["id"].(string), nil, owner.token)
	sameIDs(t, idsOf(still), mine)
	if still["version"] != float64(1) {
		t.Fatalf("versi tidak boleh naik: %v", still["version"])
	}
}

func TestModifierGroupDeletionGuard(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	sugar := seedGroup(t, srv.URL, owner.token, "Level Gula")
	topping := seedGroup(t, srv.URL, owner.token, "Topping")

	_, p := createProduct(t, srv.URL, owner.token, productBody("Es Kopi", "", nil, 0, 1000, map[string]any{"modifier_group_ids": []string{sugar, topping}}))
	id := p["id"].(string)

	res, out := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+sugar, nil, owner.token)
	if res.StatusCode != http.StatusConflict || out["code"] != "MODIFIER_GROUP_IN_USE" {
		t.Fatalf("grup terpakai: %d %v", res.StatusCode, out)
	}
	// grup tetap ada dan tetap tertaut
	if res, _ := call(t, "GET", srv.URL+"/v1/modifier-groups/"+sugar, nil, owner.token); res.StatusCode != http.StatusOK {
		t.Fatalf("grup harus tetap ada: %d", res.StatusCode)
	}

	// melepas satu grup dari produk membebaskannya
	if res, o := callH(t, "PATCH", srv.URL+"/v1/products/"+id, updateBody(p, []string{topping}), owner.token, map[string]string{"If-Match": `"1"`}); res.StatusCode != http.StatusOK {
		t.Fatalf("lepas grup: %d %v", res.StatusCode, o)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+sugar, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("grup bebas harus bisa dihapus: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+topping, nil, owner.token); res.StatusCode != http.StatusConflict {
		t.Fatalf("topping masih dipakai: %d", res.StatusCode)
	}

	// menghapus produk melepas penautannya, sehingga grup bisa dihapus
	if res, _ := call(t, "DELETE", srv.URL+"/v1/products/"+id, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("hapus produk: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+topping, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("grup setelah produk dihapus: %d", res.StatusCode)
	}
	// grup yang sudah dihapus tidak bisa ditautkan lagi
	res, o := createProduct(t, srv.URL, owner.token, productBody("Baru", "", nil, 0, 1, map[string]any{"modifier_group_ids": []string{topping}}))
	if res.StatusCode != http.StatusUnprocessableEntity || o["code"] != "INVALID_MODIFIER_GROUP" {
		t.Fatalf("tautan ke grup terhapus: %d %v", res.StatusCode, o)
	}
}

func TestProductModifierLinksVisibleToCashierAndIsolated(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)
	sugar := seedGroup(t, srv.URL, owner.token, "Level Gula")

	_, p := createProduct(t, srv.URL, owner.token, productBody("Es Kopi", "", nil, 0, 1000, map[string]any{"modifier_group_ids": []string{sugar}}))
	// kasir membaca penautan (dibutuhkan layar kasir)
	_, got := call(t, "GET", srv.URL+"/v1/products/"+p["id"].(string), nil, cashier)
	sameIDs(t, idsOf(got), sugar)
	// tenant lain tidak melihat produk maupun bisa menautkan grup ini
	if res, _ := call(t, "GET", srv.URL+"/v1/products/"+p["id"].(string), nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("lintas tenant: %d", res.StatusCode)
	}
	if res, out := createProduct(t, srv.URL, other.token, productBody("Curang", "", nil, 0, 1, map[string]any{"modifier_group_ids": []string{sugar}})); res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("tautan lintas tenant: %d %v", res.StatusCode, out)
	}
}

// Penaut dan penghapus grup yang sama berjalan bersamaan: tidak boleh tersisa tautan ke grup terhapus.
func TestModifierLinkVsDeleteRace(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	for round := range 15 {
		g := seedGroup(t, srv.URL, owner.token, "G"+string(rune('A'+round)))
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			createProduct(t, srv.URL, owner.token, productBody("P"+string(rune('A'+round)), "", nil, 0, 1, map[string]any{"modifier_group_ids": []string{g}}))
		}()
		go func() {
			defer wg.Done()
			call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+g, nil, owner.token)
		}()
		wg.Wait()

		var orphans int
		err := database.WithTenantTx(context.Background(), pool, owner.tenantID, func(tx pgx.Tx) error {
			return tx.QueryRow(context.Background(), `SELECT count(*) FROM product_modifier_groups l
				JOIN modifier_groups g ON g.id = l.group_id WHERE g.deleted_at IS NOT NULL`).Scan(&orphans)
		})
		if err != nil || orphans != 0 {
			t.Fatalf("ronde %d: tautan ke grup terhapus = %d err=%v", round, orphans, err)
		}
	}
}

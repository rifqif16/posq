package app_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func groupBody(name string, min, max int, mods ...vspec) map[string]any {
	ms := make([]map[string]any, len(mods))
	for i, m := range mods {
		ms[i] = m
	}
	return map[string]any{"name": name, "min_select": min, "max_select": max, "modifiers": ms}
}

func createGroup(t *testing.T, base, token string, body map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return call(t, "POST", base+"/v1/modifier-groups", body, token)
}

func patchGroup(t *testing.T, base, token, id string, version int, body map[string]any) (*http.Response, map[string]any) {
	t.Helper()
	return callH(t, "PATCH", base+"/v1/modifier-groups/"+id, body, token, map[string]string{"If-Match": `"` + string(rune('0'+version)) + `"`})
}

func modNames(g map[string]any) []string {
	var out []string
	for _, m := range g["modifiers"].([]any) {
		out = append(out, m.(map[string]any)["name"].(string))
	}
	return out
}

func modByName(t *testing.T, g map[string]any, name string) map[string]any {
	t.Helper()
	for _, m := range g["modifiers"].([]any) {
		if m.(map[string]any)["name"] == name {
			return m.(map[string]any)
		}
	}
	t.Fatalf("opsi %q tidak ada: %v", name, g["modifiers"])
	return nil
}

func sameNames(t *testing.T, got []string, want ...string) {
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

func TestModifierGroupCreateGetAndList(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	res, g := createGroup(t, srv.URL, owner.token, groupBody("Level Gula", 1, 1,
		vspec{"name": "Normal", "is_default": true},
		vspec{"name": "Less"},
		vspec{"name": "Tanpa Gula", "is_active": false},
	))
	if res.StatusCode != http.StatusCreated || res.Header.Get("ETag") != `"1"` || g["is_required"] != true || g["version"] != float64(1) {
		t.Fatalf("create: %d etag=%q %v", res.StatusCode, res.Header.Get("ETag"), g)
	}
	sameNames(t, modNames(g), "Normal", "Less", "Tanpa Gula") // urutan input dipertahankan
	if modByName(t, g, "Normal")["is_default"] != true || modByName(t, g, "Tanpa Gula")["is_active"] != false {
		t.Fatalf("flag opsi: %v", g["modifiers"])
	}

	res, topping := createGroup(t, srv.URL, owner.token, groupBody("Topping", 0, 3,
		vspec{"name": "Extra Shot", "price_delta": 5000}, vspec{"name": "Boba", "price_delta": 4000}))
	if res.StatusCode != http.StatusCreated || topping["is_required"] != false || modByName(t, topping, "Extra Shot")["price_delta"] != float64(5000) {
		t.Fatalf("topping: %d %v", res.StatusCode, topping)
	}

	res, one := call(t, "GET", srv.URL+"/v1/modifier-groups/"+g["id"].(string), nil, owner.token)
	if res.StatusCode != http.StatusOK || res.Header.Get("ETag") != `"1"` || one["name"] != "Level Gula" {
		t.Fatalf("get: %d %v", res.StatusCode, one)
	}
	_, list := call(t, "GET", srv.URL+"/v1/modifier-groups", nil, owner.token)
	items := list["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["name"] != "Level Gula" || items[1].(map[string]any)["name"] != "Topping" {
		t.Fatalf("list urut nama: %v", list)
	}
	if res, out := createGroup(t, srv.URL, owner.token, groupBody("level gula", 0, 1, vspec{"name": "A"})); res.StatusCode != http.StatusConflict || out["code"] != "MODIFIER_GROUP_NAME_TAKEN" {
		t.Fatalf("nama grup kembar: %d %v", res.StatusCode, out)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/modifier-groups/bukan-uuid", nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("id non-uuid: %d", res.StatusCode)
	}
}

func TestModifierGroupValidation(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	a, b := vspec{"name": "A"}, vspec{"name": "B"}

	cases := map[string]struct {
		body  map[string]any
		field string
	}{
		"nama kosong":         {groupBody(" ", 0, 1, a), "name"},
		"max kurang dari min": {groupBody("G", 2, 1, a, b), "max_select"},
		"min melebihi aktif":  {groupBody("G", 3, 3, a, b), "min_select"},
		"tanpa opsi":          {groupBody("G", 0, 1), "modifiers"},
		"nama opsi kembar":    {groupBody("G", 0, 1, a, vspec{"name": "a"}), "modifiers[1].name"},
		"harga terlalu besar": {groupBody("G", 0, 1, vspec{"name": "A", "price_delta": 1_000_000_001}), "modifiers[0].price_delta"},
		"harga negatif":       {groupBody("G", 0, 1, vspec{"name": "A", "price_delta": -1}), "modifiers[0].price_delta"},
		"default nonaktif":    {groupBody("G", 0, 1, vspec{"name": "A", "is_default": true, "is_active": false}, b), "modifiers[0].is_default"},
		"default > max":       {groupBody("G", 0, 1, vspec{"name": "A", "is_default": true}, vspec{"name": "B", "is_default": true}), "modifiers"},
	}
	for name, c := range cases {
		res, out := createGroup(t, srv.URL, owner.token, c.body)
		errs, _ := out["errors"].([]any)
		if res.StatusCode != http.StatusUnprocessableEntity || out["code"] != "VALIDATION_FAILED" || len(errs) == 0 || errs[0].(map[string]any)["field"] != c.field {
			t.Errorf("%s: %d %v", name, res.StatusCode, out)
		}
	}
	// tidak ada grup setengah jadi yang tersimpan
	_, list := call(t, "GET", srv.URL+"/v1/modifier-groups", nil, owner.token)
	if n := len(list["items"].([]any)); n != 0 {
		t.Fatalf("harus kosong, dapat %d", n)
	}
}

func TestModifierGroupUpdateSyncsOptions(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	_, g := createGroup(t, srv.URL, owner.token, groupBody("Topping", 0, 3,
		vspec{"name": "Boba", "price_delta": 4000}, vspec{"name": "Jelly", "price_delta": 3000}, vspec{"name": "Pudding", "price_delta": 5000}))
	id := g["id"].(string)
	boba, jelly := modByName(t, g, "Boba"), modByName(t, g, "Jelly")

	upd := groupBody("Topping", 0, 3)
	if res, out := patchGroup(t, srv.URL, owner.token, id, 1, upd); res.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("tanpa opsi harus 422: %d %v", res.StatusCode, out)
	}
	body := groupBody("Topping Baru", 1, 3,
		vspec{"id": jelly["id"], "name": "Jelly", "price_delta": 3500}, // urutan pindah ke depan + harga naik
		vspec{"id": boba["id"], "name": "Boba", "price_delta": 4000},
		vspec{"name": "Oreo", "price_delta": 6000}, // opsi baru; Pudding tidak dikirim -> dihapus
	)
	if res, out := callH(t, "PATCH", srv.URL+"/v1/modifier-groups/"+id, body, owner.token, nil); res.StatusCode != http.StatusPreconditionRequired || out["code"] != "PRECONDITION_REQUIRED" {
		t.Fatalf("tanpa If-Match: %d %v", res.StatusCode, out)
	}
	if res, out := patchGroup(t, srv.URL, owner.token, id, 9, body); res.StatusCode != http.StatusPreconditionFailed || out["code"] != "VERSION_CONFLICT" {
		t.Fatalf("versi basi: %d %v", res.StatusCode, out)
	}
	res, out := patchGroup(t, srv.URL, owner.token, id, 1, body)
	if res.StatusCode != http.StatusOK || out["version"] != float64(2) || out["name"] != "Topping Baru" || out["is_required"] != true {
		t.Fatalf("update: %d %v", res.StatusCode, out)
	}
	sameNames(t, modNames(out), "Jelly", "Boba", "Oreo")
	if modByName(t, out, "Jelly")["id"] != jelly["id"] || modByName(t, out, "Jelly")["price_delta"] != float64(3500) {
		t.Fatalf("opsi lama harus mempertahankan id: %v", out["modifiers"])
	}
	if res, _ := patchGroup(t, srv.URL, owner.token, id, 1, body); res.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("versi lama setelah update: %d", res.StatusCode)
	}

	// pertukaran nama antar opsi dalam satu update tidak boleh menabrak indeks unik
	oreo := modByName(t, out, "Oreo")
	res, out = patchGroup(t, srv.URL, owner.token, id, 2, groupBody("Topping Baru", 1, 3,
		vspec{"id": jelly["id"], "name": "Boba"}, vspec{"id": boba["id"], "name": "Jelly"}, vspec{"id": oreo["id"], "name": "Oreo"}))
	if res.StatusCode != http.StatusOK || modByName(t, out, "Boba")["id"] != jelly["id"] || modByName(t, out, "Jelly")["id"] != boba["id"] {
		t.Fatalf("swap nama: %d %v", res.StatusCode, out)
	}

	// nama bekas opsi terhapus ("Pudding") bebas dipakai lagi
	res, out = patchGroup(t, srv.URL, owner.token, id, 3, groupBody("Topping Baru", 1, 3,
		vspec{"id": jelly["id"], "name": "Boba"}, vspec{"id": boba["id"], "name": "Jelly"}, vspec{"id": oreo["id"], "name": "Oreo"}, vspec{"name": "Pudding"}))
	if res.StatusCode != http.StatusOK || len(out["modifiers"].([]any)) != 4 {
		t.Fatalf("pakai ulang nama: %d %v", res.StatusCode, out)
	}

	// id opsi tak dikenal ditolak; nama grup bentrok ditolak
	_, other := createGroup(t, srv.URL, owner.token, groupBody("Ukuran Es", 0, 1, vspec{"name": "Banyak"}))
	for name, mid := range map[string]any{"acak": uuid.NewString(), "grup lain": modByName(t, other, "Banyak")["id"]} {
		res, o := patchGroup(t, srv.URL, owner.token, id, 4, groupBody("Topping Baru", 0, 3, vspec{"id": mid, "name": "X"}))
		errs, _ := o["errors"].([]any)
		if res.StatusCode != http.StatusUnprocessableEntity || o["code"] != "INVALID_MODIFIER" || len(errs) != 1 || errs[0].(map[string]any)["field"] != "modifiers[0].id" {
			t.Errorf("id %s: %d %v", name, res.StatusCode, o)
		}
	}
	res, o := patchGroup(t, srv.URL, owner.token, other["id"].(string), 1, groupBody("topping baru", 0, 1, vspec{"id": modByName(t, other, "Banyak")["id"], "name": "Banyak"}))
	if res.StatusCode != http.StatusConflict || o["code"] != "MODIFIER_GROUP_NAME_TAKEN" {
		t.Fatalf("nama grup bentrok: %d %v", res.StatusCode, o)
	}
}

func TestModifierGroupDeleteAuthorizationAndIsolation(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	other := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)

	_, g := createGroup(t, srv.URL, owner.token, groupBody("Level Gula", 1, 1, vspec{"name": "Normal"}))
	id := g["id"].(string)

	// kasir boleh membaca (untuk layar kasir) tetapi tidak boleh mengelola
	if res, _ := call(t, "GET", srv.URL+"/v1/modifier-groups", nil, cashier); res.StatusCode != http.StatusOK {
		t.Fatalf("kasir baca: %d", res.StatusCode)
	}
	if res, out := createGroup(t, srv.URL, cashier, groupBody("X", 0, 1, vspec{"name": "A"})); res.StatusCode != http.StatusForbidden || out["code"] != "FORBIDDEN" {
		t.Fatalf("kasir tulis: %d %v", res.StatusCode, out)
	}
	if res, _ := patchGroup(t, srv.URL, cashier, id, 1, groupBody("X", 0, 1, vspec{"name": "A"})); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir patch: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+id, nil, cashier); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir delete: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/modifier-groups", nil, ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}

	// tenant lain tidak melihat atau mengubah
	if res, _ := call(t, "GET", srv.URL+"/v1/modifier-groups/"+id, nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("get lintas tenant: %d", res.StatusCode)
	}
	if res, _ := patchGroup(t, srv.URL, other.token, id, 1, groupBody("X", 0, 1, vspec{"name": "A"})); res.StatusCode != http.StatusNotFound {
		t.Fatalf("patch lintas tenant: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+id, nil, other.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("delete lintas tenant: %d", res.StatusCode)
	}
	_, ol := call(t, "GET", srv.URL+"/v1/modifier-groups", nil, other.token)
	if n := len(ol["items"].([]any)); n != 0 {
		t.Fatalf("tenant lain melihat %d grup", n)
	}
	if res, _ := createGroup(t, srv.URL, other.token, groupBody("Level Gula", 0, 1, vspec{"name": "Normal"})); res.StatusCode != http.StatusCreated {
		t.Fatalf("nama sama di tenant lain harus boleh: %d", res.StatusCode)
	}

	// hapus: 204, lalu 404, dan nama bisa dipakai lagi
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+id, nil, owner.token); res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if res, _ := call(t, "GET", srv.URL+"/v1/modifier-groups/"+id, nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("get terhapus: %d", res.StatusCode)
	}
	if res, _ := call(t, "DELETE", srv.URL+"/v1/modifier-groups/"+id, nil, owner.token); res.StatusCode != http.StatusNotFound {
		t.Fatalf("delete ulang: %d", res.StatusCode)
	}
	if res, out := createGroup(t, srv.URL, owner.token, groupBody("Level Gula", 0, 1, vspec{"name": "Normal"})); res.StatusCode != http.StatusCreated {
		t.Fatalf("pakai ulang nama: %d %v", res.StatusCode, out)
	}
}

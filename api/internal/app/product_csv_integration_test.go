package app_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func rawCall(t *testing.T, method, url, body, token string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res, data
}

type importResult struct {
	Valid      bool `json:"valid"`
	Products   int  `json:"products"`
	Variants   int  `json:"variants"`
	Created    int  `json:"created"`
	ErrorCount int  `json:"error_count"`
	Code       string
	Errors     []struct {
		Row     int    `json:"row"`
		Column  string `json:"column"`
		Message string `json:"message"`
	} `json:"errors"`
}

func importCSV(t *testing.T, base, token, body string, commit bool) (int, importResult) {
	t.Helper()
	url := base + "/v1/products/import"
	if commit {
		url += "?commit=true"
	}
	res, data := rawCall(t, "POST", url, body, token)
	var out importResult
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("respons bukan JSON: %s", data)
	}
	return res.StatusCode, out
}

func productCount(t *testing.T, base, token string) int {
	t.Helper()
	_, list := call(t, "GET", base+"/v1/products?limit=100", nil, token)
	return len(list["items"].([]any))
}

func hasError(r importResult, row int, column string) bool {
	for _, e := range r.Errors {
		if e.Row == row && e.Column == column {
			return true
		}
	}
	return false
}

const csvHeader = "product_name,category,kitchen_station,taxable,track_stock,is_active,modifier_groups,variant_name,sku,barcodes,cost_price,sell_price,variant_is_active\n"

func seedImportRefs(t *testing.T, base, token string) {
	t.Helper()
	_, parent := createCategory(t, base, token, map[string]any{"name": "Minuman"})
	createCategory(t, base, token, map[string]any{"name": "Kopi", "parent_id": parent["id"]})
	seedGroup(t, base, token, "Level Gula")
}

func TestProductCSVImportValidateThenCommit(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)
	seedImportRefs(t, srv.URL, owner.token)

	body := csvHeader +
		"Es Kopi,Minuman > Kopi,bar,true,false,true,Level Gula,Small,K-S,111,5000,10000,true\n" +
		"Es Kopi,,,,,,,Large,,,7000,15000,true\n" +
		"Roti,,,,,,,,ROTI,222,,3000,\n"

	status, rep := importCSV(t, srv.URL, owner.token, body, false)
	if status != http.StatusOK || !rep.Valid || rep.Products != 2 || rep.Variants != 3 || rep.Created != 0 || rep.ErrorCount != 0 {
		t.Fatalf("validasi: %d %+v", status, rep)
	}
	if n := productCount(t, srv.URL, owner.token); n != 0 {
		t.Fatalf("validasi tidak boleh menulis data, ada %d produk", n)
	}

	status, rep = importCSV(t, srv.URL, owner.token, body, true)
	if status != http.StatusCreated || rep.Created != 2 {
		t.Fatalf("commit: %d %+v", status, rep)
	}
	_, list := call(t, "GET", srv.URL+"/v1/products?q=kopi", nil, owner.token)
	p := list["items"].([]any)[0].(map[string]any)
	if p["type"] != "variant" || p["kitchen_station"] != "bar" || len(idsOf(p)) != 1 || p["category_id"] == nil || len(variantsOf(p)) != 2 {
		t.Fatalf("produk hasil impor: %v", p)
	}
	if v := variantByName(t, p, "Small"); v["sku"] != "K-S" || v["cost_price"] != float64(5000) {
		t.Fatalf("varian: %v", v)
	}
	if v := variantByName(t, p, "Large"); len(v["sku"].(string)) != 12 || v["sku"].(string)[:4] != "SKU-" {
		t.Fatalf("sku otomatis: %v", v)
	}
	_, roti := call(t, "GET", srv.URL+"/v1/products?q=222", nil, owner.token)
	if len(roti["items"].([]any)) != 1 {
		t.Fatalf("barcode hasil impor harus bisa dicari")
	}

	status, rep = importCSV(t, srv.URL, owner.token, body, true)
	if status != http.StatusUnprocessableEntity || rep.Code != "CSV_INVALID" || rep.Created != 0 {
		t.Fatalf("impor ulang harus ditolak: %d %+v", status, rep)
	}
	if !hasError(rep, 2, "product_name") || !hasError(rep, 2, "sku") || !hasError(rep, 2, "barcodes") {
		t.Fatalf("konflik harus menunjuk baris dan kolom: %+v", rep.Errors)
	}
	if n := productCount(t, srv.URL, owner.token); n != 2 {
		t.Fatalf("jumlah produk harus tetap 2, dapat %d", n)
	}
}

func TestProductCSVImportIsAllOrNothing(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	body := csvHeader +
		"Kopi,,,,,,,,K1,,,1000,\n" +
		"Teh,Hantu,,,,,,,T1,,,1000,\n" +
		"Susu,,,,,,,,S1,,,15.000,\n"
	status, rep := importCSV(t, srv.URL, owner.token, body, true)
	if status != http.StatusUnprocessableEntity || rep.ErrorCount != 2 || rep.Created != 0 || rep.Valid {
		t.Fatalf("commit file rusak: %d %+v", status, rep)
	}
	if !hasError(rep, 3, "category") || !hasError(rep, 4, "sell_price") {
		t.Fatalf("detail error: %+v", rep.Errors)
	}
	if n := productCount(t, srv.URL, owner.token); n != 0 {
		t.Fatalf("produk valid pun tidak boleh tersimpan, ada %d", n)
	}

	status, rep = importCSV(t, srv.URL, owner.token, body, false)
	if status != http.StatusOK || rep.Valid || rep.ErrorCount != 2 || rep.Products != 3 {
		t.Fatalf("validasi file rusak: %d %+v", status, rep)
	}
}

func TestProductCSVImportFileLevelProblems(t *testing.T) {
	srv, _ := setup(t)
	owner := registerTenant(t, srv.URL)

	if status, rep := importCSV(t, srv.URL, owner.token, "", false); status != http.StatusOK || rep.Valid || !hasError(rep, 0, "") {
		t.Fatalf("file kosong: %d %+v", status, rep)
	}
	if status, rep := importCSV(t, srv.URL, owner.token, "nama,harga\nKopi,1000\n", false); status != http.StatusOK || !hasError(rep, 1, "product_name") || !hasError(rep, 1, "sell_price") {
		t.Fatalf("header salah: %d %+v", status, rep)
	}
	semicolon := "\xef\xbb\xbfproduct_name;sell_price\r\nKopi;1000\r\n"
	if status, rep := importCSV(t, srv.URL, owner.token, semicolon, true); status != http.StatusCreated || rep.Created != 1 {
		t.Fatalf("titik koma + BOM + CRLF: %d %+v", status, rep)
	}
	huge := "product_name,sell_price\n" + strings.Repeat("X,1\n", 700000)
	res, _ := rawCall(t, "POST", srv.URL+"/v1/products/import", huge, owner.token)
	if res.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("file terlalu besar: %d", res.StatusCode)
	}
}

func TestProductCSVExport(t *testing.T) {
	srv, pool := setup(t)
	owner := registerTenant(t, srv.URL)
	cashier := insertCashier(t, srv.URL, pool, owner)
	seedImportRefs(t, srv.URL, owner.token)

	if status, rep := importCSV(t, srv.URL, owner.token, csvHeader+
		"=Es Kopi,Minuman > Kopi,bar,true,false,true,Level Gula,Small,K-S,111|112,5000,10000,true\n"+
		"=Es Kopi,,,,,,,Large,K-L,,7000,15000,false\n", true); status != http.StatusCreated {
		t.Fatalf("seed: %d %+v", status, rep)
	}

	res, data := rawCall(t, "GET", srv.URL+"/v1/products/export", "", owner.token)
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/csv") ||
		!strings.Contains(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("header ekspor: %d %v", res.StatusCode, res.Header)
	}
	if !bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		t.Fatal("ekspor harus diawali BOM UTF-8")
	}
	records, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil || len(records) != 3 {
		t.Fatalf("baris CSV: %v %v", records, err)
	}
	row := records[1]
	if row[0] != "'=Es Kopi" || row[1] != "Minuman > Kopi" || row[6] != "Level Gula" || row[8] != "K-S" || row[9] != "111|112" || row[10] != "5000" || row[12] != "true" {
		t.Fatalf("isi baris pertama: %q", row)
	}
	if records[2][12] != "false" || records[2][7] != "Large" {
		t.Fatalf("baris kedua: %q", records[2])
	}

	res, data = rawCall(t, "GET", srv.URL+"/v1/products/export?delimiter=semicolon", "", owner.token)
	if res.StatusCode != http.StatusOK || !strings.Contains(string(data), "product_name;category;") {
		t.Fatalf("titik koma: %d %.80s", res.StatusCode, data)
	}
	if res, _ := rawCall(t, "GET", srv.URL+"/v1/products/export?delimiter=tab", "", owner.token); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("delimiter salah: %d", res.StatusCode)
	}

	if res, _ := rawCall(t, "GET", srv.URL+"/v1/products/export", "", cashier); res.StatusCode != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh ekspor (memuat harga beli): %d", res.StatusCode)
	}
	if res, _ := rawCall(t, "GET", srv.URL+"/v1/products/export", "", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tanpa token: %d", res.StatusCode)
	}
	if status, _ := importCSV(t, srv.URL, cashier, csvHeader+"X,,,,,,,,,,,1000,\n", false); status != http.StatusForbidden {
		t.Fatalf("kasir tidak boleh impor: %d", status)
	}
}

func TestProductCSVRoundTripAcrossTenants(t *testing.T) {
	srv, _ := setup(t)
	a := registerTenant(t, srv.URL)
	b := registerTenant(t, srv.URL)
	seedImportRefs(t, srv.URL, a.token)
	seedImportRefs(t, srv.URL, b.token)

	source := csvHeader +
		"-Latte,Minuman > Kopi,bar,true,true,true,Level Gula,Small,L-S,901,5000,10000,true\n" +
		"-Latte,,,,,,,Large,L-L,902|903,7000,15000,true\n" +
		"Air Mineral,Minuman,,false,false,true,,,AIR,904,2000,5000,true\n"
	if status, rep := importCSV(t, srv.URL, a.token, source, true); status != http.StatusCreated {
		t.Fatalf("impor tenant A: %d %+v", status, rep)
	}
	_, exported := rawCall(t, "GET", srv.URL+"/v1/products/export?delimiter=semicolon", "", a.token)

	if status, rep := importCSV(t, srv.URL, b.token, string(exported), true); status != http.StatusCreated || rep.Created != 2 {
		t.Fatalf("impor hasil ekspor ke tenant B (SKU/barcode sama boleh di tenant lain): %d %+v", status, rep)
	}
	_, again := rawCall(t, "GET", srv.URL+"/v1/products/export", "", b.token)
	rowsOf := func(raw []byte) [][]string {
		recs, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")))).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return recs
	}
	_, firstExport := rawCall(t, "GET", srv.URL+"/v1/products/export", "", a.token)
	got, want := rowsOf(again), rowsOf(firstExport)
	if len(got) != len(want) {
		t.Fatalf("jumlah baris berbeda: %d vs %d", len(got), len(want))
	}
	for i := range want {
		if strings.Join(got[i], "\x00") != strings.Join(want[i], "\x00") {
			t.Fatalf("baris %d berbeda:\n%q\n%q", i, got[i], want[i])
		}
	}
	if got[1][0] != "'-Latte" {
		t.Fatalf("nama berawalan '-' harus tetap terlindungi: %q", got[1][0])
	}
	_, list := call(t, "GET", srv.URL+"/v1/products?q=latte", nil, b.token)
	if name := list["items"].([]any)[0].(map[string]any)["name"]; name != "-Latte" {
		t.Fatalf("nama asli harus pulih tanpa apostrof: %v", name)
	}
}

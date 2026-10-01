package application

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

var (
	catDrinks = uuid.New()
	catCoffee = uuid.New()
	groupSug  = uuid.New()
	groupTop  = uuid.New()
)

func testLookups() csvLookups {
	drinks := domain.Category{ID: catDrinks, Name: "Minuman"}
	coffee := domain.Category{ID: catCoffee, ParentID: &catDrinks, Name: "Kopi"}
	return newLookups(
		[]domain.Category{drinks, coffee},
		[]domain.ModifierGroup{{ID: groupSug, Name: "Level Gula"}, {ID: groupTop, Name: "Topping"}},
	)
}

func parseAll(t *testing.T, csvText string) ([]plannedProduct, []RowError, int) {
	t.Helper()
	rows, errs := readCSVRows([]byte(csvText))
	if len(errs) > 0 {
		return nil, errs, 0
	}
	groups, gerrs := groupRows(rows)
	plan, perrs := planProducts(groups, testLookups())
	return plan, append(gerrs, perrs...), len(groups)
}

func errAt(errs []RowError, row int, col string) *RowError {
	for i := range errs {
		if errs[i].Row == row && errs[i].Column == col {
			return &errs[i]
		}
	}
	return nil
}

const validCSV = "product_name,category,kitchen_station,taxable,track_stock,is_active,modifier_groups,variant_name,sku,barcodes,cost_price,sell_price,variant_is_active\n" +
	"Es Kopi,Minuman > Kopi,bar,true,false,true,Level Gula|Topping,Small,K-S,111|112,5000,10000,true\n" +
	"Es Kopi,,,,,,,Large,K-L,,7000,15000,true\n" +
	"Roti,,,ya,,,,,ROTI,,,3000,\n"

func TestParseValidFileGroupsVariantsAndResolvesReferences(t *testing.T) {
	plan, errs, groups := parseAll(t, validCSV)
	if len(errs) != 0 || groups != 2 || len(plan) != 2 {
		t.Fatalf("groups=%d plan=%d errs=%v", groups, len(plan), errs)
	}
	kopi, roti := plan[0].input, plan[1].input
	if kopi.Name != "Es Kopi" || len(kopi.Variants) != 2 || kopi.KitchenStation != "bar" || kopi.CategoryID == nil || *kopi.CategoryID != catCoffee {
		t.Fatalf("kopi: %+v", kopi)
	}
	if len(kopi.ModifierGroupIDs) != 2 || kopi.ModifierGroupIDs[0] != groupSug || kopi.ModifierGroupIDs[1] != groupTop {
		t.Fatalf("urutan grup: %v", kopi.ModifierGroupIDs)
	}
	if v := kopi.Variants[0]; v.Name != "Small" || v.SKU != "K-S" || len(v.Barcodes) != 2 || v.CostPrice != 5000 || v.SellPrice != 10000 || !v.IsActive {
		t.Fatalf("varian 1: %+v", v)
	}
	if plan[0].rows[0] != 2 || plan[0].rows[1] != 3 || plan[1].rows[0] != 4 {
		t.Fatalf("nomor baris: %v %v", plan[0].rows, plan[1].rows)
	}
	if roti.Variants[0].Name != domain.DefaultVariantName || !roti.Taxable || !roti.IsActive || roti.Variants[0].CostPrice != 0 {
		t.Fatalf("roti: %+v", roti)
	}
}

func TestParseHandlesBOMSemicolonAndCRLF(t *testing.T) {
	semi := strings.ReplaceAll(validCSV, ",", ";")
	semi = strings.ReplaceAll(semi, "\n", "\r\n")
	plan, errs, _ := parseAll(t, csvBOM+semi)
	if len(errs) != 0 || len(plan) != 2 {
		t.Fatalf("errs=%v plan=%d", errs, len(plan))
	}
}

func TestParseRowErrorsCarryRowAndColumn(t *testing.T) {
	cases := []struct {
		name string
		csv  string
		row  int
		col  string
	}{
		{"kategori tidak ada", "product_name,category,sell_price\nKopi,Hantu,1000\n", 2, "category"},
		{"grup tidak ada", "product_name,modifier_groups,sell_price\nKopi,Tidak Ada,1000\n", 2, "modifier_groups"},
		{"harga jual kosong", "product_name,sell_price\nKopi,\n", 2, "sell_price"},
		{"harga desimal", "product_name,sell_price\nKopi,15000.5\n", 2, "sell_price"},
		{"harga dengan pemisah ribuan", "product_name,sell_price\nKopi,15.000\n", 2, "sell_price"},
		{"harga beli negatif", "product_name,sell_price,cost_price\nKopi,1000,-5\n", 2, "cost_price"},
		{"boolean salah", "product_name,sell_price,is_active\nKopi,1000,mungkin\n", 2, "is_active"},
		{"nama produk kosong", "product_name,sell_price\nKopi,1000\n,2000\n", 3, "product_name"},
		{"harga di atas batas", "product_name,sell_price\nKopi,1000000001\n", 2, "sell_price"},
		{"station salah", "product_name,sell_price,kitchen_station\nKopi,1000,Bar Atas\n", 2, "kitchen_station"},
		{"nama varian wajib bila lebih dari satu", "product_name,variant_name,sell_price\nKopi,Small,1000\nKopi,,2000\n", 3, "variant_name"},
		{"nama varian kembar", "product_name,variant_name,sell_price\nKopi,Small,1000\nKopi,small,2000\n", 3, "variant_name"},
		{"barcode kembar antar varian", "product_name,variant_name,barcodes,sell_price\nKopi,S,9,1000\nKopi,L,9,2000\n", 3, "barcodes"},
		{"semua varian nonaktif", "product_name,variant_name,sell_price,variant_is_active\nKopi,S,1000,false\nKopi,L,2000,false\n", 2, "variant_is_active"},
	}
	for _, c := range cases {
		_, errs, _ := parseAll(t, c.csv)
		if errAt(errs, c.row, c.col) == nil {
			t.Errorf("%s: harus ada error di baris %d kolom %s, dapat %v", c.name, c.row, c.col, errs)
		}
	}
}

func TestParseFileLevelErrors(t *testing.T) {
	if _, errs, _ := parseAll(t, ""); len(errs) != 1 || errs[0].Row != 0 {
		t.Errorf("file kosong: %v", errs)
	}
	if _, errs, _ := parseAll(t, "nama,harga\nKopi,1000\n"); errAt(errs, 1, "product_name") == nil || errAt(errs, 1, "sell_price") == nil {
		t.Errorf("kolom wajib: %v", errs)
	}
	if _, errs, _ := parseAll(t, "product_name,sell_price\n\"Kopi,1000\n"); len(errs) != 1 || errs[0].Message != "Format CSV tidak valid" {
		t.Errorf("tanda kutip rusak: %v", errs)
	}
}

func TestParseRowLimit(t *testing.T) {
	var b strings.Builder
	b.WriteString("product_name,sell_price\n")
	for i := 0; i < MaxCSVRows; i++ {
		fmt.Fprintf(&b, "P%d,1000\n", i)
	}
	if plan, errs, _ := parseAll(t, b.String()); len(errs) != 0 || len(plan) != MaxCSVRows {
		t.Fatalf("batas tepat harus lolos: errs=%v plan=%d", errs, len(plan))
	}
	b.WriteString("Lebih,1000\n")
	if _, errs, _ := parseAll(t, b.String()); len(errs) != 1 {
		t.Fatalf("melebihi batas harus ditolak: %v", errs)
	}
}

func TestParseDetectsDuplicatesAcrossProducts(t *testing.T) {
	csvText := "product_name,variant_name,sku,barcodes,sell_price\n" +
		"Kopi,,K1,111,1000\n" +
		"Teh,,k1,222,1000\n" +
		"Susu,,S1,111,1000\n"
	_, errs, _ := parseAll(t, csvText)
	if e := errAt(errs, 3, "sku"); e == nil || !strings.Contains(e.Message, "baris 2") {
		t.Errorf("sku kembar: %v", errs)
	}
	if e := errAt(errs, 4, "barcodes"); e == nil || !strings.Contains(e.Message, "baris 2") {
		t.Errorf("barcode kembar: %v", errs)
	}
}

func TestTakenErrorsPointAtOffendingRows(t *testing.T) {
	plan, errs, _ := parseAll(t, validCSV)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	taken := TakenCodes{
		Names:    map[string]bool{"roti": true},
		SKUs:     map[string]bool{"k-l": true},
		Barcodes: map[string]bool{"112": true},
	}
	got := takenErrors(plan, taken)
	if errAt(got, 4, "product_name") == nil || errAt(got, 3, "sku") == nil || errAt(got, 2, "barcodes") == nil || len(got) != 3 {
		t.Fatalf("takenErrors: %v", got)
	}
	c := candidatesOf(plan)
	if len(c.Names) != 2 || len(c.SKUs) != 3 || len(c.Barcodes) != 2 {
		t.Fatalf("kandidat: %+v", c)
	}
}

func TestSanitizeCellRoundTrip(t *testing.T) {
	for _, in := range []string{"=HYPERLINK(\"x\")", "+1", "-1", "@SUM(A1)", "\tx"} {
		s := sanitizeCell(in)
		if s != "'"+in || unsanitizeCell(s) != in {
			t.Errorf("%q -> %q -> %q", in, s, unsanitizeCell(s))
		}
	}
	for _, in := range []string{"Kopi", "", "'biasa", "a=b"} {
		if sanitizeCell(in) != in || unsanitizeCell(in) != in {
			t.Errorf("%q tidak boleh berubah", in)
		}
	}
}

func TestWriteProductsCSVAndRoundTrip(t *testing.T) {
	cat := catCoffee
	products := []domain.Product{{
		Name: "=Es Kopi", CategoryID: &cat, Taxable: true, IsActive: true, KitchenStation: "bar",
		ModifierGroupIDs: []uuid.UUID{groupTop, groupSug},
		Variants: []domain.Variant{
			{Name: "Small", SKU: "K-S", Barcodes: []string{"1", "2"}, CostPrice: 5000, SellPrice: 10000, IsActive: true},
			{Name: "Large", SKU: "K-L", SellPrice: 15000, IsActive: false},
		},
	}}
	maps := ExportMaps{
		Categories: categoryPaths([]domain.Category{{ID: catDrinks, Name: "Minuman"}, {ID: catCoffee, ParentID: &catDrinks, Name: "Kopi"}}),
		Groups:     map[uuid.UUID]string{groupSug: "Level Gula", groupTop: "Topping"},
	}
	for _, delim := range []rune{',', ';'} {
		var buf bytes.Buffer
		if err := WriteProductsCSV(&buf, products, maps, delim); err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		if !strings.HasPrefix(out, csvBOM) || !strings.Contains(out, "'=Es Kopi") {
			t.Fatalf("BOM/sanitasi formula: %q", out)
		}
		plan, errs, _ := parseAll(t, out)
		if len(errs) != 0 || len(plan) != 1 {
			t.Fatalf("delim %q: errs=%v plan=%d", delim, errs, len(plan))
		}
		in := plan[0].input
		if in.Name != "=Es Kopi" || *in.CategoryID != catCoffee || in.ModifierGroupIDs[0] != groupTop || in.ModifierGroupIDs[1] != groupSug {
			t.Fatalf("round trip produk: %+v", in)
		}
		if len(in.Variants) != 2 || in.Variants[0].Barcodes[1] != "2" || in.Variants[1].IsActive || in.Variants[1].SKU != "K-L" || in.Variants[0].CostPrice != 5000 {
			t.Fatalf("round trip varian: %+v", in.Variants)
		}
	}
}

package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func single() ProductInput {
	return ProductInput{
		Name: "  Es Kopi Susu ", IsActive: true, KitchenStation: " BAR ",
		Variants: []VariantInput{{SKU: " KOPI-01 ", Barcodes: []string{" 8999999000011 "}, CostPrice: 8000, SellPrice: 15000, IsActive: true}},
	}
}

func multi() ProductInput {
	return ProductInput{Name: "Kopi", IsActive: true, Variants: []VariantInput{
		{Name: "Small", SKU: "K-S", Barcodes: []string{"1"}, SellPrice: 10000, IsActive: true},
		{Name: "Large", SKU: "K-L", Barcodes: []string{"2"}, SellPrice: 15000, IsActive: true},
	}}
}

func issueFields(in ProductInput) []string {
	_, err := in.Validate()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	var out []string
	for _, i := range ve.Issues {
		out = append(out, i.Field)
	}
	return out
}

func TestSingleVariantNormalizes(t *testing.T) {
	got, err := single().Validate()
	if err != nil {
		t.Fatal(err)
	}
	v := got.Variants[0]
	if got.Name != "Es Kopi Susu" || got.KitchenStation != "bar" || v.SKU != "KOPI-01" ||
		v.Barcodes[0] != "8999999000011" || v.Name != DefaultVariantName {
		t.Fatalf("tidak ternormalisasi: %+v", got)
	}
}

func TestMultiVariantValid(t *testing.T) {
	if _, err := multi().Validate(); err != nil {
		t.Fatal(err)
	}
	if ProductType(1) != "simple" || ProductType(2) != "variant" || ProductType(20) != "variant" {
		t.Fatal("ProductType salah")
	}
}

func TestProductValidateFieldErrors(t *testing.T) {
	id := uuid.New()
	cases := map[string]func(*ProductInput){
		"name":                   func(p *ProductInput) { p.Name = " " },
		"kitchen_station":        func(p *ProductInput) { p.KitchenStation = "Bar Atas" },
		"variants[0].sell_price": func(p *ProductInput) { p.Variants[0].SellPrice = MaxPrice + 1 },
		"variants[0].cost_price": func(p *ProductInput) { p.Variants[0].CostPrice = -1 },
		"variants[0].sku":        func(p *ProductInput) { p.Variants[0].SKU = "ada spasi" },
		"variants[0].barcodes":   func(p *ProductInput) { p.Variants[0].Barcodes = []string{"1", "1"} },
		"variants[0].name":       func(p *ProductInput) { p.Variants[0].Name = strings.Repeat("a", 51) },
		"variants":               func(p *ProductInput) { p.Variants = nil },
	}
	for field, mutate := range cases {
		in := single()
		mutate(&in)
		if got := issueFields(in); len(got) != 1 || got[0] != field {
			t.Errorf("%s: issue = %v", field, got)
		}
	}

	multiCases := map[string]func(*ProductInput){
		"variants[1].name":     func(p *ProductInput) { p.Variants[1].Name = "small" },                  // duplikat case-insensitive
		"variants[1].sku":      func(p *ProductInput) { p.Variants[1].SKU = "k-s" },                     // SKU kembar
		"variants[1].barcodes": func(p *ProductInput) { p.Variants[1].Barcodes = []string{"1"} },        // barcode kembar lintas varian
		"variants[1].id":       func(p *ProductInput) { p.Variants[0].ID, p.Variants[1].ID = &id, &id }, // id dikirim dua kali
	}
	for field, mutate := range multiCases {
		in := multi()
		mutate(&in)
		if got := issueFields(in); len(got) != 1 || got[0] != field {
			t.Errorf("%s: issue = %v", field, got)
		}
	}

	noName := multi()
	noName.Variants[0].Name = ""
	if got := issueFields(noName); len(got) != 1 || got[0] != "variants[0].name" {
		t.Errorf("nama kosong pada multi-varian: %v", got)
	}
	inactive := multi()
	inactive.Variants[0].IsActive, inactive.Variants[1].IsActive = false, false
	if got := issueFields(inactive); len(got) != 1 || got[0] != "variants" {
		t.Errorf("semua nonaktif: %v", got)
	}
}

func TestVariantCountBoundaries(t *testing.T) {
	make20 := func(n int) ProductInput {
		in := ProductInput{Name: "X", IsActive: true}
		for i := 0; i < n; i++ {
			in.Variants = append(in.Variants, VariantInput{Name: fmt.Sprintf("V%d", i), SellPrice: 1, IsActive: true})
		}
		return in
	}
	if _, err := make20(MaxVariants).Validate(); err != nil {
		t.Fatalf("20 varian harus valid: %v", err)
	}
	if got := issueFields(make20(MaxVariants + 1)); len(got) != 1 || got[0] != "variants" {
		t.Fatalf("21 varian harus ditolak: %v", got)
	}

	ok := single()
	ok.Variants[0].SellPrice, ok.Variants[0].CostPrice = MaxPrice, 0
	ok.Variants[0].SKU = strings.Repeat("A", 64)
	if _, err := ok.Validate(); err != nil {
		t.Fatalf("batas atas harus valid: %v", err)
	}
	tooMany := single()
	tooMany.Variants[0].Barcodes = make([]string, MaxBarcodes+1)
	if _, err := tooMany.Validate(); err == nil {
		t.Fatal("lebih dari 10 barcode harus ditolak")
	}
}

package domain

import (
	"errors"
	"strings"
	"testing"
)

func validInput() ProductInput {
	return ProductInput{
		Name: "  Es Kopi Susu ", IsActive: true, KitchenStation: " BAR ",
		Variants: []VariantInput{{SKU: " KOPI-01 ", Barcodes: []string{" 8999999000011 "}, CostPrice: 8000, SellPrice: 15000}},
	}
}

func issueFields(err error) []string {
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

func TestProductValidateNormalizes(t *testing.T) {
	got, err := validInput().Validate()
	if err != nil {
		t.Fatal(err)
	}
	v := got.Variants[0]
	if got.Name != "Es Kopi Susu" || got.KitchenStation != "bar" || v.SKU != "KOPI-01" ||
		v.Barcodes[0] != "8999999000011" || v.Name != DefaultVariantName {
		t.Fatalf("tidak ternormalisasi: %+v", got)
	}
}

func TestProductValidateBoundaries(t *testing.T) {
	ok := validInput()
	ok.Variants[0].SellPrice = MaxPrice
	ok.Variants[0].CostPrice = 0
	ok.Variants[0].SKU = strings.Repeat("A", 64)
	if _, err := ok.Validate(); err != nil {
		t.Fatalf("batas atas harus valid: %v", err)
	}

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
		in := validInput()
		mutate(&in)
		fields := issueFields(func() error { _, err := in.Validate(); return err }())
		if len(fields) != 1 || fields[0] != field {
			t.Errorf("%s: issue = %v", field, fields)
		}
	}

	many := validInput()
	many.Variants[0].Barcodes = make([]string, MaxBarcodes+1)
	if _, err := many.Validate(); err == nil {
		t.Error("lebih dari 10 barcode harus ditolak")
	}
	two := validInput()
	two.Variants = append(two.Variants, two.Variants[0])
	if _, err := two.Validate(); err == nil {
		t.Error("dua varian harus ditolak di fase ini")
	}
}

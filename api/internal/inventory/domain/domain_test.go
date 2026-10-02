package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseQty(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0", 0, true}, {"1", 1000, true}, {"12.5", 12500, true}, {"12.500", 12500, true}, {"0.001", 1, true},
		{"-2.5", -2500, true}, {"+3", 3000, true}, {" 7 ", 7000, true}, {"999999999.999", 999_999_999_999, true},
		{"", 0, false}, {"abc", 0, false}, {"1.2345", 0, false}, {"1,5", 0, false}, {"1.", 0, false}, {".5", 0, false},
		{"1000000000", 0, false}, {"--1", 0, false}, {"1e3", 0, false},
	}
	for _, c := range cases {
		got, err := ParseQty(c.in)
		if (err == nil) != c.ok || (c.ok && got != c.want) {
			t.Errorf("ParseQty(%q) = %d, %v; want %d ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

func TestFormatQtyRoundTrip(t *testing.T) {
	for _, n := range []int64{0, 1, 999, 1000, 12500, -2500, -1, 999_999_999_999, -999_999_999_999} {
		got, err := ParseQty(FormatQty(n))
		if err != nil || got != n {
			t.Errorf("round trip %d -> %q -> %d %v", n, FormatQty(n), got, err)
		}
	}
	if FormatQty(1500) != "1.500" || FormatQty(-5) != "-0.005" || FormatQty(0) != "0.000" {
		t.Fatal("format salah")
	}
}

func validMovement() MovementInput {
	return MovementInput{StoreID: uuid.New(), VariantID: uuid.New(), Type: TypePurchaseReceive, QtyDelta: "10"}
}

func movementFields(in MovementInput) []string {
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

func TestMovementValidation(t *testing.T) {
	if v, err := validMovement().Validate(); err != nil || v.QtyDelta != 10000 {
		t.Fatalf("valid: %+v %v", v, err)
	}
	cost := int64(5000)
	ok := validMovement()
	ok.UnitCost = &cost
	if v, err := ok.Validate(); err != nil || *v.UnitCost != 5000 {
		t.Fatalf("receive dengan harga: %v", err)
	}
	waste := validMovement()
	waste.Type, waste.QtyDelta, waste.Reason = TypeWaste, "-2.5", "  tumpah "
	if v, err := waste.Validate(); err != nil || v.QtyDelta != -2500 || v.Reason != "tumpah" {
		t.Fatalf("waste: %+v %v", v, err)
	}

	bad := map[string]func(*MovementInput){
		"store_id":   func(m *MovementInput) { m.StoreID = uuid.Nil },
		"variant_id": func(m *MovementInput) { m.VariantID = uuid.Nil },
		"type":       func(m *MovementInput) { m.Type = "opname" },
		"qty_delta":  func(m *MovementInput) { m.QtyDelta = "0" },
		"reason":     func(m *MovementInput) { m.Type, m.QtyDelta = TypeAdjustment, "5" },
		"unit_cost":  func(m *MovementInput) { m.Type, m.QtyDelta, m.Reason, m.UnitCost = TypeAdjustment, "5", "x", &cost },
	}
	for field, mutate := range bad {
		in := validMovement()
		mutate(&in)
		if got := movementFields(in); len(got) != 1 || got[0] != field {
			t.Errorf("%s: %v", field, got)
		}
	}
	signs := []MovementInput{
		{StoreID: uuid.New(), VariantID: uuid.New(), Type: TypePurchaseReceive, QtyDelta: "-1"},
		{StoreID: uuid.New(), VariantID: uuid.New(), Type: TypeWaste, QtyDelta: "1", Reason: "x"},
		{StoreID: uuid.New(), VariantID: uuid.New(), Type: TypeAdjustment, QtyDelta: "1.2345", Reason: "x"},
		{StoreID: uuid.New(), VariantID: uuid.New(), Type: TypeAdjustment, QtyDelta: "1", Reason: strings.Repeat("a", 201)},
	}
	for i, in := range signs {
		if got := movementFields(in); len(got) != 1 {
			t.Errorf("kasus tanda/format %d: %v", i, got)
		}
	}
	adjust := validMovement()
	adjust.Type, adjust.QtyDelta, adjust.Reason = TypeAdjustment, "-0.001", strings.Repeat("a", 200)
	if _, err := adjust.Validate(); err != nil {
		t.Fatalf("batas bawah/atas: %v", err)
	}
}

func TestCountValidation(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	ok := CountInput{StoreID: uuid.New(), Items: []CountItemInput{{a, "5"}, {b, "0"}}}
	v, err := ok.Validate()
	if err != nil || v.Reason != DefaultOpname || v.Items[0].Counted != 5000 || v.Items[1].Counted != 0 {
		t.Fatalf("valid: %+v %v", v, err)
	}
	cases := map[string]CountInput{
		"store_id":             {Items: []CountItemInput{{a, "1"}}},
		"items":                {StoreID: uuid.New()},
		"items[1].variant_id":  {StoreID: uuid.New(), Items: []CountItemInput{{a, "1"}, {a, "2"}}},
		"items[0].counted_qty": {StoreID: uuid.New(), Items: []CountItemInput{{a, "-1"}}},
		"items[0].variant_id":  {StoreID: uuid.New(), Items: []CountItemInput{{uuid.Nil, "1"}}},
	}
	for field, in := range cases {
		_, err := in.Validate()
		var ve *ValidationError
		if !errors.As(err, &ve) || ve.Issues[0].Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
	many := CountInput{StoreID: uuid.New()}
	for i := 0; i <= MaxCountItems; i++ {
		many.Items = append(many.Items, CountItemInput{uuid.New(), "1"})
	}
	if _, err := many.Validate(); err == nil {
		t.Fatal("201 item harus ditolak")
	}
	many.Items = many.Items[:MaxCountItems]
	if _, err := many.Validate(); err != nil {
		t.Fatalf("200 item harus valid: %v", err)
	}
}

package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func sugarGroup() ModifierGroupInput {
	return ModifierGroupInput{Name: "  Level Gula ", MinSelect: 1, MaxSelect: 1, Modifiers: []ModifierInput{
		{Name: "Normal", IsDefault: true, IsActive: true},
		{Name: "Less", IsActive: true},
		{Name: "Extra Shot", PriceDelta: 5000, IsActive: true},
	}}
}

func modifierIssues(in ModifierGroupInput) []string {
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

func TestModifierGroupValidNormalizes(t *testing.T) {
	got, err := sugarGroup().Validate()
	if err != nil || got.Name != "Level Gula" {
		t.Fatalf("valid: %+v %v", got, err)
	}
	if (ModifierGroup{MinSelect: 1}).IsRequired() != true || (ModifierGroup{MinSelect: 0}).IsRequired() != false {
		t.Fatal("IsRequired salah")
	}
}

func TestModifierGroupFieldErrors(t *testing.T) {
	id := uuid.New()
	cases := map[string]func(*ModifierGroupInput){
		"name":                     func(g *ModifierGroupInput) { g.Name = " " },
		"min_select":               func(g *ModifierGroupInput) { g.MinSelect = -1 },
		"max_select":               func(g *ModifierGroupInput) { g.MaxSelect = 0; g.MinSelect = 0 },
		"modifiers":                func(g *ModifierGroupInput) { g.Modifiers = nil },
		"modifiers[1].name":        func(g *ModifierGroupInput) { g.Modifiers[1].Name = "normal" },
		"modifiers[2].price_delta": func(g *ModifierGroupInput) { g.Modifiers[2].PriceDelta = MaxPrice + 1 },
		"modifiers[1].is_default":  func(g *ModifierGroupInput) { g.Modifiers[1].IsDefault, g.Modifiers[1].IsActive = true, false },
		"modifiers[1].id":          func(g *ModifierGroupInput) { g.Modifiers[0].ID, g.Modifiers[1].ID = &id, &id },
	}
	for field, mutate := range cases {
		in := sugarGroup()
		mutate(&in)
		got := modifierIssues(in)
		if len(got) == 0 || got[0] != field {
			t.Errorf("%s: issue = %v", field, got)
		}
	}
}

func TestModifierSelectionRule(t *testing.T) {
	// max < min
	in := sugarGroup()
	in.MinSelect, in.MaxSelect = 2, 1
	if got := modifierIssues(in); len(got) != 1 || got[0] != "max_select" {
		t.Errorf("max<min: %v", got)
	}
	// minimal melebihi jumlah opsi aktif
	in = sugarGroup()
	in.MinSelect, in.MaxSelect = 4, 4
	if got := modifierIssues(in); len(got) != 1 || got[0] != "min_select" {
		t.Errorf("min>aktif: %v", got)
	}
	// opsi nonaktif tidak dihitung
	in = sugarGroup()
	in.Modifiers[1].IsActive, in.Modifiers[2].IsActive = false, false
	in.MinSelect, in.MaxSelect = 2, 2
	if got := modifierIssues(in); len(got) != 1 || got[0] != "min_select" {
		t.Errorf("nonaktif: %v", got)
	}
	// default melebihi maksimal
	in = sugarGroup()
	in.Modifiers[1].IsDefault = true
	if got := modifierIssues(in); len(got) != 1 || got[0] != "modifiers" {
		t.Errorf("default>max: %v", got)
	}
	// multi-pilih sah: min 0, max 3, dua default
	in = sugarGroup()
	in.MinSelect, in.MaxSelect = 0, 3
	in.Modifiers[1].IsDefault = true
	if _, err := in.Validate(); err != nil {
		t.Errorf("multi-pilih harus valid: %v", err)
	}
}

func TestModifierCountAndLengthBoundaries(t *testing.T) {
	make50 := func(n int) ModifierGroupInput {
		g := ModifierGroupInput{Name: "G", MinSelect: 0, MaxSelect: 1}
		for i := 0; i < n; i++ {
			g.Modifiers = append(g.Modifiers, ModifierInput{Name: fmt.Sprintf("O%d", i), IsActive: true})
		}
		return g
	}
	if _, err := make50(MaxModifiers).Validate(); err != nil {
		t.Fatalf("50 opsi harus valid: %v", err)
	}
	if got := modifierIssues(make50(MaxModifiers + 1)); len(got) != 1 || got[0] != "modifiers" {
		t.Fatalf("51 opsi: %v", got)
	}
	long := sugarGroup()
	long.Name = strings.Repeat("a", 100)
	long.Modifiers[0].Name = strings.Repeat("b", 100)
	if _, err := long.Validate(); err != nil {
		t.Fatalf("100 karakter harus valid: %v", err)
	}
	long.Name = strings.Repeat("a", 101)
	if got := modifierIssues(long); len(got) != 1 || got[0] != "name" {
		t.Fatalf("101 karakter: %v", got)
	}
}

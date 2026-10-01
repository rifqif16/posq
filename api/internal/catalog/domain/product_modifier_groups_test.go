package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestModifierGroupIDsValidation(t *testing.T) {
	a, b := uuid.New(), uuid.New()

	ok := single()
	ok.ModifierGroupIDs = []uuid.UUID{a, b}
	if _, err := ok.Validate(); err != nil {
		t.Fatalf("dua grup harus valid: %v", err)
	}
	none := single()
	none.ModifierGroupIDs = nil
	if _, err := none.Validate(); err != nil {
		t.Fatalf("tanpa grup harus valid: %v", err)
	}

	dup := single()
	dup.ModifierGroupIDs = []uuid.UUID{a, b, a}
	if got := issueFields(dup); len(got) != 1 || got[0] != "modifier_group_ids[2]" {
		t.Errorf("duplikat: %v", got)
	}

	max := single()
	for range MaxModifierGroups {
		max.ModifierGroupIDs = append(max.ModifierGroupIDs, uuid.New())
	}
	if _, err := max.Validate(); err != nil {
		t.Errorf("10 grup harus valid: %v", err)
	}
	max.ModifierGroupIDs = append(max.ModifierGroupIDs, uuid.New())
	if got := issueFields(max); len(got) != 1 || got[0] != "modifier_group_ids" {
		t.Errorf("11 grup: %v", got)
	}
}

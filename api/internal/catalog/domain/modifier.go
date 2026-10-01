package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxModifiers = 50 // opsi per grup; juga batas max_select
)

type Modifier struct {
	ID         uuid.UUID
	Name       string
	PriceDelta int64
	IsDefault  bool
	IsActive   bool
}

type ModifierGroup struct {
	ID        uuid.UUID
	Name      string
	MinSelect int
	MaxSelect int
	Version   int
	Modifiers []Modifier // urut sesuai sort_order
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (g ModifierGroup) IsRequired() bool { return g.MinSelect >= 1 }

type ModifierInput struct {
	ID         *uuid.UUID
	NewID      uuid.UUID
	Name       string
	PriceDelta int64
	IsDefault  bool
	IsActive   bool
}

type ModifierGroupInput struct {
	Name      string
	MinSelect int
	MaxSelect int
	Modifiers []ModifierInput
}

func (in ModifierGroupInput) Validate() (ModifierGroupInput, error) {
	var issues []Issue
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > MaxNameLen {
		issues = append(issues, Issue{"name", "Nama grup wajib diisi (maks 100 karakter)"})
	}
	if in.MinSelect < 0 || in.MinSelect > MaxModifiers {
		issues = append(issues, Issue{"min_select", fmt.Sprintf("Minimal pilihan harus antara 0 dan %d", MaxModifiers)})
	}
	if in.MaxSelect < 1 || in.MaxSelect > MaxModifiers {
		issues = append(issues, Issue{"max_select", fmt.Sprintf("Maksimal pilihan harus antara 1 dan %d", MaxModifiers)})
	} else if in.MaxSelect < in.MinSelect {
		issues = append(issues, Issue{"max_select", "Maksimal pilihan tidak boleh kurang dari minimal"})
	}
	if n := len(in.Modifiers); n < 1 || n > MaxModifiers {
		issues = append(issues, Issue{"modifiers", fmt.Sprintf("Grup harus memiliki 1 sampai %d opsi", MaxModifiers)})
		return in, &ValidationError{Issues: issues}
	}
	mods, mIssues := validateModifiers(in.Modifiers)
	in.Modifiers = mods
	issues = append(issues, mIssues...)
	issues = append(issues, validateSelectionRule(in)...)
	if len(issues) > 0 {
		return in, &ValidationError{Issues: issues}
	}
	return in, nil
}

func validateModifiers(in []ModifierInput) ([]ModifierInput, []Issue) {
	var issues []Issue
	out := make([]ModifierInput, len(in))
	names, ids := map[string]bool{}, map[uuid.UUID]bool{}
	for i, m := range in {
		path := fmt.Sprintf("modifiers[%d]", i)
		m.Name = strings.TrimSpace(m.Name)
		switch key := strings.ToLower(m.Name); {
		case m.Name == "" || utf8.RuneCountInString(m.Name) > MaxNameLen:
			issues = append(issues, Issue{path + ".name", "Nama opsi wajib diisi (maks 100 karakter)"})
		case names[key]:
			issues = append(issues, Issue{path + ".name", "Nama opsi harus unik dalam grup"})
		default:
			names[key] = true
		}
		if m.PriceDelta < 0 || m.PriceDelta > MaxPrice {
			issues = append(issues, Issue{path + ".price_delta", fmt.Sprintf("Tambahan harga harus antara 0 dan %d", MaxPrice)})
		}
		if m.IsDefault && !m.IsActive {
			issues = append(issues, Issue{path + ".is_default", "Opsi default harus aktif"})
		}
		if m.ID != nil {
			if ids[*m.ID] {
				issues = append(issues, Issue{path + ".id", "Opsi dikirim lebih dari sekali"})
			}
			ids[*m.ID] = true
		}
		out[i] = m
	}
	return out, issues
}

func validateSelectionRule(in ModifierGroupInput) []Issue {
	var issues []Issue
	active, defaults := 0, 0
	for _, m := range in.Modifiers {
		if m.IsActive {
			active++
		}
		if m.IsDefault {
			defaults++
		}
	}
	if in.MinSelect > active {
		issues = append(issues, Issue{"min_select", "Minimal pilihan melebihi jumlah opsi aktif"})
	}
	if in.MaxSelect >= 1 && defaults > in.MaxSelect {
		issues = append(issues, Issue{"modifiers", "Jumlah opsi default melebihi maksimal pilihan"})
	}
	return issues
}

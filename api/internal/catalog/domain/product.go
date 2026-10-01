package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	MaxPrice           int64 = 1_000_000_000
	MaxBarcodes              = 10
	MaxVariants              = 20
	MaxModifierGroups        = 10 // grup modifier per produk
	MaxVariantNameLen        = 50
	DefaultVariantName       = "Default"
)

var (
	codePattern    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`) // SKU dan barcode
	stationPattern = regexp.MustCompile(`^[a-z0-9_-]{1,30}$`)
)

type Variant struct {
	ID        uuid.UUID
	Name      string
	SKU       string
	Barcodes  []string
	CostPrice int64
	SellPrice int64
	IsDefault bool
	IsActive  bool
}

type Product struct {
	ID             uuid.UUID
	Name           string
	Type           string
	CategoryID     *uuid.UUID
	Taxable        bool
	TrackStock     bool
	KitchenStation string
	IsActive       bool
	Version        int
	Variants       []Variant
	ModifierGroupIDs []uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type VariantInput struct {
	ID        *uuid.UUID
	NewID     uuid.UUID
	Name      string
	SKU       string // kosong = dibuat otomatis (varian baru) atau dipertahankan (varian lama)
	Barcodes  []string
	CostPrice int64
	SellPrice int64
	IsActive  bool
}

type ProductInput struct {
	Name           string
	CategoryID     *uuid.UUID
	Taxable        bool
	TrackStock     bool
	IsActive       bool
	KitchenStation string
	Variants       []VariantInput
	ModifierGroupIDs []uuid.UUID
}

func ProductType(variantCount int) string {
	if variantCount > 1 {
		return "variant"
	}
	return "simple"
}

func (in ProductInput) Validate() (ProductInput, error) {
	var issues []Issue
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > MaxNameLen {
		issues = append(issues, Issue{"name", "Nama produk wajib diisi (maks 100 karakter)"})
	}
	in.KitchenStation = strings.ToLower(strings.TrimSpace(in.KitchenStation))
	if in.KitchenStation != "" && !stationPattern.MatchString(in.KitchenStation) {
		issues = append(issues, Issue{"kitchen_station", "Station hanya huruf kecil, angka, - dan _ (maks 30)"})
	}
	n := len(in.Variants)
	if n < 1 || n > MaxVariants {
		issues = append(issues, Issue{"variants", fmt.Sprintf("Produk harus memiliki 1 sampai %d varian", MaxVariants)})
		return in, &ValidationError{Issues: issues}
	}
	variants, vIssues := validateVariants(in.Variants)
	in.Variants = variants
	issues = append(issues, vIssues...)
	issues = append(issues, validateModifierGroupIDs(in.ModifierGroupIDs)...)
	if len(issues) > 0 {
		return in, &ValidationError{Issues: issues}
	}
	return in, nil
}

func validateVariants(in []VariantInput) ([]VariantInput, []Issue) {
	var issues []Issue
	out := make([]VariantInput, len(in))
	names, skus, codes, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[uuid.UUID]bool{}
	active := 0
	for i, raw := range in {
		path := fmt.Sprintf("variants[%d]", i)
		v, vIssues := validateVariant(raw, path, len(in) == 1)
		issues = append(issues, vIssues...)
		if len(in) > 1 && v.Name != "" {
			if key := strings.ToLower(v.Name); names[key] {
				issues = append(issues, Issue{path + ".name", "Nama varian harus unik"})
			} else {
				names[key] = true
			}
		}
		if key := strings.ToLower(v.SKU); v.SKU != "" {
			if skus[key] {
				issues = append(issues, Issue{path + ".sku", "SKU dipakai varian lain pada produk ini"})
			}
			skus[key] = true
		}
		for _, b := range v.Barcodes {
			if codes[b] {
				issues = append(issues, Issue{path + ".barcodes", "Barcode dipakai varian lain pada produk ini"})
				break
			}
		}
		for _, b := range v.Barcodes {
			codes[b] = true
		}
		if v.ID != nil {
			if ids[*v.ID] {
				issues = append(issues, Issue{path + ".id", "Varian dikirim lebih dari sekali"})
			}
			ids[*v.ID] = true
		}
		if v.IsActive {
			active++
		}
		out[i] = v
	}
	if active == 0 {
		issues = append(issues, Issue{"variants", "Minimal satu varian harus aktif"})
	}
	return out, issues
}

func validateModifierGroupIDs(ids []uuid.UUID) []Issue {
	if len(ids) > MaxModifierGroups {
		return []Issue{{"modifier_group_ids", fmt.Sprintf("Maksimal %d grup modifier per produk", MaxModifierGroups)}}
	}
	var issues []Issue
	seen := make(map[uuid.UUID]bool, len(ids))
	for i, id := range ids {
		if seen[id] {
			issues = append(issues, Issue{fmt.Sprintf("modifier_group_ids[%d]", i), "Grup modifier dipilih lebih dari sekali"})
		}
		seen[id] = true
	}
	return issues
}

func validateVariant(v VariantInput, path string, single bool) (VariantInput, []Issue) {
	var issues []Issue
	field := func(name string) string { return path + "." + name }

	v.Name = strings.TrimSpace(v.Name)
	switch {
	case v.Name == "" && single:
		v.Name = DefaultVariantName
	case v.Name == "":
		issues = append(issues, Issue{field("name"), "Nama varian wajib diisi"})
	case utf8.RuneCountInString(v.Name) > MaxVariantNameLen:
		issues = append(issues, Issue{field("name"), "Nama varian maks 50 karakter"})
	}
	v.SKU = strings.TrimSpace(v.SKU)
	if v.SKU != "" && !codePattern.MatchString(v.SKU) {
		issues = append(issues, Issue{field("sku"), "SKU hanya huruf, angka, titik, - dan _ (maks 64)"})
	}
	for _, p := range []struct {
		name string
		val  int64
	}{{"cost_price", v.CostPrice}, {"sell_price", v.SellPrice}} {
		if p.val < 0 || p.val > MaxPrice {
			issues = append(issues, Issue{field(p.name), fmt.Sprintf("Harga harus antara 0 dan %d", MaxPrice)})
		}
	}
	barcodes, bIssue := normalizeBarcodes(v.Barcodes)
	if bIssue != "" {
		issues = append(issues, Issue{field("barcodes"), bIssue})
	}
	v.Barcodes = barcodes
	return v, issues
}

func normalizeBarcodes(in []string) ([]string, string) {
	if len(in) > MaxBarcodes {
		return in, fmt.Sprintf("Maksimal %d barcode", MaxBarcodes)
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]bool, len(in))
	for _, b := range in {
		b = strings.TrimSpace(b)
		if !codePattern.MatchString(b) {
			return in, "Barcode hanya huruf, angka, titik, - dan _ (maks 64)"
		}
		if seen[b] {
			return in, "Barcode tidak boleh duplikat"
		}
		seen[b] = true
		out = append(out, b)
	}
	return out, ""
}

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
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type VariantInput struct {
	Name      string
	SKU       string // kosong = dibuat otomatis (create) atau dipertahankan (update)
	Barcodes  []string
	CostPrice int64
	SellPrice int64
}

type ProductInput struct {
	Name           string
	CategoryID     *uuid.UUID
	Taxable        bool
	TrackStock     bool
	IsActive       bool
	KitchenStation string
	Variants       []VariantInput
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
	if len(in.Variants) != 1 {
		issues = append(issues, Issue{"variants", "Produk sederhana harus memiliki tepat satu varian"})
		return in, &ValidationError{Issues: issues}
	}
	v, vIssues := validateVariant(in.Variants[0], "variants[0]")
	in.Variants = []VariantInput{v}
	issues = append(issues, vIssues...)
	if len(issues) > 0 {
		return in, &ValidationError{Issues: issues}
	}
	return in, nil
}

func validateVariant(v VariantInput, path string) (VariantInput, []Issue) {
	var issues []Issue
	field := func(name string) string { return path + "." + name }

	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" {
		v.Name = DefaultVariantName
	}
	if utf8.RuneCountInString(v.Name) > MaxVariantNameLen {
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

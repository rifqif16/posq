package application

import (
	"encoding/csv"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

const (
	MaxCSVRows  = 2000
	MaxCSVBytes = 2 << 20
	csvBOM      = "\xef\xbb\xbf"
	formulaLead = "=+-@\t\r"
)

var CSVColumns = []string{
	"product_name", "category", "kitchen_station", "taxable", "track_stock", "is_active", "modifier_groups",
	"variant_name", "sku", "barcodes", "cost_price", "sell_price", "variant_is_active",
}

var requiredCSVColumns = []string{"product_name", "sell_price"}

type ExportMaps struct {
	Categories map[uuid.UUID]string
	Groups     map[uuid.UUID]string
}

func sanitizeCell(s string) string {
	if s != "" && strings.ContainsRune(formulaLead, rune(s[0])) {
		return "'" + s
	}
	return s
}

func unsanitizeCell(s string) string {
	if len(s) >= 2 && s[0] == '\'' && strings.ContainsRune(formulaLead, rune(s[1])) {
		return s[1:]
	}
	return s
}

func categoryPaths(cats []domain.Category) map[uuid.UUID]string {
	names := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		names[c.ID] = c.Name
	}
	paths := make(map[uuid.UUID]string, len(cats))
	for _, c := range cats {
		if c.ParentID != nil {
			paths[c.ID] = names[*c.ParentID] + " > " + c.Name
		} else {
			paths[c.ID] = c.Name
		}
	}
	return paths
}

func normalizePath(path string) string {
	parts := strings.Split(path, ">")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.ToLower(strings.Join(parts, " > "))
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func WriteProductsCSV(w io.Writer, products []domain.Product, m ExportMaps, delimiter rune) error {
	if _, err := io.WriteString(w, csvBOM); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma = delimiter
	if err := cw.Write(CSVColumns); err != nil {
		return err
	}
	for _, p := range products {
		groupNames := make([]string, 0, len(p.ModifierGroupIDs))
		for _, id := range p.ModifierGroupIDs {
			groupNames = append(groupNames, m.Groups[id])
		}
		category := ""
		if p.CategoryID != nil {
			category = m.Categories[*p.CategoryID]
		}
		for _, v := range p.Variants {
			row := []string{
				sanitizeCell(p.Name), sanitizeCell(category), sanitizeCell(p.KitchenStation),
				boolText(p.Taxable), boolText(p.TrackStock), boolText(p.IsActive), sanitizeCell(strings.Join(groupNames, "|")),
				sanitizeCell(v.Name), sanitizeCell(v.SKU), sanitizeCell(strings.Join(v.Barcodes, "|")),
				itoa(v.CostPrice), itoa(v.SellPrice), boolText(v.IsActive),
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

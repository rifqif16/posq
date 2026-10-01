package application

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

const maxReportedErrors = 200

var (
	moneyPattern = regexp.MustCompile(`^\d{1,13}$`)
	variantIssue = regexp.MustCompile(`^variants\[(\d+)\]\.(\w+)$`)
)

var variantColumns = map[string]string{
	"name": "variant_name", "id": "variant_name", "sku": "sku", "barcodes": "barcodes",
	"cost_price": "cost_price", "sell_price": "sell_price",
}

type RowError struct {
	Row     int
	Column  string
	Message string
}

type ImportReport struct {
	Valid      bool
	Products   int
	Variants   int
	Created    int
	ErrorCount int
	Errors     []RowError
}

type CodeCandidates struct {
	SKUs     []string
	Barcodes []string
	Names    []string
}

type TakenCodes struct {
	SKUs     map[string]bool
	Barcodes map[string]bool
	Names    map[string]bool
}

type CSVRepository interface {
	Repository
	ModifierRepository
	ProductRepository
	FindTakenProductCodes(ctx context.Context, tenantID uuid.UUID, c CodeCandidates) (TakenCodes, error)
	CreateProducts(ctx context.Context, a Actor, products []NewProduct, now time.Time) error
}

type ProductCSVService struct {
	repo CSVRepository
	now  func() time.Time
}

func NewProductCSVService(repo CSVRepository) *ProductCSVService {
	return &ProductCSVService{repo: repo, now: time.Now}
}

type csvRow struct {
	line  int
	cells map[string]string
}

type csvLookups struct {
	categories map[string]uuid.UUID
	groups     map[string]uuid.UUID
}

type plannedProduct struct {
	input domain.ProductInput
	rows  []int
}

func detectDelimiter(data []byte) rune {
	first := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		first = data[:i]
	}
	if bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		return ';'
	}
	return ','
}

func readCSVRows(data []byte) ([]csvRow, []RowError) {
	data = bytes.TrimPrefix(data, []byte(csvBOM))
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, []RowError{{Row: 0, Message: "File kosong"}}
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectDelimiter(data)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, []RowError{parseFailure(err, 1)}
	}
	index := make(map[string]int, len(header))
	for i, h := range header {
		index[strings.ToLower(strings.TrimSpace(h))] = i
	}
	var errs []RowError
	for _, col := range requiredCSVColumns {
		if _, ok := index[col]; !ok {
			errs = append(errs, RowError{Row: 1, Column: col, Message: "Kolom wajib tidak ditemukan"})
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	var rows []csvRow
	for {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			return rows, nil
		}
		if err != nil {
			return nil, []RowError{parseFailure(err, 0)}
		}
		line, _ := r.FieldPos(0)
		cells := make(map[string]string, len(index))
		blank := true
		for col, i := range index {
			if i >= len(record) {
				continue
			}
			v := strings.TrimSpace(unsanitizeCell(strings.TrimSpace(record[i])))
			cells[col] = v
			if v != "" {
				blank = false
			}
		}
		if blank {
			continue
		}
		if len(rows) >= MaxCSVRows {
			return nil, []RowError{{Row: line, Message: fmt.Sprintf("Maksimal %d baris data per file", MaxCSVRows)}}
		}
		rows = append(rows, csvRow{line: line, cells: cells})
	}
}

func parseFailure(err error, fallbackLine int) RowError {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return RowError{Row: pe.Line, Message: "Format CSV tidak valid"}
	}
	return RowError{Row: fallbackLine, Message: "Format CSV tidak valid"}
}

func parseBool(cell string, def bool) (bool, bool) {
	switch strings.ToLower(cell) {
	case "":
		return def, true
	case "true", "ya", "yes", "1":
		return true, true
	case "false", "tidak", "no", "0":
		return false, true
	}
	return false, false
}

func splitList(cell string) []string {
	var out []string
	for _, part := range strings.Split(cell, "|") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

type productGroup struct {
	rows []csvRow
}

func groupRows(rows []csvRow) ([]productGroup, []RowError) {
	var groups []productGroup
	index := map[string]int{}
	var errs []RowError
	for _, r := range rows {
		name := r.cells["product_name"]
		if name == "" {
			errs = append(errs, RowError{Row: r.line, Column: "product_name", Message: "Nama produk wajib diisi pada setiap baris"})
			continue
		}
		key := strings.ToLower(name)
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, productGroup{})
		}
		groups[i].rows = append(groups[i].rows, r)
	}
	return groups, errs
}

func buildProduct(g productGroup, lk csvLookups) (plannedProduct, []RowError) {
	first := g.rows[0]
	var errs []RowError
	add := func(r csvRow, col, msg string) { errs = append(errs, RowError{Row: r.line, Column: col, Message: msg}) }
	boolOr := func(r csvRow, col string, def bool) bool {
		v, ok := parseBool(r.cells[col], def)
		if !ok {
			add(r, col, "Nilai harus true/false (atau ya/tidak, 1/0)")
		}
		return v
	}

	in := domain.ProductInput{
		Name:           first.cells["product_name"],
		KitchenStation: first.cells["kitchen_station"],
		Taxable:        boolOr(first, "taxable", true),
		TrackStock:     boolOr(first, "track_stock", false),
		IsActive:       boolOr(first, "is_active", true),
	}
	if c := first.cells["category"]; c != "" {
		if id, ok := lk.categories[normalizePath(c)]; ok {
			in.CategoryID = &id
		} else {
			add(first, "category", "Kategori tidak ditemukan: "+c)
		}
	}
	for _, name := range splitList(first.cells["modifier_groups"]) {
		if id, ok := lk.groups[strings.ToLower(name)]; ok {
			in.ModifierGroupIDs = append(in.ModifierGroupIDs, id)
		} else {
			add(first, "modifier_groups", "Grup modifier tidak ditemukan: "+name)
		}
	}

	lines := make([]int, len(g.rows))
	for i, r := range g.rows {
		lines[i] = r.line
		v := domain.VariantInput{
			Name: r.cells["variant_name"], SKU: r.cells["sku"], Barcodes: splitList(r.cells["barcodes"]),
			IsActive: boolOr(r, "variant_is_active", true),
		}
		v.CostPrice = moneyOr(r, "cost_price", false, &errs)
		v.SellPrice = moneyOr(r, "sell_price", true, &errs)
		in.Variants = append(in.Variants, v)
	}
	if len(errs) > 0 {
		return plannedProduct{}, errs
	}

	validated, err := in.Validate()
	var ve *domain.ValidationError
	if errors.As(err, &ve) {
		for _, is := range ve.Issues {
			errs = append(errs, issueToRowError(is, lines))
		}
		return plannedProduct{}, errs
	}
	return plannedProduct{input: validated, rows: lines}, nil
}

func moneyOr(r csvRow, col string, required bool, errs *[]RowError) int64 {
	cell := r.cells[col]
	if cell == "" {
		if required {
			*errs = append(*errs, RowError{Row: r.line, Column: col, Message: "Harga jual wajib diisi"})
		}
		return 0
	}
	if !moneyPattern.MatchString(cell) {
		*errs = append(*errs, RowError{Row: r.line, Column: col, Message: "Harga harus bilangan bulat tanpa pemisah (contoh 15000)"})
		return 0
	}
	n, _ := strconv.ParseInt(cell, 10, 64)
	return n
}

func issueToRowError(is domain.Issue, lines []int) RowError {
	if m := variantIssue.FindStringSubmatch(is.Field); m != nil {
		i, _ := strconv.Atoi(m[1])
		return RowError{Row: lines[i], Column: variantColumns[m[2]], Message: is.Message}
	}
	col := ""
	switch {
	case is.Field == "name":
		col = "product_name"
	case is.Field == "kitchen_station":
		col = "kitchen_station"
	case is.Field == "variants":
		col = "variant_is_active"
	case strings.HasPrefix(is.Field, "modifier_group_ids"):
		col = "modifier_groups"
	}
	return RowError{Row: lines[0], Column: col, Message: is.Message}
}

func planProducts(groups []productGroup, lk csvLookups) ([]plannedProduct, []RowError) {
	var plan []plannedProduct
	var errs []RowError
	skus, codes := map[string]int{}, map[string]int{}
	for _, g := range groups {
		p, perrs := buildProduct(g, lk)
		if len(perrs) > 0 {
			errs = append(errs, perrs...)
			continue
		}
		errs = append(errs, duplicateErrors(p, skus, codes)...)
		plan = append(plan, p)
	}
	return plan, errs
}

func duplicateErrors(p plannedProduct, skus, codes map[string]int) []RowError {
	var errs []RowError
	for i, v := range p.input.Variants {
		row := p.rows[i]
		if v.SKU != "" {
			key := strings.ToLower(v.SKU)
			if prev, ok := skus[key]; ok {
				errs = append(errs, RowError{Row: row, Column: "sku", Message: fmt.Sprintf("SKU sama dengan baris %d", prev)})
			} else {
				skus[key] = row
			}
		}
		for _, b := range v.Barcodes {
			if prev, ok := codes[b]; ok {
				errs = append(errs, RowError{Row: row, Column: "barcodes", Message: fmt.Sprintf("Barcode %s sama dengan baris %d", b, prev)})
			} else {
				codes[b] = row
			}
		}
	}
	return errs
}

func candidatesOf(plan []plannedProduct) CodeCandidates {
	var c CodeCandidates
	for _, p := range plan {
		c.Names = append(c.Names, strings.ToLower(p.input.Name))
		for _, v := range p.input.Variants {
			if v.SKU != "" {
				c.SKUs = append(c.SKUs, strings.ToLower(v.SKU))
			}
			c.Barcodes = append(c.Barcodes, v.Barcodes...)
		}
	}
	return c
}

func takenErrors(plan []plannedProduct, taken TakenCodes) []RowError {
	var errs []RowError
	for _, p := range plan {
		if taken.Names[strings.ToLower(p.input.Name)] {
			errs = append(errs, RowError{Row: p.rows[0], Column: "product_name", Message: "Produk dengan nama ini sudah ada"})
		}
		for i, v := range p.input.Variants {
			if v.SKU != "" && taken.SKUs[strings.ToLower(v.SKU)] {
				errs = append(errs, RowError{Row: p.rows[i], Column: "sku", Message: "SKU sudah dipakai produk lain"})
			}
			for _, b := range v.Barcodes {
				if taken.Barcodes[b] {
					errs = append(errs, RowError{Row: p.rows[i], Column: "barcodes", Message: "Barcode sudah dipakai: " + b})
				}
			}
		}
	}
	return errs
}

func newLookups(cats []domain.Category, groups []domain.ModifierGroup) csvLookups {
	lk := csvLookups{categories: map[string]uuid.UUID{}, groups: map[string]uuid.UUID{}}
	for id, path := range categoryPaths(cats) {
		lk.categories[normalizePath(path)] = id
	}
	for _, g := range groups {
		lk.groups[strings.ToLower(g.Name)] = g.ID
	}
	return lk
}

func finishReport(rep *ImportReport, errs []RowError) {
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Row != errs[j].Row {
			return errs[i].Row < errs[j].Row
		}
		return errs[i].Column < errs[j].Column
	})
	rep.ErrorCount = len(errs)
	rep.Valid = len(errs) == 0
	if len(errs) > maxReportedErrors {
		errs = errs[:maxReportedErrors]
	}
	rep.Errors = errs
}

func (s *ProductCSVService) Import(ctx context.Context, a Actor, data []byte, commit bool) (ImportReport, error) {
	rep := ImportReport{Errors: []RowError{}}
	rows, errs := readCSVRows(data)
	if len(errs) > 0 {
		finishReport(&rep, errs)
		return rep, nil
	}
	cats, err := s.repo.ListCategories(ctx, a.TenantID)
	if err != nil {
		return rep, err
	}
	groups, err := s.repo.ListModifierGroups(ctx, a.TenantID)
	if err != nil {
		return rep, err
	}
	productGroups, errs := groupRows(rows)
	rep.Products, rep.Variants = len(productGroups), len(rows)

	plan, planErrs := planProducts(productGroups, newLookups(cats, groups))
	errs = append(errs, planErrs...)
	if len(plan) > 0 {
		taken, err := s.repo.FindTakenProductCodes(ctx, a.TenantID, candidatesOf(plan))
		if err != nil {
			return rep, err
		}
		errs = append(errs, takenErrors(plan, taken)...)
	}
	finishReport(&rep, errs)
	if !commit || !rep.Valid {
		return rep, nil
	}

	products := make([]NewProduct, len(plan))
	for i, p := range plan {
		id, err := uuid.NewV7()
		if err != nil {
			return rep, fmt.Errorf("buat id: %w", err)
		}
		if err := assignNewVariants(p.input.Variants); err != nil {
			return rep, err
		}
		products[i] = NewProduct{ID: id, Input: p.input}
	}
	if err := s.repo.CreateProducts(ctx, a, products, s.now()); err != nil {
		return rep, err
	}
	rep.Created = len(products)
	return rep, nil
}

func (s *ProductCSVService) Export(ctx context.Context, a Actor, w io.Writer, delimiter rune) error {
	cats, err := s.repo.ListCategories(ctx, a.TenantID)
	if err != nil {
		return err
	}
	groups, err := s.repo.ListModifierGroups(ctx, a.TenantID)
	if err != nil {
		return err
	}
	groupNames := make(map[uuid.UUID]string, len(groups))
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}

	var products []domain.Product
	f := ProductFilter{Limit: maxPageSize}
	for {
		page, err := s.repo.ListProducts(ctx, a.TenantID, f)
		if err != nil {
			return err
		}
		products = append(products, page.Items...)
		if !page.HasMore || len(page.Items) == 0 {
			break
		}
		key, id := page.LastKey, page.Items[len(page.Items)-1].ID
		f.AfterKey, f.AfterID = &key, &id
	}
	return WriteProductsCSV(w, products, ExportMaps{Categories: categoryPaths(cats), Groups: groupNames}, delimiter)
}

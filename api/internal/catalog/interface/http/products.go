package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

type variantRequest struct {
	ID        *uuid.UUID `json:"id"`        // varian yang sudah ada (PATCH); kosong = varian baru
	IsActive  *bool      `json:"is_active"` // default true
	Name      string     `json:"name"`
	SKU       string     `json:"sku"`
	Barcodes  []string   `json:"barcodes"`
	CostPrice int64      `json:"cost_price"`
	SellPrice int64      `json:"sell_price"`
}

type productRequest struct {
	Name           string           `json:"name"`
	CategoryID     *uuid.UUID       `json:"category_id"`
	Taxable        *bool            `json:"taxable"` // default true
	TrackStock     bool             `json:"track_stock"`
	IsActive       *bool            `json:"is_active"` // default true
	KitchenStation string           `json:"kitchen_station"`
	Variants       []variantRequest `json:"variants"`
	ModifierGroupIDs []uuid.UUID `json:"modifier_group_ids"`
}

func (p productRequest) toInput() domain.ProductInput {
	in := domain.ProductInput{
		Name: p.Name, CategoryID: p.CategoryID, Taxable: true, TrackStock: p.TrackStock,
		IsActive: true, KitchenStation: p.KitchenStation, ModifierGroupIDs: p.ModifierGroupIDs,
	}
	if p.Taxable != nil {
		in.Taxable = *p.Taxable
	}
	if p.IsActive != nil {
		in.IsActive = *p.IsActive
	}
	for _, v := range p.Variants {
		active := true
		if v.IsActive != nil {
			active = *v.IsActive
		}
		in.Variants = append(in.Variants, domain.VariantInput{
			ID: v.ID, Name: v.Name, SKU: v.SKU, Barcodes: v.Barcodes, CostPrice: v.CostPrice, SellPrice: v.SellPrice, IsActive: active,
		})
	}
	return in
}

type variantDTO struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	SKU       string    `json:"sku"`
	Barcodes  []string  `json:"barcodes"`
	CostPrice *int64    `json:"cost_price"` // null bila pemanggil tidak punya product:cost_price_read
	SellPrice int64     `json:"sell_price"`
	IsDefault bool      `json:"is_default"`
	IsActive  bool      `json:"is_active"`
}

type productDTO struct {
	ID             uuid.UUID    `json:"id"`
	Name           string       `json:"name"`
	Type           string       `json:"type"`
	CategoryID     *uuid.UUID   `json:"category_id"`
	Taxable        bool         `json:"taxable"`
	TrackStock     bool         `json:"track_stock"`
	KitchenStation string       `json:"kitchen_station"`
	IsActive       bool         `json:"is_active"`
	Version        int          `json:"version"`
	Variants       []variantDTO `json:"variants"`
	ModifierGroupIDs []uuid.UUID `json:"modifier_group_ids"`
}

func toProductDTO(p domain.Product, showCost bool) productDTO {
	variants := make([]variantDTO, len(p.Variants))
	for i, v := range p.Variants {
		variants[i] = variantDTO{ID: v.ID, Name: v.Name, SKU: v.SKU, Barcodes: v.Barcodes, SellPrice: v.SellPrice, IsDefault: v.IsDefault, IsActive: v.IsActive}
		if showCost {
			cost := v.CostPrice
			variants[i].CostPrice = &cost
		}
	}
	return productDTO{
		ID: p.ID, Name: p.Name, Type: p.Type, CategoryID: p.CategoryID, Taxable: p.Taxable,
		TrackStock: p.TrackStock, KitchenStation: p.KitchenStation, IsActive: p.IsActive,
		Version: p.Version, Variants: variants, ModifierGroupIDs: groupIDs(p.ModifierGroupIDs),
	}
}

func groupIDs(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

func writeProduct(w http.ResponseWriter, r *http.Request, status int, p domain.Product) {
	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, p.Version))
	httpx.WriteJSON(w, status, toProductDTO(p, principal(r).Has(permProductCostRead)))
}

func (h *Handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeProductError(w, r, err)
		return
	}
	p, err := h.products.Create(r.Context(), actor(r), req.toInput())
	if err != nil {
		h.writeProductError(w, r, err)
		return
	}
	writeProduct(w, r, http.StatusCreated, p)
}

func (h *Handler) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := h.products.Get(r.Context(), actor(r), id)
	if err != nil {
		h.writeProductError(w, r, err)
		return
	}
	writeProduct(w, r, http.StatusOK, p)
}

func (h *Handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	version, ok := parseIfMatch(r.Header.Get("If-Match"))
	if !ok {
		httpx.WriteProblem(w, r, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "Header If-Match dengan versi produk wajib diisi")
		return
	}
	var req productRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeProductError(w, r, err)
		return
	}
	p, err := h.products.Update(r.Context(), actor(r), id, version, req.toInput())
	if err != nil {
		h.writeProductError(w, r, err)
		return
	}
	writeProduct(w, r, http.StatusOK, p)
}

func (h *Handler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.products.Delete(r.Context(), actor(r), id); err != nil {
		h.writeProductError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var categoryID *uuid.UUID
	if raw := q.Get("category_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "category_id tidak valid")
			return
		}
		categoryID = &id
	}
	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit tidak valid")
			return
		}
		limit = n
	}
	res, err := h.products.List(r.Context(), actor(r), q.Get("q"), categoryID, limit, q.Get("cursor"))
	if err != nil {
		h.writeProductError(w, r, err)
		return
	}
	showCost := principal(r).Has(permProductCostRead)
	items := make([]productDTO, len(res.Items))
	for i, p := range res.Items {
		items[i] = toProductDTO(p, showCost)
	}
	var next *string
	if res.NextCursor != "" {
		next = &res.NextCursor
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

func (h *Handler) writeProductError(w http.ResponseWriter, r *http.Request, err error) {
	var iv *application.InvalidVariantError
	var ig *application.InvalidModifierGroupError
	switch {
	case errors.As(err, &iv):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "INVALID_VARIANT", "Varian tidak valid",
			httpx.FieldError{Field: fmt.Sprintf("variants[%d].id", iv.Index), Message: "Varian tidak ditemukan pada produk ini"})
	case errors.As(err, &ig):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "INVALID_MODIFIER_GROUP", "Grup modifier tidak valid",
			httpx.FieldError{Field: fmt.Sprintf("modifier_group_ids[%d]", ig.Index), Message: "Grup modifier tidak ditemukan"})
	default:
		h.writeError(w, r, err)
	}
}

func parseIfMatch(header string) (int, bool) {
	v := strings.Trim(strings.TrimPrefix(strings.TrimSpace(header), "W/"), `"`)
	n, err := strconv.Atoi(v)
	return n, err == nil && n >= 1
}

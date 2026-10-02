package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/inventory/application"
	"github.com/rifqif16/posq/api/internal/inventory/domain"
	"github.com/rifqif16/posq/api/internal/platform/authn"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

const (
	permProductRead     = "product:read"
	permInventoryAdjust = "inventory:adjust"
	permInventoryCount  = "inventory:opname"
)

type Guard func(permission string) func(http.Handler) http.Handler

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func NewHandler(svc *application.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Mount(r chi.Router, require Guard) {
	r.With(require(permProductRead)).Get("/inventory/levels", h.levels)
	r.With(require(permInventoryAdjust)).Get("/inventory/movements", h.movements)
	r.With(require(permInventoryAdjust)).Post("/inventory/movements", h.recordMovement)
	r.With(require(permInventoryCount)).Post("/inventory/counts", h.applyCounts)
}

func actor(r *http.Request) application.Actor {
	p, _ := authn.From(r.Context())
	return application.Actor{TenantID: p.TenantID, UserID: p.UserID}
}

type movementRequest struct {
	StoreID   uuid.UUID `json:"store_id"`
	VariantID uuid.UUID `json:"variant_id"`
	Type      string    `json:"type"`
	QtyDelta  string    `json:"qty_delta"`
	UnitCost  *int64    `json:"unit_cost"`
	Reason    string    `json:"reason"`
}

type countItemRequest struct {
	VariantID  uuid.UUID `json:"variant_id"`
	CountedQty string    `json:"counted_qty"`
}

type countRequest struct {
	StoreID uuid.UUID          `json:"store_id"`
	Reason  string             `json:"reason"`
	Items   []countItemRequest `json:"items"`
}

type createdByDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type movementDTO struct {
	ID          uuid.UUID    `json:"id"`
	VariantID   uuid.UUID    `json:"variant_id"`
	ProductName string       `json:"product_name"`
	VariantName string       `json:"variant_name"`
	SKU         string       `json:"sku"`
	Type        string       `json:"type"`
	QtyDelta    string       `json:"qty_delta"`
	UnitCost    *int64       `json:"unit_cost"`
	Reason      string       `json:"reason"`
	CreatedBy   createdByDTO `json:"created_by"`
	CreatedAt   time.Time    `json:"created_at"`
}

type levelDTO struct {
	VariantID   uuid.UUID `json:"variant_id"`
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name"`
	VariantName string    `json:"variant_name"`
	SKU         string    `json:"sku"`
	QtyOnHand   string    `json:"qty_on_hand"`
}

type countResultDTO struct {
	VariantID uuid.UUID `json:"variant_id"`
	Before    string    `json:"before"`
	Counted   string    `json:"counted"`
	Delta     string    `json:"delta"`
}

func toMovementDTO(m domain.Movement) movementDTO {
	return movementDTO{
		ID: m.ID, VariantID: m.VariantID, ProductName: m.ProductName, VariantName: m.VariantName, SKU: m.SKU,
		Type: string(m.Type), QtyDelta: domain.FormatQty(m.QtyDelta), UnitCost: m.UnitCost, Reason: m.Reason,
		CreatedBy: createdByDTO{ID: m.UserID, Name: m.UserName}, CreatedAt: m.CreatedAt,
	}
}

func queryStore(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.URL.Query().Get("store_id"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "store_id wajib berupa UUID")
		return uuid.Nil, false
	}
	return id, true
}

func queryLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit tidak valid")
		return 0, false
	}
	return n, true
}

func (h *Handler) levels(w http.ResponseWriter, r *http.Request) {
	store, ok := queryStore(w, r)
	if !ok {
		return
	}
	limit, ok := queryLimit(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	res, err := h.svc.Levels(r.Context(), actor(r), store, q.Get("q"), limit, q.Get("cursor"))
	if err != nil {
		h.writeError(w, r, err, "variant_id")
		return
	}
	items := make([]levelDTO, len(res.Items))
	for i, l := range res.Items {
		items[i] = levelDTO{
			VariantID: l.VariantID, ProductID: l.ProductID, ProductName: l.ProductName, VariantName: l.VariantName,
			SKU: l.SKU, QtyOnHand: domain.FormatQty(l.QtyOnHand),
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nextCursor(res.NextCursor)})
}

func (h *Handler) movements(w http.ResponseWriter, r *http.Request) {
	store, ok := queryStore(w, r)
	if !ok {
		return
	}
	limit, ok := queryLimit(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var variantID *uuid.UUID
	if raw := q.Get("variant_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "variant_id tidak valid")
			return
		}
		variantID = &id
	}
	typ := q.Get("type")
	if typ != "" && !domain.KnownType(typ) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "type tidak dikenal")
		return
	}
	res, err := h.svc.Movements(r.Context(), actor(r), store, variantID, typ, limit, q.Get("cursor"))
	if err != nil {
		h.writeError(w, r, err, "variant_id")
		return
	}
	items := make([]movementDTO, len(res.Items))
	for i, m := range res.Items {
		items[i] = toMovementDTO(m)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nextCursor(res.NextCursor)})
}

func (h *Handler) recordMovement(w http.ResponseWriter, r *http.Request) {
	var req movementRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err, "variant_id")
		return
	}
	res, err := h.svc.RecordMovement(r.Context(), actor(r), domain.MovementInput{
		StoreID: req.StoreID, VariantID: req.VariantID, Type: domain.MovementType(req.Type),
		QtyDelta: req.QtyDelta, UnitCost: req.UnitCost, Reason: req.Reason,
	})
	if err != nil {
		h.writeError(w, r, err, "variant_id")
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"movement": toMovementDTO(res.Movement), "qty_on_hand": domain.FormatQty(res.QtyOnHand),
	})
}

func (h *Handler) applyCounts(w http.ResponseWriter, r *http.Request) {
	var req countRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err, "")
		return
	}
	items := make([]domain.CountItemInput, len(req.Items))
	for i, it := range req.Items {
		items[i] = domain.CountItemInput{VariantID: it.VariantID, CountedQty: it.CountedQty}
	}
	results, err := h.svc.ApplyCounts(r.Context(), actor(r), domain.CountInput{StoreID: req.StoreID, Reason: req.Reason, Items: items})
	if err != nil {
		h.writeError(w, r, err, "")
		return
	}
	out := make([]countResultDTO, len(results))
	changed := 0
	for i, c := range results {
		out[i] = countResultDTO{
			VariantID: c.VariantID, Before: domain.FormatQty(c.Before), Counted: domain.FormatQty(c.Counted), Delta: domain.FormatQty(c.Delta),
		}
		if c.Delta != 0 {
			changed++
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"results": out, "adjusted": changed})
}

func nextCursor(c string) *string {
	if c == "" {
		return nil
	}
	return &c
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error, singleField string) {
	var ve *domain.ValidationError
	var vp *application.VariantProblemError
	switch {
	case errors.As(err, &ve):
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Message}
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Data tidak valid", fields...)
	case errors.As(err, &vp):
		field := singleField
		if field == "" {
			field = fmt.Sprintf("items[%d].variant_id", vp.Index)
		}
		code, msg := "INVALID_VARIANT", "Varian tidak ditemukan"
		if vp.NotTracked {
			code, msg = "STOCK_NOT_TRACKED", "Produk ini tidak melacak stok; aktifkan \"Lacak stok\" pada produk"
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, code, msg, httpx.FieldError{Field: field, Message: msg})
	case errors.Is(err, application.ErrStoreNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "STORE_NOT_FOUND", "Outlet tidak ditemukan")
	case errors.Is(err, httpx.ErrBadBody), errors.Is(err, application.ErrInvalidCursor):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "Permintaan tidak valid")
	default:
		h.log.Error("inventory: error tak terduga", "err", err, "path", r.URL.Path)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Terjadi kesalahan pada server")
	}
}

package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

type PriceHistoryHandler struct {
	svc *application.PriceHistoryService
	log *slog.Logger
}

func NewPriceHistoryHandler(svc *application.PriceHistoryService, log *slog.Logger) *PriceHistoryHandler {
	return &PriceHistoryHandler{svc: svc, log: log}
}

func (h *PriceHistoryHandler) Mount(r chi.Router, require Guard) {
	r.With(require(permProductCostRead)).Get("/products/{id}/price-history", h.list)
}

type changedByDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type priceChangeDTO struct {
	ID          uuid.UUID    `json:"id"`
	VariantID   uuid.UUID    `json:"variant_id"`
	VariantName string       `json:"variant_name"`
	Field       string       `json:"field"`
	OldValue    int64        `json:"old_value"`
	NewValue    int64        `json:"new_value"`
	ChangedBy   changedByDTO `json:"changed_by"`
	At          time.Time    `json:"at"`
}

func toPriceChangeDTO(c domain.PriceChange) priceChangeDTO {
	return priceChangeDTO{
		ID: c.ID, VariantID: c.VariantID, VariantName: c.VariantName, Field: c.Field,
		OldValue: c.OldValue, NewValue: c.NewValue,
		ChangedBy: changedByDTO{ID: c.ChangedByID, Name: c.ChangedByName}, At: c.At,
	}
}

func (h *PriceHistoryHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := 0
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "limit tidak valid")
			return
		}
		limit = n
	}
	res, err := h.svc.List(r.Context(), actor(r), id, limit, q.Get("cursor"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]priceChangeDTO, len(res.Items))
	for i, c := range res.Items {
		items[i] = toPriceChangeDTO(c)
	}
	var next *string
	if res.NextCursor != "" {
		next = &res.NextCursor
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": next})
}

func (h *PriceHistoryHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, application.ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
	case errors.Is(err, application.ErrInvalidCursor):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "Permintaan tidak valid")
	default:
		h.log.Error("price history: error tak terduga", "err", err, "path", r.URL.Path)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Terjadi kesalahan pada server")
	}
}

// Package httpapi (catalog): entrypoint HTTP katalog. Permission dipasang lewat guard yang disuntikkan.
package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/authn"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

const (
	permProductRead  = "product:read"
	permProductWrite = "product:write"
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
	r.With(require(permProductRead)).Get("/categories", h.listCategories)
	r.With(require(permProductWrite)).Post("/categories", h.createCategory)
	r.With(require(permProductWrite)).Patch("/categories/{id}", h.updateCategory)
	r.With(require(permProductWrite)).Delete("/categories/{id}", h.deleteCategory)
}

type categoryDTO struct {
	ID        uuid.UUID  `json:"id"`
	ParentID  *uuid.UUID `json:"parent_id"`
	Name      string     `json:"name"`
	SortOrder int        `json:"sort_order"`
}

func toDTO(c domain.Category) categoryDTO {
	return categoryDTO{ID: c.ID, ParentID: c.ParentID, Name: c.Name, SortOrder: c.SortOrder}
}

type createCategoryRequest struct {
	Name      string     `json:"name"`
	ParentID  *uuid.UUID `json:"parent_id"`
	SortOrder int        `json:"sort_order"`
}

type updateCategoryRequest struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

func actor(r *http.Request) application.Actor {
	p, _ := authn.From(r.Context()) // guard menjamin principal ada
	return application.Actor{TenantID: p.TenantID, UserID: p.UserID}
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.svc.ListCategories(r.Context(), actor(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]categoryDTO, len(cats))
	for i, c := range cats {
		items[i] = toDTO(c)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var req createCategoryRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	c, err := h.svc.CreateCategory(r.Context(), actor(r), req.ParentID, req.Name, req.SortOrder)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toDTO(c))
}

func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateCategoryRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	c, err := h.svc.UpdateCategory(r.Context(), actor(r), id, req.Name, req.SortOrder)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toDTO(c))
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), actor(r), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pathID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	switch {
	case errors.As(err, &ve):
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Message}
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Data tidak valid", fields...)
	case errors.Is(err, httpx.ErrBadBody):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "Body permintaan tidak valid")
	case errors.Is(err, application.ErrNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Data tidak ditemukan")
	case errors.Is(err, application.ErrCategoryNameTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "CATEGORY_NAME_TAKEN", "Nama kategori sudah dipakai")
	case errors.Is(err, application.ErrCategoryInUse):
		httpx.WriteProblem(w, r, http.StatusConflict, "CATEGORY_IN_USE", "Kategori masih memiliki sub-kategori")
	case errors.Is(err, application.ErrInvalidParent):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "INVALID_PARENT", "Induk kategori tidak valid (maksimal 2 level)")
	default:
		h.log.Error("catalog: error tak terduga", "err", err, "path", r.URL.Path)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Terjadi kesalahan pada server")
	}
}

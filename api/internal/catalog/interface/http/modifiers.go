package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

type modifierRequest struct {
	ID         *uuid.UUID `json:"id"` // opsi yang sudah ada (PATCH); kosong = opsi baru
	Name       string     `json:"name"`
	PriceDelta int64      `json:"price_delta"`
	IsDefault  bool       `json:"is_default"`
	IsActive   *bool      `json:"is_active"` // default true
}

type modifierGroupRequest struct {
	Name      string            `json:"name"`
	MinSelect int               `json:"min_select"`
	MaxSelect int               `json:"max_select"`
	Modifiers []modifierRequest `json:"modifiers"`
}

func (g modifierGroupRequest) toInput() domain.ModifierGroupInput {
	in := domain.ModifierGroupInput{Name: g.Name, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect}
	for _, m := range g.Modifiers {
		active := true
		if m.IsActive != nil {
			active = *m.IsActive
		}
		in.Modifiers = append(in.Modifiers, domain.ModifierInput{
			ID: m.ID, Name: m.Name, PriceDelta: m.PriceDelta, IsDefault: m.IsDefault, IsActive: active,
		})
	}
	return in
}

type modifierDTO struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	PriceDelta int64     `json:"price_delta"`
	IsDefault  bool      `json:"is_default"`
	IsActive   bool      `json:"is_active"`
}

type modifierGroupDTO struct {
	ID         uuid.UUID     `json:"id"`
	Name       string        `json:"name"`
	MinSelect  int           `json:"min_select"`
	MaxSelect  int           `json:"max_select"`
	IsRequired bool          `json:"is_required"`
	Version    int           `json:"version"`
	Modifiers  []modifierDTO `json:"modifiers"`
}

func toGroupDTO(g domain.ModifierGroup) modifierGroupDTO {
	mods := make([]modifierDTO, len(g.Modifiers))
	for i, m := range g.Modifiers {
		mods[i] = modifierDTO{ID: m.ID, Name: m.Name, PriceDelta: m.PriceDelta, IsDefault: m.IsDefault, IsActive: m.IsActive}
	}
	return modifierGroupDTO{
		ID: g.ID, Name: g.Name, MinSelect: g.MinSelect, MaxSelect: g.MaxSelect,
		IsRequired: g.IsRequired(), Version: g.Version, Modifiers: mods,
	}
}

func writeGroup(w http.ResponseWriter, status int, g domain.ModifierGroup) {
	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, g.Version))
	httpx.WriteJSON(w, status, toGroupDTO(g))
}

func (h *Handler) listModifierGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.modifiers.List(r.Context(), actor(r))
	if err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	items := make([]modifierGroupDTO, len(groups))
	for i, g := range groups {
		items[i] = toGroupDTO(g)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) getModifierGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	g, err := h.modifiers.Get(r.Context(), actor(r), id)
	if err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	writeGroup(w, http.StatusOK, g)
}

func (h *Handler) createModifierGroup(w http.ResponseWriter, r *http.Request) {
	var req modifierGroupRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	g, err := h.modifiers.Create(r.Context(), actor(r), req.toInput())
	if err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	writeGroup(w, http.StatusCreated, g)
}

func (h *Handler) updateModifierGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	version, ok := parseIfMatch(r.Header.Get("If-Match"))
	if !ok {
		httpx.WriteProblem(w, r, http.StatusPreconditionRequired, "PRECONDITION_REQUIRED", "Header If-Match dengan versi grup wajib diisi")
		return
	}
	var req modifierGroupRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	g, err := h.modifiers.Update(r.Context(), actor(r), id, version, req.toInput())
	if err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	writeGroup(w, http.StatusOK, g)
}

func (h *Handler) deleteModifierGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.modifiers.Delete(r.Context(), actor(r), id); err != nil {
		h.writeModifierError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeModifierError(w http.ResponseWriter, r *http.Request, err error) {
	var im *application.InvalidModifierError
	switch {
	case errors.As(err, &im):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "INVALID_MODIFIER", "Opsi tidak valid",
			httpx.FieldError{Field: fmt.Sprintf("modifiers[%d].id", im.Index), Message: "Opsi tidak ditemukan pada grup ini"})
	case errors.Is(err, application.ErrModifierGroupNameTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "MODIFIER_GROUP_NAME_TAKEN", "Nama grup modifier sudah dipakai")
	case errors.Is(err, application.ErrVersionConflict):
		httpx.WriteProblem(w, r, http.StatusPreconditionFailed, "VERSION_CONFLICT", "Grup sudah diubah pihak lain, muat ulang data")
	default:
		h.writeError(w, r, err)
	}
}

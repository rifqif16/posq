package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/authn"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

const permUserManage = "user:manage"

type Guard func(permission string) func(http.Handler) http.Handler

type StaffHandler struct {
	svc *application.StaffService
	log *slog.Logger
}

func NewStaffHandler(svc *application.StaffService, log *slog.Logger) *StaffHandler {
	return &StaffHandler{svc: svc, log: log}
}

func (h *StaffHandler) Mount(r chi.Router, require Guard) {
	r.With(require(permUserManage)).Get("/stores", h.stores)
	r.With(require(permUserManage)).Get("/staff", h.list)
	r.With(require(permUserManage)).Post("/staff", h.create)
	r.With(require(permUserManage)).Patch("/staff/{id}", h.update)
	r.With(require(permUserManage)).Post("/staff/{id}/password", h.resetPassword)
	r.With(require(permUserManage)).Put("/staff/{id}/pin", h.setPIN)
	r.With(require(permUserManage)).Delete("/staff/{id}/pin", h.clearPIN)
}

func staffActor(r *http.Request) application.StaffActor {
	p, _ := authn.From(r.Context())
	return application.StaffActor{TenantID: p.TenantID, UserID: p.UserID, Role: domain.Role(p.Role)}
}

type createStaffRequest struct {
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	Password string      `json:"password"`
	Role     string      `json:"role"`
	StoreIDs []uuid.UUID `json:"store_ids"`
	PIN      string      `json:"pin"`
}

type updateStaffRequest struct {
	Name     string      `json:"name"`
	Role     string      `json:"role"`
	StoreIDs []uuid.UUID `json:"store_ids"`
	Status   string      `json:"status"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

type pinRequest struct {
	PIN string `json:"pin"`
}

type staffDTO struct {
	ID         uuid.UUID   `json:"id"`
	Name       string      `json:"name"`
	Email      string      `json:"email"`
	Role       string      `json:"role"`
	Status     string      `json:"status"`
	StoreIDs   []uuid.UUID `json:"store_ids"`
	HasPIN     bool        `json:"has_pin"`
	Manageable bool        `json:"manageable"`
	CreatedAt  time.Time   `json:"created_at"`
}

type storeDTO struct {
	ID   uuid.UUID `json:"id"`
	Code string    `json:"code"`
	Name string    `json:"name"`
}

func toStaffDTO(m application.StaffMember, a application.StaffActor) staffDTO {
	return staffDTO{
		ID: m.ID, Name: m.Name, Email: m.Email, Role: string(m.Role), Status: string(m.Status), StoreIDs: m.StoreIDs,
		HasPIN: m.HasPIN, Manageable: m.ID != a.UserID && domain.CanManage(a.Role, m.Role), CreatedAt: m.CreatedAt,
	}
}

func (h *StaffHandler) stores(w http.ResponseWriter, r *http.Request) {
	stores, err := h.svc.Stores(r.Context(), staffActor(r))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]storeDTO, len(stores))
	for i, s := range stores {
		items[i] = storeDTO{ID: s.ID, Code: s.Code, Name: s.Name}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *StaffHandler) list(w http.ResponseWriter, r *http.Request) {
	a := staffActor(r)
	members, err := h.svc.List(r.Context(), a)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items := make([]staffDTO, len(members))
	for i, m := range members {
		items[i] = toStaffDTO(m, a)
	}
	assignable := domain.AssignableRoles(a.Role)
	roles := make([]string, len(assignable))
	for i, role := range assignable {
		roles[i] = string(role)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "assignable_roles": roles})
}

func (h *StaffHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createStaffRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	a := staffActor(r)
	m, err := h.svc.Create(r.Context(), a, domain.StaffInput{
		Name: req.Name, Email: req.Email, Password: req.Password, Role: domain.Role(req.Role), StoreIDs: req.StoreIDs, PIN: req.PIN,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toStaffDTO(m, a))
}

func (h *StaffHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := staffID(w, r)
	if !ok {
		return
	}
	var req updateStaffRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	a := staffActor(r)
	m, err := h.svc.Update(r.Context(), a, id, domain.StaffChange{
		Name: req.Name, Role: domain.Role(req.Role), StoreIDs: req.StoreIDs, Status: domain.UserStatus(req.Status),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toStaffDTO(m, a))
}

func (h *StaffHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := staffID(w, r)
	if !ok {
		return
	}
	var req passwordRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.ResetPassword(r.Context(), staffActor(r), id, req.Password); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *StaffHandler) setPIN(w http.ResponseWriter, r *http.Request) {
	id, ok := staffID(w, r)
	if !ok {
		return
	}
	var req pinRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.SetPIN(r.Context(), staffActor(r), id, req.PIN); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *StaffHandler) clearPIN(w http.ResponseWriter, r *http.Request) {
	id, ok := staffID(w, r)
	if !ok {
		return
	}
	if err := h.svc.ClearPIN(r.Context(), staffActor(r), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func staffID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Pengguna tidak ditemukan")
		return uuid.Nil, false
	}
	return id, true
}

func (h *StaffHandler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	var se *application.StoreInvalidError
	switch {
	case errors.As(err, &ve):
		fields := make([]httpx.FieldError, len(ve.Issues))
		for i, is := range ve.Issues {
			fields[i] = httpx.FieldError{Field: is.Field, Message: is.Message}
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "VALIDATION_FAILED", "Data tidak valid", fields...)
	case errors.As(err, &se):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "INVALID_STORE", "Outlet tidak valid",
			httpx.FieldError{Field: fmt.Sprintf("store_ids[%d]", se.Index), Message: "Outlet tidak ditemukan atau bukan milik Anda"})
	case errors.Is(err, application.ErrForbiddenTarget):
		httpx.WriteProblem(w, r, http.StatusForbidden, "FORBIDDEN", "Anda tidak boleh mengelola pengguna ini")
	case errors.Is(err, application.ErrRoleNotAllowed):
		httpx.WriteProblem(w, r, http.StatusForbidden, "ROLE_NOT_ALLOWED", "Anda tidak boleh memberikan role ini")
	case errors.Is(err, application.ErrEmailTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "EMAIL_TAKEN", "Email sudah terdaftar",
			httpx.FieldError{Field: "email", Message: "Email sudah terdaftar"})
	case errors.Is(err, application.ErrPlanLimit):
		httpx.WriteProblem(w, r, http.StatusConflict, "PLAN_LIMIT_REACHED", "Batas jumlah pengguna aktif pada paket Anda sudah tercapai")
	case errors.Is(err, application.ErrUserNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Pengguna tidak ditemukan")
	case errors.Is(err, httpx.ErrBadBody):
		httpx.WriteProblem(w, r, http.StatusBadRequest, "BAD_REQUEST", "Permintaan tidak valid")
	default:
		h.log.Error("staff: error tak terduga", "err", err, "path", r.URL.Path)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Terjadi kesalahan pada server")
	}
}

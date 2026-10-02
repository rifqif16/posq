package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

const permSettingsWrite = "settings:write"

type DeviceHandler struct {
	*Handler
	devices *application.DeviceService
	pin     *application.PinLoginService
}

func NewDeviceHandler(base *Handler, devices *application.DeviceService, pin *application.PinLoginService) *DeviceHandler {
	return &DeviceHandler{Handler: base, devices: devices, pin: pin}
}

func (h *DeviceHandler) Mount(r chi.Router, require Guard) {
	limit := h.limiter.Middleware(
		func(r *http.Request) string { return httpx.ClientIP(r, h.trustedIP) },
		func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak percobaan, coba lagi nanti")
		})
	r.With(require(permSettingsWrite)).Get("/devices", h.list)
	r.With(require(permSettingsWrite)).Post("/devices", h.create)
	r.With(require(permSettingsWrite)).Post("/devices/{id}/revoke", h.revoke)
	r.With(limit).Post("/auth/pin-users", h.pinUsers)
	r.With(limit).Post("/auth/pin-login", h.pinLogin)
}

type createDeviceRequest struct {
	StoreID uuid.UUID `json:"store_id"`
	Name    string    `json:"name"`
}

type deviceCredentials struct {
	DeviceID     uuid.UUID `json:"device_id"`
	DeviceSecret string    `json:"device_secret"`
}

type pinLoginRequest struct {
	deviceCredentials
	UserID uuid.UUID `json:"user_id"`
	PIN    string    `json:"pin"`
}

type deviceDTO struct {
	ID           uuid.UUID  `json:"id"`
	StoreID      uuid.UUID  `json:"store_id"`
	Code         string     `json:"code"`
	Name         string     `json:"name"`
	Status       string     `json:"status"`
	RegisteredAt time.Time  `json:"registered_at"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	RegisteredBy string     `json:"registered_by"`
}

type pinUserDTO struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Role string    `json:"role"`
}

func toDeviceDTO(d application.Device) deviceDTO {
	return deviceDTO{
		ID: d.ID, StoreID: d.StoreID, Code: d.Code, Name: d.Name, Status: d.Status,
		RegisteredAt: d.RegisteredAt, LastSeenAt: d.LastSeenAt, RegisteredBy: d.RegisteredBy,
	}
}

func (h *DeviceHandler) list(w http.ResponseWriter, r *http.Request) {
	devices, err := h.devices.List(r.Context(), staffActor(r))
	if err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	items := make([]deviceDTO, len(devices))
	for i, d := range devices {
		items[i] = toDeviceDTO(d)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *DeviceHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createDeviceRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	d, secret, err := h.devices.Create(r.Context(), staffActor(r), req.StoreID, req.Name)
	if err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, struct {
		deviceDTO
		Secret string `json:"secret"`
	}{toDeviceDTO(d), secret})
}

func (h *DeviceHandler) revoke(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Perangkat tidak ditemukan")
		return
	}
	if err := h.devices.Revoke(r.Context(), staffActor(r), id); err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *DeviceHandler) pinUsers(w http.ResponseWriter, r *http.Request) {
	var req deviceCredentials
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	res, err := h.pin.Users(r.Context(), req.DeviceID, req.DeviceSecret)
	if err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	items := make([]pinUserDTO, len(res.Users))
	for i, u := range res.Users {
		items[i] = pinUserDTO{ID: u.ID, Name: u.Name, Role: string(u.Role)}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"device_name": res.DeviceName, "store_id": res.StoreID, "items": items})
}

func (h *DeviceHandler) pinLogin(w http.ResponseWriter, r *http.Request) {
	var req pinLoginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	session, err := h.pin.Login(r.Context(), application.PinLoginInput{
		DeviceID: req.DeviceID, DeviceSecret: req.DeviceSecret, UserID: req.UserID, PIN: req.PIN,
	})
	if err != nil {
		h.writeDeviceError(w, r, err)
		return
	}
	h.writeSession(w, http.StatusOK, session)
}

func (h *DeviceHandler) writeDeviceError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *domain.ValidationError
	var se *application.StoreInvalidError
	switch {
	case errors.As(err, &ve):
		h.Handler.writeError(w, r, err)
	case errors.As(err, &se):
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "INVALID_STORE", "Outlet tidak valid",
			httpx.FieldError{Field: "store_id", Message: "Outlet tidak ditemukan atau bukan milik Anda"})
	case errors.Is(err, application.ErrDevicePlanLimit):
		httpx.WriteProblem(w, r, http.StatusConflict, "DEVICE_LIMIT_REACHED", "Batas jumlah perangkat aktif pada paket Anda sudah tercapai")
	case errors.Is(err, application.ErrDeviceNotFound):
		httpx.WriteProblem(w, r, http.StatusNotFound, "NOT_FOUND", "Perangkat tidak ditemukan")
	case errors.Is(err, application.ErrInvalidCredentials):
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "PIN salah atau perangkat tidak valid")
	case errors.Is(err, application.ErrPinLocked):
		w.Header().Set("Retry-After", strconv.Itoa(int(application.PinLockDuration.Seconds())))
		httpx.WriteProblem(w, r, http.StatusTooManyRequests, "PIN_LOCKED", "Terlalu banyak PIN salah; coba lagi dalam 15 menit")
	default:
		h.Handler.writeError(w, r, err)
	}
}

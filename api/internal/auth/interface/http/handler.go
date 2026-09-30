// Package httpapi (auth): entrypoint HTTP. Memetakan request/response dan error aplikasi.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
	"github.com/rifqif16/posq/api/internal/platform/ratelimit"
)

const refreshCookieName = "posq_rt"

type Handler struct {
	svc          *application.Service
	log          *slog.Logger
	cookieSecure bool
	trustedIP    string
	limiter      *ratelimit.Limiter
}

func NewHandler(svc *application.Service, log *slog.Logger, cookieSecure bool, trustedIPHeader string) *Handler {
	return &Handler{
		svc: svc, log: log, cookieSecure: cookieSecure, trustedIP: trustedIPHeader,
		limiter: ratelimit.New(10, time.Minute), // 10 percobaan/menit/IP untuk register+login
	}
}

// Mount memasang rute pada router /v1.
func (h *Handler) Mount(r chi.Router) {
	limit := h.limiter.Middleware(
		func(r *http.Request) string { return httpx.ClientIP(r, h.trustedIP) },
		func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteProblem(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak percobaan, coba lagi nanti")
		})
	r.With(limit).Post("/auth/register", h.register)
	r.With(limit).Post("/auth/login", h.login)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)
	r.With(h.RequireAuth).Get("/me", h.me)
}

type registerRequest struct {
	BusinessName string `json:"business_name"`
	OwnerName    string `json:"owner_name"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	sess, err := h.svc.Register(r.Context(), domain.Registration(req))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeSession(w, http.StatusCreated, sess)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	sess, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeSession(w, http.StatusOK, sess)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		h.writeError(w, r, application.ErrInvalidRefreshToken)
		return
	}
	sess, err := h.svc.Refresh(r.Context(), cookie.Value)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.writeSession(w, http.StatusOK, sess)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		if err := h.svc.Logout(r.Context(), cookie.Value); err != nil {
			h.writeError(w, r, err)
			return
		}
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := ClaimsFrom(r.Context())
	p, err := h.svc.Me(r.Context(), claims)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toProfile(p))
}

type ctxKey struct{}

// RequireAuth memvalidasi header "Authorization: Bearer <access token>".
func (h *Handler) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			h.writeError(w, r, application.ErrUnauthorized)
			return
		}
		claims, err := h.svc.Authenticate(token)
		if err != nil {
			h.writeError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
	})
}

func ClaimsFrom(ctx context.Context) (application.AccessClaims, bool) {
	c, ok := ctx.Value(ctxKey{}).(application.AccessClaims)
	return c, ok
}

func (h *Handler) writeSession(w http.ResponseWriter, status int, s application.Session) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: s.RefreshToken, Path: "/", Expires: s.RefreshExpiresAt,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	})
	httpx.WriteJSON(w, status, sessionResponse{
		AccessToken: s.AccessToken, TokenType: "Bearer", ExpiresIn: int(s.AccessExpiresIn.Seconds()),
		Profile: toProfile(s.Principal),
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
	})
}

// writeError memetakan error aplikasi ke problem+json. Error tak dikenal dicatat, tidak dibocorkan.
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
	case errors.Is(err, application.ErrEmailTaken):
		httpx.WriteProblem(w, r, http.StatusConflict, "EMAIL_TAKEN", "Email sudah terdaftar")
	case errors.Is(err, application.ErrInvalidCredentials):
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email atau password salah")
	case errors.Is(err, application.ErrAccountDisabled):
		httpx.WriteProblem(w, r, http.StatusForbidden, "ACCOUNT_DISABLED", "Akun dinonaktifkan")
	case errors.Is(err, application.ErrInvalidRefreshToken), errors.Is(err, application.ErrRefreshTokenReuse):
		h.clearRefreshCookie(w)
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Sesi berakhir, silakan login kembali")
	case errors.Is(err, application.ErrUnauthorized):
		httpx.WriteProblem(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "Token tidak valid atau kedaluwarsa")
	default:
		h.log.Error("auth: error tak terduga", "err", err, "path", r.URL.Path)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "INTERNAL", "Terjadi kesalahan pada server")
	}
}

type sessionResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Profile
}

type Profile struct {
	User     userDTO     `json:"user"`
	Tenant   tenantDTO   `json:"tenant"`
	StoreIDs []uuid.UUID `json:"store_ids"`
}

type userDTO struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Role  string    `json:"role"`
}

type tenantDTO struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	TrialEndsAt *time.Time `json:"trial_ends_at"`
}

func toProfile(p application.Principal) Profile {
	stores := p.StoreIDs
	if stores == nil {
		stores = []uuid.UUID{}
	}
	return Profile{
		User:     userDTO{ID: p.UserID, Name: p.Name, Email: p.Email, Role: string(p.Role)},
		Tenant:   tenantDTO{ID: p.TenantID, Name: p.TenantName, Status: p.TenantStatus, TrialEndsAt: p.TrialEndsAt},
		StoreIDs: stores,
	}
}

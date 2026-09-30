package httpapi

import (
	"net/http"

	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/authn"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

func (h *Handler) Require(permission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return h.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, _ := ClaimsFrom(r.Context())
			if !domain.Can(c.Role, permission) {
				httpx.WriteProblem(w, r, http.StatusForbidden, "FORBIDDEN", "Anda tidak memiliki izin untuk aksi ini")
				return
			}
			p := authn.Principal{UserID: c.UserID, TenantID: c.TenantID, Role: string(c.Role)}
			next.ServeHTTP(w, r.WithContext(authn.With(r.Context(), p)))
		}))
	}
}

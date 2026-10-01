package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/auth"
	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/catalog"
	"github.com/rifqif16/posq/api/internal/platform/config"
	"github.com/rifqif16/posq/api/internal/platform/httpx"
)

type Deps struct {
	Pool   *pgxpool.Pool
	Config config.Config
	Logger *slog.Logger
	Hasher application.PasswordHasher
}

func NewRouter(d Deps) (http.Handler, error) {
	authHandler, err := auth.New(auth.Deps{Pool: d.Pool, Config: d.Config, Logger: d.Logger, Hasher: d.Hasher})
	if err != nil {
		return nil, err
	}
	catalogHandlers := catalog.New(catalog.Deps{Pool: d.Pool, Logger: d.Logger})

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, httpx.RequestLogger(d.Logger), httpx.SecurityHeaders)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", healthz(d.Pool))
	r.Route("/v1", func(r chi.Router) {
		authHandler.Mount(r)
		catalogHandlers.Catalog.Mount(r, authHandler.Require)
		catalogHandlers.PriceHistory.Mount(r, authHandler.Require)
		catalogHandlers.ProductCSV.Mount(r, authHandler.Require)
	})
	return r, nil
}

func healthz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "Database tidak tersedia")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

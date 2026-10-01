package catalog

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	catalogpg "github.com/rifqif16/posq/api/internal/catalog/infrastructure/pg"
	httpapi "github.com/rifqif16/posq/api/internal/catalog/interface/http"
)

type Deps struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

func New(d Deps) *httpapi.Handler {
	repo := catalogpg.New(d.Pool)
	return httpapi.NewHandler(
		application.NewService(repo), application.NewProductService(repo), application.NewModifierService(repo), d.Logger)
}

package inventory

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/inventory/application"
	inventorypg "github.com/rifqif16/posq/api/internal/inventory/infrastructure/pg"
	httpapi "github.com/rifqif16/posq/api/internal/inventory/interface/http"
)

type Deps struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

func New(d Deps) *httpapi.Handler {
	return httpapi.NewHandler(application.NewService(inventorypg.New(d.Pool)), d.Logger)
}

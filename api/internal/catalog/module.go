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

type Handlers struct {
	Catalog      *httpapi.Handler
	PriceHistory *httpapi.PriceHistoryHandler
	ProductCSV   *httpapi.ProductCSVHandler
}

func New(d Deps) Handlers {
	repo := catalogpg.New(d.Pool)
	return Handlers{
		Catalog: httpapi.NewHandler(
			application.NewService(repo), application.NewProductService(repo), application.NewModifierService(repo), d.Logger),
		PriceHistory: httpapi.NewPriceHistoryHandler(application.NewPriceHistoryService(repo), d.Logger),
		ProductCSV:   httpapi.NewProductCSVHandler(application.NewProductCSVService(repo), d.Logger),
	}
}

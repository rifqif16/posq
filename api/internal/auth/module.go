// Package auth adalah composition root modul auth: merangkai domain, application,
// infrastructure, dan entrypoint HTTP.
package auth

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/infrastructure"
	authpg "github.com/rifqif16/posq/api/internal/auth/infrastructure/pg"
	httpapi "github.com/rifqif16/posq/api/internal/auth/interface/http"
	"github.com/rifqif16/posq/api/internal/platform/config"
)

type Deps struct {
	Pool   *pgxpool.Pool
	Config config.Config
	Logger *slog.Logger
	// Hasher opsional; default argon2id parameter produksi. Test menyuntik versi cepat.
	Hasher application.PasswordHasher
}

func New(d Deps) (*httpapi.Handler, error) {
	hasher := d.Hasher
	if hasher == nil {
		hasher = infrastructure.NewArgon2Hasher(infrastructure.DefaultArgon2Params)
	}
	svc, err := application.NewService(
		authpg.New(d.Pool), hasher, infrastructure.NewJWTIssuer(d.Config.JWTPrivateKey),
		application.Options{AccessTTL: d.Config.AccessTTL, RefreshTTL: d.Config.RefreshTTL, TrialDays: d.Config.TrialDays},
	)
	if err != nil {
		return nil, err
	}
	return httpapi.NewHandler(svc, d.Logger, d.Config.CookieSecure, d.Config.TrustedIPHeader), nil
}

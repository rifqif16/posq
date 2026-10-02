package auth

import (
	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/infrastructure"
	authpg "github.com/rifqif16/posq/api/internal/auth/infrastructure/pg"
	httpapi "github.com/rifqif16/posq/api/internal/auth/interface/http"
)

func NewDevices(d Deps) (*httpapi.DeviceHandler, error) {
	hasher := d.Hasher
	if hasher == nil {
		hasher = infrastructure.NewArgon2Hasher(infrastructure.DefaultArgon2Params)
	}
	repo := authpg.New(d.Pool)
	svc, err := application.NewService(
		repo, hasher, infrastructure.NewJWTIssuer(d.Config.JWTPrivateKey),
		application.Options{AccessTTL: d.Config.AccessTTL, RefreshTTL: d.Config.RefreshTTL, TrialDays: d.Config.TrialDays},
	)
	if err != nil {
		return nil, err
	}
	base := httpapi.NewHandler(svc, d.Logger, d.Config.CookieSecure, d.Config.TrustedIPHeader)
	return httpapi.NewDeviceHandler(base, application.NewDeviceService(repo), application.NewPinLoginService(svc, repo)), nil
}

package auth

import (
	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/infrastructure"
	authpg "github.com/rifqif16/posq/api/internal/auth/infrastructure/pg"
	httpapi "github.com/rifqif16/posq/api/internal/auth/interface/http"
)

func NewStaff(d Deps) *httpapi.StaffHandler {
	hasher := d.Hasher
	if hasher == nil {
		hasher = infrastructure.NewArgon2Hasher(infrastructure.DefaultArgon2Params)
	}
	return httpapi.NewStaffHandler(application.NewStaffService(authpg.New(d.Pool), hasher), d.Logger)
}

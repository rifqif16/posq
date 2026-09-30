// Package application (auth): use case dan port. Tidak mengenal HTTP atau SQL.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/rifqif16/posq/api/internal/auth/domain"
)

var (
	ErrEmailTaken          = errors.New("email sudah terdaftar")
	ErrInvalidCredentials  = errors.New("email atau password salah")
	ErrAccountDisabled     = errors.New("akun dinonaktifkan")
	ErrInvalidRefreshToken = errors.New("refresh token tidak valid")
	ErrRefreshTokenReuse   = errors.New("refresh token dipakai ulang")
	ErrUnauthorized        = errors.New("tidak terautentikasi")
	ErrUserNotFound        = errors.New("user tidak ditemukan") // dikembalikan repository
)

type NewAccount struct {
	TenantID, StoreID, UserID uuid.UUID
	BusinessName              string
	OwnerName, Email          string
	PasswordHash              string
	TrialEndsAt               time.Time
}

type LoginUser struct {
	ID, TenantID uuid.UUID
	PasswordHash string
	Status       domain.UserStatus
}

type Principal struct {
	UserID, TenantID uuid.UUID
	Name, Email      string
	Role             domain.Role
	StoreIDs         []uuid.UUID
	TenantName       string
	TenantStatus     string
	TrialEndsAt      *time.Time
}

type NewRefreshToken struct {
	ID, TenantID, UserID, FamilyID uuid.UUID
	Hash                           []byte
	ExpiresAt                      time.Time
}

type RotateOutcome int

const (
	RotateOK RotateOutcome = iota
	RotateInvalid
	RotateReused
	RotateUserDisabled
)

type RotateInput struct {
	TenantID  uuid.UUID
	OldHash   []byte
	NewID     uuid.UUID
	NewHash   []byte
	NewExpiry time.Time
	Now       time.Time
}

type RotateResult struct {
	Outcome RotateOutcome
	UserID  uuid.UUID
}

// Repository adalah port persistensi. Implementasi menjamin isolasi tenant (RLS).
type Repository interface {
	CreateAccount(ctx context.Context, a NewAccount) error // ErrEmailTaken bila email dipakai
	FindLoginUser(ctx context.Context, email string) (LoginUser, error)
	GetPrincipal(ctx context.Context, tenantID, userID uuid.UUID) (Principal, error)
	SaveRefreshToken(ctx context.Context, t NewRefreshToken) error
	// RotateRefreshToken atomik: cabut token lama dan buat baru; bila token lama sudah
	// dicabut (reuse) seluruh keluarga dicabut dan hasilnya RotateReused.
	RotateRefreshToken(ctx context.Context, in RotateInput) (RotateResult, error)
	RevokeRefreshFamily(ctx context.Context, tenantID uuid.UUID, tokenHash []byte, now time.Time) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}

type AccessClaims struct {
	UserID, TenantID uuid.UUID
	Role             domain.Role
	StoreIDs         []uuid.UUID
}

type TokenIssuer interface {
	Issue(c AccessClaims, now time.Time, ttl time.Duration) (string, error)
	Parse(token string, now time.Time) (AccessClaims, error)
}

type Session struct {
	AccessToken      string
	AccessExpiresIn  time.Duration
	RefreshToken     string
	RefreshExpiresAt time.Time
	Principal        Principal
}

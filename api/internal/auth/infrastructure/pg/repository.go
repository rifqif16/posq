// Package pg (auth): implementasi application.Repository di PostgreSQL.
// Semua query tenant berjalan lewat database.WithTenantTx sehingga RLS berlaku.
package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
	tenantpg "github.com/rifqif16/posq/api/internal/tenant/infrastructure/pg"
)

const uniqueViolation = "23505"

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ application.Repository = (*Repository)(nil)

func (r *Repository) CreateAccount(ctx context.Context, a application.NewAccount) error {
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		err := tenantpg.InsertWithFirstStore(ctx, tx, tenantpg.Seed{
			TenantID: a.TenantID, StoreID: a.StoreID, BusinessName: a.BusinessName, TrialEndsAt: a.TrialEndsAt,
		})
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, tenant_id, email, password_hash, name) VALUES ($1,$2,$3,$4,$5)`,
			a.UserID, a.TenantID, a.Email, a.PasswordHash, a.OwnerName); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_store_roles (user_id, store_id, tenant_id, role) VALUES ($1,$2,$3,$4)`,
			a.UserID, a.StoreID, a.TenantID, string(domain.RoleOwner)); err != nil {
			return fmt.Errorf("insert role: %w", err)
		}
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == "users_email_key" {
		return application.ErrEmailTaken
	}
	return err
}

// FindLoginUser satu-satunya query lintas-tenant; lewat fungsi SECURITY DEFINER (lihat migrasi).
func (r *Repository) FindLoginUser(ctx context.Context, email string) (application.LoginUser, error) {
	var u application.LoginUser
	var status string
	var name, mail string // kolom fungsi; tidak dipakai di sini
	err := r.pool.QueryRow(ctx, `SELECT id, tenant_id, name, email, password_hash, status FROM auth_find_user_by_email($1)`, email).
		Scan(&u.ID, &u.TenantID, &name, &mail, &u.PasswordHash, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.LoginUser{}, application.ErrUserNotFound
	}
	if err != nil {
		return application.LoginUser{}, fmt.Errorf("cari user: %w", err)
	}
	u.Status = domain.UserStatus(status)
	return u, nil
}

func (r *Repository) GetPrincipal(ctx context.Context, tenantID, userID uuid.UUID) (application.Principal, error) {
	p := application.Principal{UserID: userID, TenantID: tenantID}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT u.name, u.email, t.name, t.status, t.trial_ends_at
			FROM users u JOIN tenants t ON t.id = u.tenant_id
			WHERE u.id = $1`, userID).Scan(&p.Name, &p.Email, &p.TenantName, &p.TenantStatus, &p.TrialEndsAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("baca user: %w", err)
		}
		return loadRoles(ctx, tx, userID, &p)
	})
	return p, err
}

// loadRoles mengambil role tertinggi (owner > admin > cashier > kitchen) dan daftar toko.
func loadRoles(ctx context.Context, tx pgx.Tx, userID uuid.UUID, p *application.Principal) error {
	rows, err := tx.Query(ctx, `
		SELECT store_id, role FROM user_store_roles WHERE user_id = $1
		ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'cashier' THEN 2 ELSE 3 END, store_id`, userID)
	if err != nil {
		return fmt.Errorf("baca role: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var store uuid.UUID
		var role string
		if err := rows.Scan(&store, &role); err != nil {
			return fmt.Errorf("scan role: %w", err)
		}
		if p.Role == "" {
			p.Role = domain.Role(role)
		}
		p.StoreIDs = append(p.StoreIDs, store)
	}
	return rows.Err()
}

func (r *Repository) SaveRefreshToken(ctx context.Context, t application.NewRefreshToken) error {
	return database.WithTenantTx(ctx, r.pool, t.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO refresh_tokens (id, tenant_id, user_id, family_id, token_hash, expires_at)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			t.ID, t.TenantID, t.UserID, t.FamilyID, t.Hash, t.ExpiresAt)
		if err != nil {
			return fmt.Errorf("simpan refresh token: %w", err)
		}
		return nil
	})
}

func (r *Repository) RotateRefreshToken(ctx context.Context, in application.RotateInput) (application.RotateResult, error) {
	res := application.RotateResult{Outcome: application.RotateInvalid}
	// Hasil dikembalikan lewat res (bukan error) agar pencabutan keluarga tetap di-commit.
	err := database.WithTenantTx(ctx, r.pool, in.TenantID, func(tx pgx.Tx) error {
		var id, userID, familyID uuid.UUID
		var expiresAt time.Time
		var revokedAt *time.Time
		var userStatus string
		err := tx.QueryRow(ctx, `
			SELECT rt.id, rt.user_id, rt.family_id, rt.expires_at, rt.revoked_at, u.status
			FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id
			WHERE rt.token_hash = $1 FOR UPDATE OF rt`, in.OldHash).
			Scan(&id, &userID, &familyID, &expiresAt, &revokedAt, &userStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("baca refresh token: %w", err)
		}
		switch {
		case revokedAt != nil:
			res.Outcome = application.RotateReused
			return revokeFamily(ctx, tx, familyID, in.Now)
		case userStatus != string(domain.UserActive):
			res.Outcome = application.RotateUserDisabled
			return revokeFamily(ctx, tx, familyID, in.Now)
		case !in.Now.Before(expiresAt):
			return nil
		}
		if err := insertNext(ctx, tx, in, id, userID, familyID); err != nil {
			return err
		}
		res = application.RotateResult{Outcome: application.RotateOK, UserID: userID}
		return nil
	})
	return res, err
}

func insertNext(ctx context.Context, tx pgx.Tx, in application.RotateInput, oldID, userID, familyID uuid.UUID) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, tenant_id, user_id, family_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		in.NewID, in.TenantID, userID, familyID, in.NewHash, in.NewExpiry); err != nil {
		return fmt.Errorf("buat refresh token baru: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2, replaced_by = $3 WHERE id = $1`,
		oldID, in.Now, in.NewID); err != nil {
		return fmt.Errorf("cabut refresh token lama: %w", err)
	}
	return nil
}

func revokeFamily(ctx context.Context, tx pgx.Tx, familyID uuid.UUID, now time.Time) error {
	_, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2 WHERE family_id = $1 AND revoked_at IS NULL`, familyID, now)
	if err != nil {
		return fmt.Errorf("cabut keluarga token: %w", err)
	}
	return nil
}

func (r *Repository) RevokeRefreshFamily(ctx context.Context, tenantID uuid.UUID, tokenHash []byte, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var familyID uuid.UUID
		err := tx.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE token_hash = $1`, tokenHash).Scan(&familyID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cari keluarga token: %w", err)
		}
		return revokeFamily(ctx, tx, familyID, now)
	})
}

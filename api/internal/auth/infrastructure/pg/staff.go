package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

var _ application.StaffRepository = (*Repository)(nil)

const usersEmailKey = "users_email_key"

const staffSelect = `
	SELECT u.id, u.name, u.email, u.status, u.pin_hash IS NOT NULL, u.created_at,
	       COALESCE((SELECT r.role FROM user_store_roles r WHERE r.user_id = u.id
	                 ORDER BY CASE r.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'cashier' THEN 2 ELSE 3 END LIMIT 1), ''),
	       ARRAY(SELECT r.store_id::text FROM user_store_roles r WHERE r.user_id = u.id ORDER BY r.store_id)
	FROM users u`

func scanStaff(row pgx.Row) (application.StaffMember, error) {
	var m application.StaffMember
	var status, role string
	var stores []string
	if err := row.Scan(&m.ID, &m.Name, &m.Email, &status, &m.HasPIN, &m.CreatedAt, &role, &stores); err != nil {
		return m, err
	}
	m.Status, m.Role = domain.UserStatus(status), domain.Role(role)
	m.StoreIDs = make([]uuid.UUID, len(stores))
	for i, s := range stores {
		m.StoreIDs[i] = uuid.MustParse(s)
	}
	return m, nil
}

func getStaff(ctx context.Context, tx pgx.Tx, id uuid.UUID) (application.StaffMember, error) {
	m, err := scanStaff(tx.QueryRow(ctx, staffSelect+` WHERE u.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return m, application.ErrUserNotFound
	}
	if err != nil {
		return m, fmt.Errorf("baca staf: %w", err)
	}
	return m, nil
}

func (r *Repository) ListStaff(ctx context.Context, tenantID uuid.UUID) ([]application.StaffMember, error) {
	out := []application.StaffMember{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, staffSelect+`
			ORDER BY (SELECT min(CASE r.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'cashier' THEN 2 ELSE 3 END)
			          FROM user_store_roles r WHERE r.user_id = u.id), lower(u.name), u.id`)
		if err != nil {
			return fmt.Errorf("list staf: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			m, err := scanStaff(rows)
			if err != nil {
				return fmt.Errorf("scan staf: %w", err)
			}
			out = append(out, m)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) ListStores(ctx context.Context, tenantID uuid.UUID) ([]application.StoreInfo, error) {
	out := []application.StoreInfo{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, code, name FROM stores ORDER BY code`)
		if err != nil {
			return fmt.Errorf("list outlet: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var s application.StoreInfo
			if err := rows.Scan(&s.ID, &s.Code, &s.Name); err != nil {
				return fmt.Errorf("scan outlet: %w", err)
			}
			out = append(out, s)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) GetStaff(ctx context.Context, tenantID, id uuid.UUID) (application.StaffMember, error) {
	var m application.StaffMember
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var err error
		m, err = getStaff(ctx, tx, id)
		return err
	})
	return m, err
}

func (r *Repository) UserStoreIDs(ctx context.Context, tenantID, userID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT store_id FROM user_store_roles WHERE user_id = $1`, userID)
		if err != nil {
			return fmt.Errorf("baca outlet pengguna: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var id uuid.UUID
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func ensureUserCapacity(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	var limit *int
	err := tx.QueryRow(ctx, `SELECT (p.limits->>'users')::int FROM tenants t JOIN plans p ON p.id = t.plan_id
		WHERE t.id = $1 FOR UPDATE OF t`, tenantID).Scan(&limit)
	if err != nil {
		return fmt.Errorf("baca batas paket: %w", err)
	}
	if limit == nil {
		return nil
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users WHERE status = 'active'`).Scan(&active); err != nil {
		return fmt.Errorf("hitung pengguna: %w", err)
	}
	if active >= *limit {
		return application.ErrPlanLimit
	}
	return nil
}

func syncStores(ctx context.Context, tx pgx.Tx, tenantID, userID uuid.UUID, role domain.Role, storeIDs []uuid.UUID) error {
	keep := make([]string, len(storeIDs))
	for i, id := range storeIDs {
		var found uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM stores WHERE id = $1`, id).Scan(&found)
		if errors.Is(err, pgx.ErrNoRows) {
			return &application.StoreInvalidError{Index: i}
		}
		if err != nil {
			return fmt.Errorf("cek outlet: %w", err)
		}
		keep[i] = id.String()
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_store_roles WHERE user_id = $1 AND NOT (store_id = ANY($2::uuid[]))`, userID, keep); err != nil {
		return fmt.Errorf("lepas outlet: %w", err)
	}
	for _, id := range storeIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_store_roles (user_id, store_id, tenant_id, role) VALUES ($1,$2,$3,$4)
			ON CONFLICT (user_id, store_id) DO UPDATE SET role = EXCLUDED.role`, userID, id, tenantID, string(role)); err != nil {
			return fmt.Errorf("tetapkan outlet: %w", err)
		}
	}
	return nil
}

func revokeUserTokens(ctx context.Context, tx pgx.Tx, userID uuid.UUID, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return fmt.Errorf("cabut sesi: %w", err)
	}
	return nil
}

func (r *Repository) CreateStaff(ctx context.Context, a application.StaffActor, n application.NewStaff, now time.Time) error {
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		if err := ensureUserCapacity(ctx, tx, a.TenantID); err != nil {
			return err
		}
		in := n.Input
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, tenant_id, email, password_hash, pin_hash, name, status, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,'active',$7,$7)`,
			n.ID, a.TenantID, in.Email, n.PasswordHash, n.PINHash, in.Name, now); err != nil {
			return err
		}
		return syncStores(ctx, tx, a.TenantID, n.ID, in.Role, in.StoreIDs)
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == usersEmailKey {
		return application.ErrEmailTaken
	}
	return err
}

func (r *Repository) UpdateStaff(ctx context.Context, a application.StaffActor, id uuid.UUID, c domain.StaffChange, now time.Time) (application.StaffMember, error) {
	var out application.StaffMember
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var current string
		err := tx.QueryRow(ctx, `SELECT status FROM users WHERE id = $1 FOR UPDATE`, id).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("kunci pengguna: %w", err)
		}
		if domain.UserStatus(current) == domain.UserDisabled && c.Status == domain.UserActive {
			if err := ensureUserCapacity(ctx, tx, a.TenantID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET name = $2, status = $3, updated_at = $4 WHERE id = $1`, id, c.Name, string(c.Status), now); err != nil {
			return fmt.Errorf("update pengguna: %w", err)
		}
		if err := syncStores(ctx, tx, a.TenantID, id, c.Role, c.StoreIDs); err != nil {
			return err
		}
		if c.Status == domain.UserDisabled {
			if err := revokeUserTokens(ctx, tx, id, now); err != nil {
				return err
			}
		}
		out, err = getStaff(ctx, tx, id)
		return err
	})
	return out, err
}

func (r *Repository) SetPassword(ctx context.Context, tenantID, id uuid.UUID, hash string, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET password_hash = $2, updated_at = $3 WHERE id = $1`, id, hash, now)
		if err != nil {
			return fmt.Errorf("ganti password: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return application.ErrUserNotFound
		}
		return revokeUserTokens(ctx, tx, id, now)
	})
}

func (r *Repository) SetPIN(ctx context.Context, tenantID, id uuid.UUID, hash *string, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET pin_hash = $2, updated_at = $3 WHERE id = $1`, id, hash, now)
		if err != nil {
			return fmt.Errorf("atur PIN: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return application.ErrUserNotFound
		}
		return nil
	})
}

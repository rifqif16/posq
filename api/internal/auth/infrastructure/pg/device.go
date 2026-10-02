package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/auth/application"
	"github.com/rifqif16/posq/api/internal/auth/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

var _ application.DeviceRepository = (*Repository)(nil)

const deviceSelect = `
	SELECT d.id, d.store_id, d.code, d.name, d.status, d.registered_at, d.last_seen_at, u.name
	FROM devices d JOIN users u ON u.id = d.registered_by`

func scanDevice(row pgx.Row) (application.Device, error) {
	var d application.Device
	err := row.Scan(&d.ID, &d.StoreID, &d.Code, &d.Name, &d.Status, &d.RegisteredAt, &d.LastSeenAt, &d.RegisteredBy)
	return d, err
}

func (r *Repository) ListDevices(ctx context.Context, tenantID uuid.UUID) ([]application.Device, error) {
	out := []application.Device{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, deviceSelect+` ORDER BY d.registered_at, d.id`)
		if err != nil {
			return fmt.Errorf("list perangkat: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			d, err := scanDevice(rows)
			if err != nil {
				return fmt.Errorf("scan perangkat: %w", err)
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) CreateDevice(ctx context.Context, a application.StaffActor, n application.NewDevice, now time.Time) (application.Device, error) {
	var out application.Device
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var one int
		err := tx.QueryRow(ctx, `SELECT 1 FROM user_store_roles WHERE user_id = $1 AND store_id = $2 LIMIT 1`, a.UserID, n.StoreID).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return &application.StoreInvalidError{Index: 0}
		}
		if err != nil {
			return fmt.Errorf("cek outlet: %w", err)
		}
		if err := ensureDeviceCapacity(ctx, tx, a.TenantID); err != nil {
			return err
		}
		var seq int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(substring(code from 2)::int), 0) + 1 FROM devices WHERE store_id = $1`, n.StoreID).Scan(&seq); err != nil {
			return fmt.Errorf("nomor perangkat: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO devices (id, tenant_id, store_id, code, name, secret_hash, registered_by, registered_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			n.ID, a.TenantID, n.StoreID, fmt.Sprintf("K%02d", seq), n.Name, n.SecretHash, a.UserID, now); err != nil {
			return fmt.Errorf("simpan perangkat: %w", err)
		}
		out, err = scanDevice(tx.QueryRow(ctx, deviceSelect+` WHERE d.id = $1`, n.ID))
		return err
	})
	return out, err
}

func ensureDeviceCapacity(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error {
	var limit *int
	err := tx.QueryRow(ctx, `SELECT (p.limits->>'devices')::int FROM tenants t JOIN plans p ON p.id = t.plan_id
		WHERE t.id = $1 FOR UPDATE OF t`, tenantID).Scan(&limit)
	if err != nil {
		return fmt.Errorf("baca batas paket: %w", err)
	}
	if limit == nil {
		return nil
	}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM devices WHERE status = 'active'`).Scan(&active); err != nil {
		return fmt.Errorf("hitung perangkat: %w", err)
	}
	if active >= *limit {
		return application.ErrDevicePlanLimit
	}
	return nil
}

func (r *Repository) RevokeDevice(ctx context.Context, tenantID, id uuid.UUID, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var got uuid.UUID
		err := tx.QueryRow(ctx, `UPDATE devices SET status = 'revoked', revoked_at = $2 WHERE id = $1 AND status = 'active' RETURNING id`, id, now).Scan(&got)
		if err == nil {
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("cabut perangkat: %w", err)
		}
		var one int
		err = tx.QueryRow(ctx, `SELECT 1 FROM devices WHERE id = $1`, id).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrDeviceNotFound
		}
		return err
	})
}

func (r *Repository) FindDevice(ctx context.Context, id uuid.UUID) (application.DeviceAuth, error) {
	var d application.DeviceAuth
	err := r.pool.QueryRow(ctx, `SELECT id, tenant_id, store_id, code, name, secret_hash, status FROM auth_find_device($1)`, id).
		Scan(&d.ID, &d.TenantID, &d.StoreID, &d.Code, &d.Name, &d.SecretHash, &d.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, application.ErrDeviceNotFound
	}
	if err != nil {
		return d, fmt.Errorf("cari perangkat: %w", err)
	}
	return d, nil
}

func (r *Repository) TouchDevice(ctx context.Context, tenantID, id uuid.UUID, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE devices SET last_seen_at = $2 WHERE id = $1`, id, now)
		return err
	})
}

func (r *Repository) PinUsers(ctx context.Context, tenantID, storeID uuid.UUID) ([]application.PinUser, error) {
	out := []application.PinUser{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT u.id, u.name, r.role FROM users u JOIN user_store_roles r ON r.user_id = u.id
			WHERE r.store_id = $1 AND r.role IN ('cashier','kitchen') AND u.status = 'active' AND u.pin_hash IS NOT NULL
			ORDER BY lower(u.name), u.id`, storeID)
		if err != nil {
			return fmt.Errorf("list pengguna PIN: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var u application.PinUser
			var role string
			if err := rows.Scan(&u.ID, &u.Name, &role); err != nil {
				return fmt.Errorf("scan pengguna PIN: %w", err)
			}
			u.Role = domain.Role(role)
			out = append(out, u)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) FindPinCandidate(ctx context.Context, tenantID, storeID, userID uuid.UUID) (application.PinCandidate, error) {
	var c application.PinCandidate
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var role, status string
		err := tx.QueryRow(ctx, `
			SELECT r.role, u.status, u.pin_hash, u.pin_locked_until
			FROM users u JOIN user_store_roles r ON r.user_id = u.id AND r.store_id = $2
			WHERE u.id = $1`, userID, storeID).Scan(&role, &status, &c.PINHash, &c.LockedUntil)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrUserNotFound
		}
		if err != nil {
			return fmt.Errorf("cari pengguna PIN: %w", err)
		}
		c.Role, c.Status = domain.Role(role), domain.UserStatus(status)
		return nil
	})
	return c, err
}

func (r *Repository) RecordPinFailure(ctx context.Context, tenantID, userID uuid.UUID, maxAttempts int, lockUntil time.Time) (bool, error) {
	locked := false
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			UPDATE users SET
			  pin_failed_attempts = CASE WHEN pin_failed_attempts + 1 >= $2 THEN 0 ELSE pin_failed_attempts + 1 END,
			  pin_locked_until = CASE WHEN pin_failed_attempts + 1 >= $2 THEN $3 ELSE pin_locked_until END
			WHERE id = $1 RETURNING pin_failed_attempts = 0`, userID, maxAttempts, lockUntil).Scan(&locked)
	})
	return locked, err
}

func (r *Repository) ResetPinFailures(ctx context.Context, tenantID, userID uuid.UUID) error {
	return database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE users SET pin_failed_attempts = 0, pin_locked_until = NULL
			WHERE id = $1 AND (pin_failed_attempts <> 0 OR pin_locked_until IS NOT NULL)`, userID)
		return err
	})
}

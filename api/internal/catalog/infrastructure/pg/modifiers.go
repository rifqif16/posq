package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

const (
	modifierGroupNameKey = "modifier_groups_name_key"
	groupColumns         = "g.id, g.name, g.min_select, g.max_select, g.version, g.created_at, g.updated_at"
)

var _ application.ModifierRepository = (*Repository)(nil)

func scanGroup(row pgx.Row) (domain.ModifierGroup, error) {
	var g domain.ModifierGroup
	err := row.Scan(&g.ID, &g.Name, &g.MinSelect, &g.MaxSelect, &g.Version, &g.CreatedAt, &g.UpdatedAt)
	return g, err
}

func mapModifierError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == modifierGroupNameKey {
		return application.ErrModifierGroupNameTaken
	}
	return err
}

func attachModifiers(ctx context.Context, tx pgx.Tx, groups []domain.ModifierGroup) error {
	if len(groups) == 0 {
		return nil
	}
	ids := make([]string, len(groups))
	index := make(map[uuid.UUID]int, len(groups))
	for i, g := range groups {
		ids[i] = g.ID.String()
		index[g.ID] = i
		groups[i].Modifiers = []domain.Modifier{}
	}
	rows, err := tx.Query(ctx, `SELECT group_id, id, name, price_delta, is_default, is_active FROM modifiers
		WHERE group_id = ANY($1::uuid[]) AND deleted_at IS NULL ORDER BY sort_order, id`, ids)
	if err != nil {
		return fmt.Errorf("baca opsi: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var gid uuid.UUID
		var m domain.Modifier
		if err := rows.Scan(&gid, &m.ID, &m.Name, &m.PriceDelta, &m.IsDefault, &m.IsActive); err != nil {
			return fmt.Errorf("scan opsi: %w", err)
		}
		i := index[gid]
		groups[i].Modifiers = append(groups[i].Modifiers, m)
	}
	return rows.Err()
}

func getGroup(ctx context.Context, tx pgx.Tx, id uuid.UUID) (domain.ModifierGroup, error) {
	g, err := scanGroup(tx.QueryRow(ctx, `SELECT `+groupColumns+` FROM modifier_groups g WHERE g.id = $1 AND g.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ModifierGroup{}, application.ErrNotFound
	}
	if err != nil {
		return domain.ModifierGroup{}, fmt.Errorf("baca grup: %w", err)
	}
	list := []domain.ModifierGroup{g}
	if err := attachModifiers(ctx, tx, list); err != nil {
		return domain.ModifierGroup{}, err
	}
	return list[0], nil
}

func (r *Repository) ListModifierGroups(ctx context.Context, tenantID uuid.UUID) ([]domain.ModifierGroup, error) {
	out := []domain.ModifierGroup{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+groupColumns+` FROM modifier_groups g WHERE g.deleted_at IS NULL ORDER BY lower(g.name), g.id`)
		if err != nil {
			return fmt.Errorf("list grup: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			g, err := scanGroup(rows)
			if err != nil {
				return fmt.Errorf("scan grup: %w", err)
			}
			out = append(out, g)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		return attachModifiers(ctx, tx, out)
	})
	return out, err
}

func (r *Repository) GetModifierGroup(ctx context.Context, tenantID, id uuid.UUID) (domain.ModifierGroup, error) {
	var out domain.ModifierGroup
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var err error
		out, err = getGroup(ctx, tx, id)
		return err
	})
	return out, err
}

func insertModifier(ctx context.Context, tx pgx.Tx, a application.Actor, groupID, id uuid.UUID, m domain.ModifierInput, order int, now time.Time) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO modifiers (id, tenant_id, group_id, name, price_delta, is_default, is_active, sort_order, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$10,$10)`,
		id, a.TenantID, groupID, m.Name, m.PriceDelta, m.IsDefault, m.IsActive, order, a.UserID, now)
	return err
}

func (r *Repository) CreateModifierGroup(ctx context.Context, a application.Actor, ng application.NewModifierGroup, now time.Time) (domain.ModifierGroup, error) {
	var out domain.ModifierGroup
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		in := ng.Input
		if _, err := tx.Exec(ctx, `
			INSERT INTO modifier_groups (id, tenant_id, name, min_select, max_select, created_by, updated_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$7)`,
			ng.ID, a.TenantID, in.Name, in.MinSelect, in.MaxSelect, a.UserID, now); err != nil {
			return err
		}
		for i, m := range in.Modifiers {
			if err := insertModifier(ctx, tx, a, ng.ID, m.NewID, m, i, now); err != nil {
				return err
			}
		}
		var err error
		out, err = getGroup(ctx, tx, ng.ID)
		return err
	})
	return out, mapModifierError(err)
}

func (r *Repository) UpdateModifierGroup(ctx context.Context, a application.Actor, id uuid.UUID, version int, in domain.ModifierGroupInput, now time.Time) (domain.ModifierGroup, error) {
	var out domain.ModifierGroup
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var current int
		err := tx.QueryRow(ctx, `SELECT version FROM modifier_groups WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("kunci grup: %w", err)
		}
		if current != version {
			return application.ErrVersionConflict
		}
		return applyGroupUpdate(ctx, tx, a, id, in, now, &out)
	})
	return out, mapModifierError(err)
}

func existingModifierIDs(ctx context.Context, tx pgx.Tx, groupID uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM modifiers WHERE group_id = $1 AND deleted_at IS NULL FOR UPDATE`, groupID)
	if err != nil {
		return nil, fmt.Errorf("kunci opsi: %w", err)
	}
	defer rows.Close()
	ids := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan opsi: %w", err)
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func applyGroupUpdate(ctx context.Context, tx pgx.Tx, a application.Actor, id uuid.UUID, in domain.ModifierGroupInput, now time.Time, out *domain.ModifierGroup) error {
	existing, err := existingModifierIDs(ctx, tx, id)
	if err != nil {
		return err
	}
	kept := map[uuid.UUID]bool{}
	for i, m := range in.Modifiers {
		if m.ID == nil {
			continue
		}
		if !existing[*m.ID] {
			return &application.InvalidModifierError{Index: i}
		}
		kept[*m.ID] = true
	}
	var keptIDs, removedIDs []uuid.UUID
	for mid := range existing {
		if kept[mid] {
			keptIDs = append(keptIDs, mid)
		} else {
			removedIDs = append(removedIDs, mid)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE modifier_groups SET name = $2, min_select = $3, max_select = $4,
		version = version + 1, updated_by = $5, updated_at = $6 WHERE id = $1`,
		id, in.Name, in.MinSelect, in.MaxSelect, a.UserID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE modifiers SET deleted_at = $2, updated_at = $2, updated_by = $3
		WHERE id = ANY($1::uuid[])`, uuidStrings(removedIDs), now, a.UserID); err != nil {
		return fmt.Errorf("hapus opsi: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE modifiers SET name = id::text WHERE id = ANY($1::uuid[])`, uuidStrings(keptIDs)); err != nil {
		return fmt.Errorf("parkir opsi: %w", err)
	}
	for i, m := range in.Modifiers {
		if m.ID == nil {
			if err := insertModifier(ctx, tx, a, id, m.NewID, m, i, now); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE modifiers SET name = $2, price_delta = $3, is_default = $4, is_active = $5,
			sort_order = $6, updated_by = $7, updated_at = $8 WHERE id = $1`,
			*m.ID, m.Name, m.PriceDelta, m.IsDefault, m.IsActive, i, a.UserID, now); err != nil {
			return err
		}
	}
	updated, err := getGroup(ctx, tx, id)
	*out = updated
	return err
}

func (r *Repository) DeleteModifierGroup(ctx context.Context, a application.Actor, id uuid.UUID, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var locked uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM modifier_groups WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("kunci grup: %w", err)
		}
		used, err := modifierGroupInUse(ctx, tx, id)
		if err != nil {
			return err
		}
		if used {
			return application.ErrModifierGroupInUse
		}
		if _, err := tx.Exec(ctx, `UPDATE modifier_groups SET deleted_at = $2, updated_at = $2, updated_by = $3 WHERE id = $1`,
			id, now, a.UserID); err != nil {
			return fmt.Errorf("hapus grup: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE modifiers SET deleted_at = $2, updated_at = $2, updated_by = $3
			WHERE group_id = $1 AND deleted_at IS NULL`, id, now, a.UserID); err != nil {
			return fmt.Errorf("hapus opsi: %w", err)
		}
		return nil
	})
}

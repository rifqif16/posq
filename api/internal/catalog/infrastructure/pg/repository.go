// Package pg (catalog): implementasi application.Repository di PostgreSQL (RLS via WithTenantTx).
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

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

const (
	uniqueViolation = "23505"
	categoryNameKey = "categories_name_key"
	categoryColumns = "id, parent_id, name, sort_order, created_at, updated_at"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ application.Repository = (*Repository)(nil)

func scanCategory(row pgx.Row) (domain.Category, error) {
	var c domain.Category
	err := row.Scan(&c.ID, &c.ParentID, &c.Name, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func mapWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == categoryNameKey {
		return application.ErrCategoryNameTaken
	}
	return err
}

func (r *Repository) ListCategories(ctx context.Context, tenantID uuid.UUID) ([]domain.Category, error) {
	out := []domain.Category{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+categoryColumns+` FROM categories
			WHERE deleted_at IS NULL ORDER BY sort_order, lower(name), id`)
		if err != nil {
			return fmt.Errorf("list kategori: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanCategory(rows)
			if err != nil {
				return fmt.Errorf("scan kategori: %w", err)
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}

func (r *Repository) CreateCategory(ctx context.Context, a application.Actor, c application.NewCategory, now time.Time) (domain.Category, error) {
	var created domain.Category
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		if c.ParentID != nil {
			var pid uuid.UUID
			err := tx.QueryRow(ctx, `SELECT id FROM categories
				WHERE id = $1 AND parent_id IS NULL AND deleted_at IS NULL FOR SHARE`, *c.ParentID).Scan(&pid)
			if errors.Is(err, pgx.ErrNoRows) {
				return application.ErrInvalidParent
			}
			if err != nil {
				return fmt.Errorf("cek induk: %w", err)
			}
		}
		var err error
		created, err = scanCategory(tx.QueryRow(ctx, `
			INSERT INTO categories (id, tenant_id, parent_id, name, sort_order, created_by, updated_by, created_at, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$6,$7,$7) RETURNING `+categoryColumns,
			c.ID, a.TenantID, c.ParentID, c.Name, c.SortOrder, a.UserID, now))
		return err
	})
	return created, mapWriteError(err)
}

func (r *Repository) UpdateCategory(ctx context.Context, a application.Actor, id uuid.UUID, name string, sortOrder int, now time.Time) (domain.Category, error) {
	var updated domain.Category
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var err error
		updated, err = scanCategory(tx.QueryRow(ctx, `
			UPDATE categories SET name = $2, sort_order = $3, updated_by = $4, updated_at = $5
			WHERE id = $1 AND deleted_at IS NULL RETURNING `+categoryColumns,
			id, name, sortOrder, a.UserID, now))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Category{}, application.ErrNotFound
	}
	return updated, mapWriteError(err)
}

func (r *Repository) DeleteCategory(ctx context.Context, a application.Actor, id uuid.UUID, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var locked uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("kunci kategori: %w", err)
		}
		var hasChildren bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM categories WHERE parent_id = $1 AND deleted_at IS NULL)`, id).Scan(&hasChildren)
		if err != nil {
			return fmt.Errorf("cek anak: %w", err)
		}
		var hasProducts bool
		err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE category_id = $1 AND deleted_at IS NULL)`, id).Scan(&hasProducts)
		if err != nil {
			return fmt.Errorf("cek produk: %w", err)
		}
		if hasChildren || hasProducts {
			return application.ErrCategoryInUse
		}
		_, err = tx.Exec(ctx, `UPDATE categories SET deleted_at = $2, updated_at = $2, updated_by = $3 WHERE id = $1`, id, now, a.UserID)
		if err != nil {
			return fmt.Errorf("hapus kategori: %w", err)
		}
		return nil
	})
}

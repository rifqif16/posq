package pg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rifqif16/posq/api/internal/inventory/application"
	"github.com/rifqif16/posq/api/internal/inventory/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

const movementSelect = `
	SELECT m.id, m.variant_id, p.name, v.name, v.sku, m.type, (m.qty_delta * 1000)::bigint, m.unit_cost, m.reason,
	       m.user_id, u.name, m.created_at
	FROM stock_movements m
	JOIN product_variants v ON v.id = m.variant_id
	JOIN products p ON p.id = v.product_id
	JOIN users u ON u.id = m.user_id`

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ application.Repository = (*Repository)(nil)

func checkStore(ctx context.Context, tx pgx.Tx, userID, storeID uuid.UUID) error {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM user_store_roles WHERE user_id = $1 AND store_id = $2 LIMIT 1`, userID, storeID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrStoreNotFound
	}
	if err != nil {
		return fmt.Errorf("cek outlet: %w", err)
	}
	return nil
}

func requireTracked(ctx context.Context, tx pgx.Tx, variantID uuid.UUID, index int) error {
	var tracked bool
	err := tx.QueryRow(ctx, `
		SELECT p.track_stock FROM product_variants v JOIN products p ON p.id = v.product_id
		WHERE v.id = $1 AND v.deleted_at IS NULL AND p.deleted_at IS NULL`, variantID).Scan(&tracked)
	if errors.Is(err, pgx.ErrNoRows) {
		return &application.VariantProblemError{Index: index}
	}
	if err != nil {
		return fmt.Errorf("cek varian: %w", err)
	}
	if !tracked {
		return &application.VariantProblemError{Index: index, NotTracked: true}
	}
	return nil
}

func scanMovement(row pgx.Row) (domain.Movement, error) {
	var m domain.Movement
	var typ string
	err := row.Scan(&m.ID, &m.VariantID, &m.ProductName, &m.VariantName, &m.SKU, &typ, &m.QtyDelta, &m.UnitCost,
		&m.Reason, &m.UserID, &m.UserName, &m.CreatedAt)
	m.Type = domain.MovementType(typ)
	return m, err
}

const insertMovement = `
	INSERT INTO stock_movements (id, tenant_id, store_id, variant_id, type, qty_delta, unit_cost, reason, user_id, created_at)
	VALUES ($1,$2,$3,$4,$5,($6::bigint)::numeric / 1000,$7,$8,$9,$10)`

func (r *Repository) RecordMovement(ctx context.Context, a application.Actor, id uuid.UUID, m domain.ValidatedMovement, now time.Time) (application.MovementResult, error) {
	var res application.MovementResult
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		if err := checkStore(ctx, tx, a.UserID, m.StoreID); err != nil {
			return err
		}
		if err := requireTracked(ctx, tx, m.VariantID, 0); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, insertMovement, id, a.TenantID, m.StoreID, m.VariantID, string(m.Type), m.QtyDelta, m.UnitCost, m.Reason, a.UserID, now); err != nil {
			return fmt.Errorf("catat pergerakan: %w", err)
		}
		err := tx.QueryRow(ctx, `
			INSERT INTO stock_levels (tenant_id, store_id, variant_id, qty_on_hand, updated_at)
			VALUES ($1,$2,$3,($4::bigint)::numeric / 1000,$5)
			ON CONFLICT (store_id, variant_id)
			DO UPDATE SET qty_on_hand = stock_levels.qty_on_hand + EXCLUDED.qty_on_hand, updated_at = EXCLUDED.updated_at
			RETURNING (qty_on_hand * 1000)::bigint`, a.TenantID, m.StoreID, m.VariantID, m.QtyDelta, now).Scan(&res.QtyOnHand)
		if err != nil {
			return fmt.Errorf("perbarui saldo: %w", err)
		}
		res.Movement, err = scanMovement(tx.QueryRow(ctx, movementSelect+` WHERE m.id = $1`, id))
		return err
	})
	return res, err
}

func (r *Repository) ApplyCounts(ctx context.Context, a application.Actor, c domain.ValidatedCount, ids []uuid.UUID, now time.Time) ([]application.CountResult, error) {
	results := make([]application.CountResult, len(c.Items))
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		if err := checkStore(ctx, tx, a.UserID, c.StoreID); err != nil {
			return err
		}
		order := make([]int, len(c.Items))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(x, y int) bool {
			return strings.Compare(c.Items[order[x]].VariantID.String(), c.Items[order[y]].VariantID.String()) < 0
		})
		for _, i := range order {
			res, err := countOne(ctx, tx, a, c, i, ids[i], now)
			if err != nil {
				return err
			}
			results[i] = res
		}
		return nil
	})
	return results, err
}

func countOne(ctx context.Context, tx pgx.Tx, a application.Actor, c domain.ValidatedCount, i int, movementID uuid.UUID, now time.Time) (application.CountResult, error) {
	item := c.Items[i]
	if err := requireTracked(ctx, tx, item.VariantID, i); err != nil {
		return application.CountResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_levels (tenant_id, store_id, variant_id, qty_on_hand, updated_at) VALUES ($1,$2,$3,0,$4)
		ON CONFLICT (store_id, variant_id) DO NOTHING`, a.TenantID, c.StoreID, item.VariantID, now); err != nil {
		return application.CountResult{}, fmt.Errorf("siapkan saldo: %w", err)
	}
	var before int64
	if err := tx.QueryRow(ctx, `SELECT (qty_on_hand * 1000)::bigint FROM stock_levels
		WHERE store_id = $1 AND variant_id = $2 FOR UPDATE`, c.StoreID, item.VariantID).Scan(&before); err != nil {
		return application.CountResult{}, fmt.Errorf("kunci saldo: %w", err)
	}
	res := application.CountResult{VariantID: item.VariantID, Before: before, Counted: item.Counted, Delta: item.Counted - before}
	if res.Delta == 0 {
		return res, nil
	}
	if _, err := tx.Exec(ctx, insertMovement, movementID, a.TenantID, c.StoreID, item.VariantID, string(domain.TypeOpname), res.Delta, nil, c.Reason, a.UserID, now); err != nil {
		return application.CountResult{}, fmt.Errorf("catat opname: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE stock_levels SET qty_on_hand = ($3::bigint)::numeric / 1000, updated_at = $4
		WHERE store_id = $1 AND variant_id = $2`, c.StoreID, item.VariantID, item.Counted, now); err != nil {
		return application.CountResult{}, fmt.Errorf("perbarui saldo: %w", err)
	}
	return res, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Repository) ListLevels(ctx context.Context, a application.Actor, f application.LevelFilter) (application.LevelPage, error) {
	page := application.LevelPage{Items: []domain.Level{}}
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		if err := checkStore(ctx, tx, a.UserID, f.StoreID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			SELECT p.id, v.id, p.name, v.name, v.sku, COALESCE((l.qty_on_hand * 1000)::bigint, 0), lower(p.name), lower(v.name)
			FROM product_variants v
			JOIN products p ON p.id = v.product_id
			LEFT JOIN stock_levels l ON l.variant_id = v.id AND l.store_id = $1
			WHERE v.deleted_at IS NULL AND p.deleted_at IS NULL AND p.track_stock
			  AND ($2::text = '' OR p.name ILIKE $3 OR v.name ILIKE $3 OR v.sku ILIKE $3
			       OR EXISTS (SELECT 1 FROM variant_barcodes b WHERE b.variant_id = v.id AND b.barcode = $2::text))
			  AND ($4::text IS NULL OR (lower(p.name), lower(v.name), v.id) > ($4::text, $5::text, $6::uuid))
			ORDER BY lower(p.name), lower(v.name), v.id
			LIMIT $7`,
			f.StoreID, f.Query, "%"+escapeLike(f.Query)+"%", f.AfterProduct, f.AfterVariant, f.AfterID, f.Limit+1)
		if err != nil {
			return fmt.Errorf("baca saldo: %w", err)
		}
		defer rows.Close()
		var productKeys, variantKeys []string
		for rows.Next() {
			var l domain.Level
			var pk, vk string
			if err := rows.Scan(&l.ProductID, &l.VariantID, &l.ProductName, &l.VariantName, &l.SKU, &l.QtyOnHand, &pk, &vk); err != nil {
				return fmt.Errorf("scan saldo: %w", err)
			}
			page.Items = append(page.Items, l)
			productKeys, variantKeys = append(productKeys, pk), append(variantKeys, vk)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(page.Items) > f.Limit {
			page.HasMore = true
			page.Items = page.Items[:f.Limit]
		}
		if n := len(page.Items); n > 0 {
			page.LastProduct, page.LastVariant = productKeys[n-1], variantKeys[n-1]
		}
		return nil
	})
	return page, err
}

func (r *Repository) ListMovements(ctx context.Context, a application.Actor, f application.MovementFilter) (application.MovementPage, error) {
	page := application.MovementPage{Items: []domain.Movement{}}
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		if err := checkStore(ctx, tx, a.UserID, f.StoreID); err != nil {
			return err
		}
		var typ *string
		if f.Type != "" {
			typ = &f.Type
		}
		rows, err := tx.Query(ctx, movementSelect+`
			WHERE m.store_id = $1
			  AND ($2::uuid IS NULL OR m.variant_id = $2::uuid)
			  AND ($3::text IS NULL OR m.type = $3::text)
			  AND ($4::timestamptz IS NULL OR (m.created_at, m.id) < ($4::timestamptz, $5::uuid))
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $6`, f.StoreID, f.VariantID, typ, f.AfterAt, f.AfterID, f.Limit+1)
		if err != nil {
			return fmt.Errorf("baca pergerakan: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			m, err := scanMovement(rows)
			if err != nil {
				return fmt.Errorf("scan pergerakan: %w", err)
			}
			page.Items = append(page.Items, m)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		if len(page.Items) > f.Limit {
			page.HasMore = true
			page.Items = page.Items[:f.Limit]
		}
		return nil
	})
	return page, err
}

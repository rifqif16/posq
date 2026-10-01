package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

var _ application.CSVRepository = (*Repository)(nil)

func collectStrings(ctx context.Context, tx pgx.Tx, query string, candidates []string) (map[string]bool, error) {
	found := map[string]bool{}
	if len(candidates) == 0 {
		return found, nil
	}
	rows, err := tx.Query(ctx, query, candidates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		found[v] = true
	}
	return found, rows.Err()
}

func (r *Repository) FindTakenProductCodes(ctx context.Context, tenantID uuid.UUID, c application.CodeCandidates) (application.TakenCodes, error) {
	taken := application.TakenCodes{}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var err error
		if taken.SKUs, err = collectStrings(ctx, tx,
			`SELECT lower(sku) FROM product_variants WHERE deleted_at IS NULL AND lower(sku) = ANY($1::text[])`, c.SKUs); err != nil {
			return fmt.Errorf("cek sku: %w", err)
		}
		if taken.Barcodes, err = collectStrings(ctx, tx,
			`SELECT barcode FROM variant_barcodes WHERE barcode = ANY($1::text[])`, c.Barcodes); err != nil {
			return fmt.Errorf("cek barcode: %w", err)
		}
		if taken.Names, err = collectStrings(ctx, tx,
			`SELECT lower(name) FROM products WHERE deleted_at IS NULL AND lower(name) = ANY($1::text[])`, c.Names); err != nil {
			return fmt.Errorf("cek nama produk: %w", err)
		}
		return nil
	})
	return taken, err
}

func (r *Repository) CreateProducts(ctx context.Context, a application.Actor, products []application.NewProduct, now time.Time) error {
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		for _, np := range products {
			in := np.Input
			if err := checkCategory(ctx, tx, in.CategoryID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO products (id, tenant_id, name, type, category_id, taxable, track_stock, kitchen_station, is_active, created_by, updated_by, created_at, updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11,$11)`,
				np.ID, a.TenantID, in.Name, domain.ProductType(len(in.Variants)), in.CategoryID, in.Taxable, in.TrackStock,
				nullable(in.KitchenStation), in.IsActive, a.UserID, now); err != nil {
				return fmt.Errorf("insert produk: %w", err)
			}
			for i, v := range in.Variants {
				if err := insertVariant(ctx, tx, a, np.ID, v, i == 0, now); err != nil {
					return err
				}
			}
			if err := replaceModifierLinks(ctx, tx, a.TenantID, np.ID, in.ModifierGroupIDs); err != nil {
				return err
			}
		}
		return nil
	})
	return mapProductError(err)
}

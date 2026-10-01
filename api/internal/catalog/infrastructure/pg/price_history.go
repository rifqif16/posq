package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

var _ application.PriceHistoryRepository = (*Repository)(nil)

func (r *Repository) ListPriceHistory(ctx context.Context, tenantID uuid.UUID, f application.PriceHistoryFilter) (application.PriceHistoryPage, error) {
	page := application.PriceHistoryPage{Items: []domain.PriceChange{}}
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var exists uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM products WHERE id = $1 AND deleted_at IS NULL`, f.ProductID).Scan(&exists)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("cek produk: %w", err)
		}
		rows, err := tx.Query(ctx, `
			SELECT h.id, h.variant_id, v.name, h.field, h.old_value, h.new_value, h.changed_by, u.name, h.at
			FROM product_price_history h
			JOIN product_variants v ON v.id = h.variant_id
			JOIN users u ON u.id = h.changed_by
			WHERE v.product_id = $1
			  AND ($2::timestamptz IS NULL OR (h.at, h.id) < ($2::timestamptz, $3::uuid))
			ORDER BY h.at DESC, h.id DESC
			LIMIT $4`, f.ProductID, f.AfterAt, f.AfterID, f.Limit+1)
		if err != nil {
			return fmt.Errorf("baca riwayat harga: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var c domain.PriceChange
			if err := rows.Scan(&c.ID, &c.VariantID, &c.VariantName, &c.Field, &c.OldValue, &c.NewValue, &c.ChangedByID, &c.ChangedByName, &c.At); err != nil {
				return fmt.Errorf("scan riwayat harga: %w", err)
			}
			page.Items = append(page.Items, c)
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

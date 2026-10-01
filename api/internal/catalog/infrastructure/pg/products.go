package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
	"github.com/rifqif16/posq/api/internal/platform/database"
)

const (
	skuKey         = "product_variants_sku_key"
	barcodeKey     = "variant_barcodes_barcode_key"
	productColumns = "p.id, p.name, p.type, p.category_id, p.taxable, p.track_stock, p.kitchen_station, p.is_active, p.version, p.created_at, p.updated_at"
)

var _ application.ProductRepository = (*Repository)(nil)

func scanProduct(row pgx.Row, extra ...any) (domain.Product, error) {
	var p domain.Product
	var station *string
	dest := append([]any{&p.ID, &p.Name, &p.Type, &p.CategoryID, &p.Taxable, &p.TrackStock, &station,
		&p.IsActive, &p.Version, &p.CreatedAt, &p.UpdatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return p, err
	}
	if station != nil {
		p.KitchenStation = *station
	}
	return p, nil
}

func mapProductError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		switch pgErr.ConstraintName {
		case skuKey:
			return application.ErrSKUTaken
		case barcodeKey:
			return application.ErrBarcodeTaken
		}
	}
	return err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func checkCategory(ctx context.Context, tx pgx.Tx, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	var found uuid.UUID
	err := tx.QueryRow(ctx, `SELECT id FROM categories WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, *id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrInvalidCategory
	}
	if err != nil {
		return fmt.Errorf("cek kategori: %w", err)
	}
	return nil
}

func insertBarcodes(ctx context.Context, tx pgx.Tx, tenantID, variantID uuid.UUID, barcodes []string) error {
	for _, b := range barcodes {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("buat id barcode: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO variant_barcodes (id, tenant_id, variant_id, barcode) VALUES ($1,$2,$3,$4)`,
			id, tenantID, variantID, b); err != nil {
			return err
		}
	}
	return nil
}

func insertPriceHistory(ctx context.Context, tx pgx.Tx, a application.Actor, variantID uuid.UUID, field string, oldV, newV int64, now time.Time) error {
	if oldV == newV {
		return nil
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("buat id riwayat: %w", err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO product_price_history (id, tenant_id, variant_id, field, old_value, new_value, changed_by, at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, id, a.TenantID, variantID, field, oldV, newV, a.UserID, now)
	return err
}

func attachVariants(ctx context.Context, tx pgx.Tx, products []domain.Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]string, len(products))
	index := make(map[uuid.UUID]int, len(products))
	for i, p := range products {
		ids[i] = p.ID.String()
		index[p.ID] = i
	}
	rows, err := tx.Query(ctx, `SELECT v.id, v.product_id, v.name, v.sku, v.cost_price, v.sell_price, v.is_default, v.is_active
		FROM product_variants v WHERE v.product_id = ANY($1::uuid[]) AND v.deleted_at IS NULL
		ORDER BY v.is_default DESC, v.id`, ids)
	if err != nil {
		return fmt.Errorf("baca varian: %w", err)
	}
	defer rows.Close()
	variantIDs := []string{}
	type ref struct{ product, variant int }
	refs := map[uuid.UUID]ref{}
	for rows.Next() {
		var v domain.Variant
		var productID uuid.UUID
		if err := rows.Scan(&v.ID, &productID, &v.Name, &v.SKU, &v.CostPrice, &v.SellPrice, &v.IsDefault, &v.IsActive); err != nil {
			return fmt.Errorf("scan varian: %w", err)
		}
		v.Barcodes = []string{}
		pi := index[productID]
		products[pi].Variants = append(products[pi].Variants, v)
		refs[v.ID] = ref{pi, len(products[pi].Variants) - 1}
		variantIDs = append(variantIDs, v.ID.String())
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	bRows, err := tx.Query(ctx, `SELECT variant_id, barcode FROM variant_barcodes WHERE variant_id = ANY($1::uuid[]) ORDER BY id`, variantIDs)
	if err != nil {
		return fmt.Errorf("baca barcode: %w", err)
	}
	defer bRows.Close()
	for bRows.Next() {
		var vid uuid.UUID
		var code string
		if err := bRows.Scan(&vid, &code); err != nil {
			return fmt.Errorf("scan barcode: %w", err)
		}
		r := refs[vid]
		v := &products[r.product].Variants[r.variant]
		v.Barcodes = append(v.Barcodes, code)
	}
	return bRows.Err()
}

func getProduct(ctx context.Context, tx pgx.Tx, id uuid.UUID) (domain.Product, error) {
	p, err := scanProduct(tx.QueryRow(ctx, `SELECT `+productColumns+` FROM products p WHERE p.id = $1 AND p.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Product{}, application.ErrNotFound
	}
	if err != nil {
		return domain.Product{}, fmt.Errorf("baca produk: %w", err)
	}
	list := []domain.Product{p}
	if err := attachVariants(ctx, tx, list); err != nil {
		return domain.Product{}, err
	}
	if err := attachModifierGroupIDs(ctx, tx, list); err != nil {
		return domain.Product{}, err
	}
	return list[0], nil
}

func (r *Repository) GetProduct(ctx context.Context, tenantID, id uuid.UUID) (domain.Product, error) {
	var out domain.Product
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		var err error
		out, err = getProduct(ctx, tx, id)
		return err
	})
	return out, err
}

func insertVariant(ctx context.Context, tx pgx.Tx, a application.Actor, productID uuid.UUID, v domain.VariantInput, isDefault bool, now time.Time) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO product_variants (id, tenant_id, product_id, name, sku, cost_price, sell_price, is_default, is_active, created_by, updated_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10,$11,$11)`,
		v.NewID, a.TenantID, productID, v.Name, v.SKU, v.CostPrice, v.SellPrice, isDefault, v.IsActive, a.UserID, now); err != nil {
		return err
	}
	return insertBarcodes(ctx, tx, a.TenantID, v.NewID, v.Barcodes)
}

func (r *Repository) CreateProduct(ctx context.Context, a application.Actor, np application.NewProduct, now time.Time) (domain.Product, error) {
	var out domain.Product
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
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
		var err error
		out, err = getProduct(ctx, tx, np.ID)
		return err
	})
	return out, mapProductError(err)
}

func (r *Repository) UpdateProduct(ctx context.Context, a application.Actor, id uuid.UUID, version int, in domain.ProductInput, now time.Time) (domain.Product, error) {
	var out domain.Product
	err := database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var current int
		err := tx.QueryRow(ctx, `SELECT version FROM products WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("kunci produk: %w", err)
		}
		if current != version {
			return application.ErrVersionConflict
		}
		if err := checkCategory(ctx, tx, in.CategoryID); err != nil {
			return err
		}
		return applyUpdate(ctx, tx, a, id, in, now, &out)
	})
	return out, mapProductError(err)
}

type variantState struct {
	sku              string
	costPrice, price int64
}

func loadVariantStates(ctx context.Context, tx pgx.Tx, productID uuid.UUID) (map[uuid.UUID]variantState, error) {
	rows, err := tx.Query(ctx, `SELECT id, sku, cost_price, sell_price FROM product_variants
		WHERE product_id = $1 AND deleted_at IS NULL FOR UPDATE`, productID)
	if err != nil {
		return nil, fmt.Errorf("kunci varian: %w", err)
	}
	defer rows.Close()
	states := map[uuid.UUID]variantState{}
	for rows.Next() {
		var id uuid.UUID
		var st variantState
		if err := rows.Scan(&id, &st.sku, &st.costPrice, &st.price); err != nil {
			return nil, fmt.Errorf("scan varian: %w", err)
		}
		states[id] = st
	}
	return states, rows.Err()
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func applyUpdate(ctx context.Context, tx pgx.Tx, a application.Actor, id uuid.UUID, in domain.ProductInput, now time.Time, out *domain.Product) error {
	existing, err := loadVariantStates(ctx, tx, id)
	if err != nil {
		return err
	}
	kept := map[uuid.UUID]bool{}
	for i, v := range in.Variants {
		if v.ID == nil {
			continue
		}
		if _, ok := existing[*v.ID]; !ok {
			return &application.InvalidVariantError{Index: i}
		}
		kept[*v.ID] = true
	}
	var keptIDs, removedIDs, allIDs []uuid.UUID
	for vid := range existing {
		allIDs = append(allIDs, vid)
		if kept[vid] {
			keptIDs = append(keptIDs, vid)
		} else {
			removedIDs = append(removedIDs, vid)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE products SET name = $2, type = $3, category_id = $4, taxable = $5, track_stock = $6, kitchen_station = $7,
		       is_active = $8, version = version + 1, updated_by = $9, updated_at = $10 WHERE id = $1`,
		id, in.Name, domain.ProductType(len(in.Variants)), in.CategoryID, in.Taxable, in.TrackStock,
		nullable(in.KitchenStation), in.IsActive, a.UserID, now); err != nil {
		return fmt.Errorf("update produk: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE product_variants SET deleted_at = $2, is_default = false, updated_at = $2, updated_by = $3
		WHERE id = ANY($1::uuid[])`, uuidStrings(removedIDs), now, a.UserID); err != nil {
		return fmt.Errorf("hapus varian: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM variant_barcodes WHERE variant_id = ANY($1::uuid[])`, uuidStrings(allIDs)); err != nil {
		return fmt.Errorf("hapus barcode lama: %w", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE product_variants SET name = id::text, sku = id::text, is_default = false
		WHERE id = ANY($1::uuid[])`, uuidStrings(keptIDs)); err != nil {
		return fmt.Errorf("parkir varian: %w", err)
	}
	for i, v := range in.Variants {
		if v.ID == nil {
			if err := insertVariant(ctx, tx, a, id, v, i == 0, now); err != nil {
				return err
			}
			continue
		}
		if err := updateVariant(ctx, tx, a, v, existing[*v.ID], i == 0, now); err != nil {
			return err
		}
	}
	if err := replaceModifierLinks(ctx, tx, a.TenantID, id, in.ModifierGroupIDs); err != nil {
		return err
	}
	updated, err := getProduct(ctx, tx, id)
	*out = updated
	return err
}

func updateVariant(ctx context.Context, tx pgx.Tx, a application.Actor, v domain.VariantInput, old variantState, isDefault bool, now time.Time) error {
	sku := v.SKU
	if sku == "" {
		sku = old.sku // SKU kosong = pertahankan
	}
	if _, err := tx.Exec(ctx, `
		UPDATE product_variants SET name = $2, sku = $3, cost_price = $4, sell_price = $5, is_default = $6, is_active = $7,
		       updated_by = $8, updated_at = $9 WHERE id = $1`,
		*v.ID, v.Name, sku, v.CostPrice, v.SellPrice, isDefault, v.IsActive, a.UserID, now); err != nil {
		return err
	}
	if err := insertPriceHistory(ctx, tx, a, *v.ID, "cost_price", old.costPrice, v.CostPrice, now); err != nil {
		return err
	}
	if err := insertPriceHistory(ctx, tx, a, *v.ID, "sell_price", old.price, v.SellPrice, now); err != nil {
		return err
	}
	return insertBarcodes(ctx, tx, a.TenantID, *v.ID, v.Barcodes)
}

func (r *Repository) DeleteProduct(ctx context.Context, a application.Actor, id uuid.UUID, now time.Time) error {
	return database.WithTenantTx(ctx, r.pool, a.TenantID, func(tx pgx.Tx) error {
		var locked uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM products WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return application.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("kunci produk: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE products SET deleted_at = $2, is_active = false, version = version + 1,
			updated_at = $2, updated_by = $3 WHERE id = $1`, id, now, a.UserID); err != nil {
			return fmt.Errorf("hapus produk: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE product_variants SET deleted_at = $2, updated_at = $2, updated_by = $3
			WHERE product_id = $1 AND deleted_at IS NULL`, id, now, a.UserID); err != nil {
			return fmt.Errorf("hapus varian: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM variant_barcodes
			WHERE variant_id IN (SELECT id FROM product_variants WHERE product_id = $1)`, id); err != nil {
			return fmt.Errorf("hapus barcode: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM product_modifier_groups WHERE product_id = $1`, id); err != nil {
			return fmt.Errorf("hapus penautan grup: %w", err)
		}
		return nil
	})
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Repository) ListProducts(ctx context.Context, tenantID uuid.UUID, f application.ProductFilter) (application.ProductPage, error) {
	var page application.ProductPage
	err := database.WithTenantTx(ctx, r.pool, tenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+productColumns+`, lower(p.name) FROM products p
			WHERE p.deleted_at IS NULL
			  AND ($1::text = '' OR p.name ILIKE $2
			       OR EXISTS (SELECT 1 FROM product_variants v WHERE v.product_id = p.id AND v.deleted_at IS NULL AND v.sku ILIKE $2)
			       OR EXISTS (SELECT 1 FROM product_variants v JOIN variant_barcodes b ON b.variant_id = v.id
			                  WHERE v.product_id = p.id AND v.deleted_at IS NULL AND b.barcode = $1::text))
			  AND ($3::uuid IS NULL OR p.category_id = $3::uuid)
			  AND ($4::text IS NULL OR (lower(p.name), p.id) > ($4::text, $5::uuid))
			ORDER BY lower(p.name), p.id LIMIT $6`,
			f.Query, "%"+escapeLike(f.Query)+"%", f.CategoryID, f.AfterKey, f.AfterID, f.Limit+1)
		if err != nil {
			return fmt.Errorf("list produk: %w", err)
		}
		defer rows.Close()
		keys := []string{}
		for rows.Next() {
			var key string
			p, err := scanProduct(rows, &key)
			if err != nil {
				return fmt.Errorf("scan produk: %w", err)
			}
			page.Items = append(page.Items, p)
			keys = append(keys, key)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		if len(page.Items) > f.Limit {
			page.HasMore = true
			page.Items = page.Items[:f.Limit]
		}
		if n := len(page.Items); n > 0 {
			page.LastKey = keys[n-1]
		}
		if err := attachVariants(ctx, tx, page.Items); err != nil {
			return err
		}
		return attachModifierGroupIDs(ctx, tx, page.Items)
	})
	if page.Items == nil {
		page.Items = []domain.Product{}
	}
	return page, err
}

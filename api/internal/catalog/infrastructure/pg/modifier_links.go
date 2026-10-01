package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/catalog/application"
	"github.com/rifqif16/posq/api/internal/catalog/domain"
)

// replaceModifierLinks membangun ulang penautan produk–grup sesuai urutan input.
// Setiap grup dikunci FOR SHARE agar tidak terhapus bersamaan (DeleteModifierGroup memakai FOR UPDATE).
func replaceModifierLinks(ctx context.Context, tx pgx.Tx, tenantID, productID uuid.UUID, groupIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, `DELETE FROM product_modifier_groups WHERE product_id = $1`, productID); err != nil {
		return fmt.Errorf("hapus penautan lama: %w", err)
	}
	for i, gid := range groupIDs {
		var locked uuid.UUID
		err := tx.QueryRow(ctx, `SELECT id FROM modifier_groups WHERE id = $1 AND deleted_at IS NULL FOR SHARE`, gid).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return &application.InvalidModifierGroupError{Index: i}
		}
		if err != nil {
			return fmt.Errorf("kunci grup: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO product_modifier_groups (tenant_id, product_id, group_id, sort_order) VALUES ($1,$2,$3,$4)`,
			tenantID, productID, gid, i); err != nil {
			return fmt.Errorf("tautkan grup: %w", err)
		}
	}
	return nil
}

func attachModifierGroupIDs(ctx context.Context, tx pgx.Tx, products []domain.Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]string, len(products))
	index := make(map[uuid.UUID]int, len(products))
	for i, p := range products {
		ids[i] = p.ID.String()
		index[p.ID] = i
		products[i].ModifierGroupIDs = []uuid.UUID{}
	}
	rows, err := tx.Query(ctx, `SELECT product_id, group_id FROM product_modifier_groups
		WHERE product_id = ANY($1::uuid[]) ORDER BY sort_order`, ids)
	if err != nil {
		return fmt.Errorf("baca penautan grup: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var pid, gid uuid.UUID
		if err := rows.Scan(&pid, &gid); err != nil {
			return fmt.Errorf("scan penautan grup: %w", err)
		}
		i := index[pid]
		products[i].ModifierGroupIDs = append(products[i].ModifierGroupIDs, gid)
	}
	return rows.Err()
}

func modifierGroupInUse(ctx context.Context, tx pgx.Tx, groupID uuid.UUID) (bool, error) {
	var used bool
	err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM product_modifier_groups WHERE group_id = $1)`, groupID).Scan(&used)
	if err != nil {
		return false, fmt.Errorf("cek pemakaian grup: %w", err)
	}
	return used, nil
}

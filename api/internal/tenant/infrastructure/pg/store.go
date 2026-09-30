// Package pg (tenant): akses tabel tenants dan stores. Hanya boleh dipanggil
// dari transaksi yang sudah ber-scope tenant (database.WithTenantTx).
package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rifqif16/posq/api/internal/tenant/domain"
)

type Seed struct {
	TenantID     uuid.UUID
	StoreID      uuid.UUID
	BusinessName string
	TrialEndsAt  time.Time
}

// InsertWithFirstStore membuat tenant (status trial, paket trial) beserta outlet pertama.
func InsertWithFirstStore(ctx context.Context, tx pgx.Tx, s Seed) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO tenants (id, name, status, plan_id, trial_ends_at)
		SELECT $1, $2, $3, p.id, $4 FROM plans p WHERE p.code = $5`,
		s.TenantID, s.BusinessName, string(domain.StatusTrial), s.TrialEndsAt, domain.TrialPlanCode)
	if err != nil {
		return fmt.Errorf("insert tenant: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO stores (id, tenant_id, code, name) VALUES ($1, $2, $3, $4)`,
		s.StoreID, s.TenantID, domain.DefaultStoreCode, s.BusinessName)
	if err != nil {
		return fmt.Errorf("insert store: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("insert store: baris terpengaruh %d", tag.RowsAffected())
	}
	return nil
}

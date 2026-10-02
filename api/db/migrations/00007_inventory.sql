-- +goose Up
ALTER TABLE stores ADD CONSTRAINT stores_tenant_id_id_key UNIQUE (tenant_id, id);

CREATE TABLE stock_movements (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    store_id   uuid NOT NULL,
    variant_id uuid NOT NULL,
    type       text NOT NULL CHECK (type IN ('sale','refund','void','purchase_receive','adjustment','opname','transfer_in','transfer_out','waste','recipe_consume')),
    qty_delta  numeric(12,3) NOT NULL CHECK (qty_delta <> 0),
    unit_cost  bigint CHECK (unit_cost IS NULL OR unit_cost >= 0),
    reason     text NOT NULL DEFAULT '' CHECK (char_length(reason) <= 200),
    ref_type   text,
    ref_id     uuid,
    user_id    uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores (tenant_id, id),
    FOREIGN KEY (tenant_id, variant_id) REFERENCES product_variants (tenant_id, id)
);
CREATE INDEX stock_movements_store_time_idx ON stock_movements (store_id, created_at DESC, id DESC);
CREATE INDEX stock_movements_variant_idx ON stock_movements (variant_id, created_at DESC);

CREATE TABLE stock_levels (
    tenant_id   uuid NOT NULL REFERENCES tenants (id),
    store_id    uuid NOT NULL,
    variant_id  uuid NOT NULL,
    qty_on_hand numeric(14,3) NOT NULL DEFAULT 0,
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (store_id, variant_id),
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores (tenant_id, id),
    FOREIGN KEY (tenant_id, variant_id) REFERENCES product_variants (tenant_id, id)
);

ALTER TABLE stock_movements ENABLE ROW LEVEL SECURITY;
ALTER TABLE stock_levels    ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON stock_movements
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON stock_levels
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

GRANT SELECT, INSERT ON stock_movements TO posq_app;
GRANT SELECT, INSERT, UPDATE ON stock_levels TO posq_app;

-- +goose Down
DROP TABLE IF EXISTS stock_levels;
DROP TABLE IF EXISTS stock_movements;
ALTER TABLE stores DROP CONSTRAINT IF EXISTS stores_tenant_id_id_key;

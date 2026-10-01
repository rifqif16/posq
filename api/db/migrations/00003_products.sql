-- +goose Up
CREATE TABLE products (
    id              uuid PRIMARY KEY,
    tenant_id       uuid NOT NULL REFERENCES tenants (id),
    name            text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    type            text NOT NULL CHECK (type IN ('simple','variant')),
    category_id     uuid,
    taxable         boolean NOT NULL DEFAULT true,
    track_stock     boolean NOT NULL DEFAULT false,
    kitchen_station text CHECK (kitchen_station ~ '^[a-z0-9_-]{1,30}$'),
    is_active       boolean NOT NULL DEFAULT true,
    version         integer NOT NULL DEFAULT 1,
    created_by      uuid NOT NULL REFERENCES users (id),
    updated_by      uuid NOT NULL REFERENCES users (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, category_id) REFERENCES categories (tenant_id, id)
);
CREATE INDEX products_list_idx ON products (tenant_id, lower(name), id) WHERE deleted_at IS NULL;
CREATE INDEX products_category_idx ON products (category_id) WHERE deleted_at IS NULL;

CREATE TABLE product_variants (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    product_id uuid NOT NULL,
    name       text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    sku        text NOT NULL CHECK (sku ~ '^[A-Za-z0-9._-]{1,64}$'),
    cost_price bigint NOT NULL DEFAULT 0 CHECK (cost_price >= 0),
    sell_price bigint NOT NULL CHECK (sell_price >= 0),
    is_default boolean NOT NULL DEFAULT false,
    created_by uuid NOT NULL REFERENCES users (id),
    updated_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id)
);
-- SKU unik per tenant (case-insensitive) di antara varian aktif.
CREATE UNIQUE INDEX product_variants_sku_key ON product_variants (tenant_id, lower(sku)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX product_variants_default_key ON product_variants (product_id) WHERE is_default AND deleted_at IS NULL;
CREATE INDEX product_variants_product_idx ON product_variants (product_id) WHERE deleted_at IS NULL;

CREATE TABLE variant_barcodes (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    variant_id uuid NOT NULL,
    barcode    text NOT NULL CHECK (barcode ~ '^[A-Za-z0-9._-]{1,64}$'),
    FOREIGN KEY (tenant_id, variant_id) REFERENCES product_variants (tenant_id, id)
);
CREATE UNIQUE INDEX variant_barcodes_barcode_key ON variant_barcodes (tenant_id, barcode);
CREATE INDEX variant_barcodes_variant_idx ON variant_barcodes (variant_id);

CREATE TABLE product_price_history (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    variant_id uuid NOT NULL,
    field      text NOT NULL CHECK (field IN ('cost_price','sell_price')),
    old_value  bigint NOT NULL,
    new_value  bigint NOT NULL,
    changed_by uuid NOT NULL REFERENCES users (id),
    at         timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, variant_id) REFERENCES product_variants (tenant_id, id)
);
CREATE INDEX product_price_history_variant_idx ON product_price_history (variant_id, at DESC);

ALTER TABLE products              ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_variants      ENABLE ROW LEVEL SECURITY;
ALTER TABLE variant_barcodes      ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_price_history ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON products
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON product_variants
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON variant_barcodes
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON product_price_history
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- Produk/varian: soft delete lewat UPDATE. Barcode boleh dihapus (dibebaskan untuk dipakai ulang).
-- Riwayat harga append-only.
GRANT SELECT, INSERT, UPDATE ON products, product_variants TO posq_app;
GRANT SELECT, INSERT, DELETE ON variant_barcodes TO posq_app;
GRANT SELECT, INSERT ON product_price_history TO posq_app;

-- +goose Down
DROP TABLE IF EXISTS product_price_history;
DROP TABLE IF EXISTS variant_barcodes;
DROP TABLE IF EXISTS product_variants;
DROP TABLE IF EXISTS products;

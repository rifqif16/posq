-- +goose Up
ALTER TABLE product_variants ADD COLUMN is_active boolean NOT NULL DEFAULT true;

-- Nama varian unik per produk (case-insensitive) di antara varian aktif.
CREATE UNIQUE INDEX product_variants_name_key ON product_variants (product_id, lower(name)) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS product_variants_name_key;
ALTER TABLE product_variants DROP COLUMN IF EXISTS is_active;

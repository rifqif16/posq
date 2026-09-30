-- +goose Up
CREATE TABLE categories (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    parent_id  uuid,
    name       text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    sort_order integer NOT NULL DEFAULT 0,
    created_by uuid NOT NULL REFERENCES users (id),
    updated_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    UNIQUE (tenant_id, id),
    -- Induk wajib berasal dari tenant yang sama (FK komposit; RLS tidak berlaku pada pengecekan FK).
    FOREIGN KEY (tenant_id, parent_id) REFERENCES categories (tenant_id, id)
);

-- Nama unik per induk (case-insensitive) di antara kategori aktif; induk NULL = tingkat atas.
CREATE UNIQUE INDEX categories_name_key ON categories
    (tenant_id, COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name))
    WHERE deleted_at IS NULL;
CREATE INDEX categories_parent_idx ON categories (parent_id) WHERE deleted_at IS NULL;

ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON categories
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- Soft delete lewat UPDATE; tanpa hak DELETE.
GRANT SELECT, INSERT, UPDATE ON categories TO posq_app;

-- +goose Down
DROP TABLE IF EXISTS categories;

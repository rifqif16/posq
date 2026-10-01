-- +goose Up
CREATE TABLE modifier_groups (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    name       text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    -- Wajib/opsional diturunkan: min_select >= 1 berarti wajib.
    min_select integer NOT NULL CHECK (min_select >= 0),
    max_select integer NOT NULL CHECK (max_select >= 1),
    version    integer NOT NULL DEFAULT 1,
    created_by uuid NOT NULL REFERENCES users (id),
    updated_by uuid NOT NULL REFERENCES users (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK (max_select >= min_select),
    UNIQUE (tenant_id, id)
);
CREATE UNIQUE INDEX modifier_groups_name_key ON modifier_groups (tenant_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE modifiers (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants (id),
    group_id    uuid NOT NULL,
    name        text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    price_delta bigint NOT NULL DEFAULT 0 CHECK (price_delta >= 0),
    is_default  boolean NOT NULL DEFAULT false,
    is_active   boolean NOT NULL DEFAULT true,
    sort_order  integer NOT NULL DEFAULT 0,
    created_by  uuid NOT NULL REFERENCES users (id),
    updated_by  uuid NOT NULL REFERENCES users (id),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    deleted_at  timestamptz,
    UNIQUE (tenant_id, id),
    FOREIGN KEY (tenant_id, group_id) REFERENCES modifier_groups (tenant_id, id)
);
CREATE UNIQUE INDEX modifiers_name_key ON modifiers (group_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX modifiers_group_idx ON modifiers (group_id, sort_order) WHERE deleted_at IS NULL;

ALTER TABLE modifier_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE modifiers       ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON modifier_groups
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON modifiers
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- Soft delete lewat UPDATE; tanpa hak DELETE.
GRANT SELECT, INSERT, UPDATE ON modifier_groups, modifiers TO posq_app;

-- +goose Down
DROP TABLE IF EXISTS modifiers;
DROP TABLE IF EXISTS modifier_groups;

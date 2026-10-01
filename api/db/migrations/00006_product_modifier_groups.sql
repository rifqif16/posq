-- +goose Up
CREATE TABLE product_modifier_groups (
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    product_id uuid NOT NULL,
    group_id   uuid NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, group_id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id),
    FOREIGN KEY (tenant_id, group_id) REFERENCES modifier_groups (tenant_id, id)
);
CREATE INDEX product_modifier_groups_group_idx ON product_modifier_groups (group_id);

ALTER TABLE product_modifier_groups ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON product_modifier_groups
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- Penautan dibangun ulang pada setiap perubahan produk, jadi butuh DELETE.
GRANT SELECT, INSERT, DELETE ON product_modifier_groups TO posq_app;

-- +goose Down
DROP TABLE IF EXISTS product_modifier_groups;

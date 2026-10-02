-- +goose Up
CREATE TABLE devices (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants (id),
    store_id      uuid NOT NULL,
    code          text NOT NULL,
    name          text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    secret_hash   bytea NOT NULL,
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','revoked')),
    registered_by uuid NOT NULL REFERENCES users (id),
    registered_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz,
    revoked_at    timestamptz,
    FOREIGN KEY (tenant_id, store_id) REFERENCES stores (tenant_id, id),
    UNIQUE (store_id, code)
);
CREATE INDEX devices_tenant_idx ON devices (tenant_id);

ALTER TABLE users
    ADD COLUMN pin_failed_attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN pin_locked_until timestamptz;

ALTER TABLE devices ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON devices
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- +goose StatementBegin
CREATE FUNCTION auth_find_device(p_id uuid)
RETURNS TABLE (id uuid, tenant_id uuid, store_id uuid, code text, name text, secret_hash bytea, status text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
    SELECT d.id, d.tenant_id, d.store_id, d.code, d.name, d.secret_hash, d.status
    FROM devices d WHERE d.id = p_id
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth_find_device(uuid) FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE ON devices TO posq_app;
GRANT EXECUTE ON FUNCTION auth_find_device(uuid) TO posq_app;

-- +goose Down
DROP FUNCTION IF EXISTS auth_find_device(uuid);
ALTER TABLE users DROP COLUMN IF EXISTS pin_locked_until, DROP COLUMN IF EXISTS pin_failed_attempts;
DROP TABLE IF EXISTS devices;

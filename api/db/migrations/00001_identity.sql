-- +goose Up
CREATE TABLE plans (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code       text NOT NULL UNIQUE,
    name       text NOT NULL,
    limits     jsonb NOT NULL DEFAULT '{}',
    features   jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Batas paket final menunggu Q17; nilai ini placeholder trial.
INSERT INTO plans (code, name, limits) VALUES
    ('trial', 'Trial', '{"stores":1,"users":5,"devices":3,"products":500}');

CREATE TABLE tenants (
    id            uuid PRIMARY KEY,
    name          text NOT NULL,
    status        text NOT NULL CHECK (status IN ('trial','active','past_due','suspended','cancelled')),
    plan_id       uuid NOT NULL REFERENCES plans (id),
    trial_ends_at timestamptz,
    settings      jsonb NOT NULL DEFAULT '{}',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE stores (
    id         uuid PRIMARY KEY,
    tenant_id  uuid NOT NULL REFERENCES tenants (id),
    code       text NOT NULL,
    name       text NOT NULL,
    address    text NOT NULL DEFAULT '',
    timezone   text NOT NULL DEFAULT 'Asia/Jakarta',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, code)
);

CREATE TABLE users (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants (id),
    email         text NOT NULL CHECK (email = lower(email)),
    password_hash text NOT NULL,
    pin_hash      text,
    name          text NOT NULL,
    status        text NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
-- Email unik global: login memakai email saja tanpa memilih tenant.
CREATE UNIQUE INDEX users_email_key ON users (email);
CREATE INDEX users_tenant_idx ON users (tenant_id);

CREATE TABLE user_store_roles (
    user_id   uuid NOT NULL REFERENCES users (id),
    store_id  uuid NOT NULL REFERENCES stores (id),
    tenant_id uuid NOT NULL REFERENCES tenants (id),
    role      text NOT NULL CHECK (role IN ('owner','admin','cashier','kitchen')),
    PRIMARY KEY (user_id, store_id)
);
CREATE INDEX user_store_roles_tenant_idx ON user_store_roles (tenant_id);

CREATE TABLE refresh_tokens (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants (id),
    user_id     uuid NOT NULL REFERENCES users (id),
    family_id   uuid NOT NULL,
    token_hash  bytea NOT NULL UNIQUE,
    expires_at  timestamptz NOT NULL,
    revoked_at  timestamptz,
    replaced_by uuid,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- Row-Level Security: setiap transaksi wajib set_config('app.tenant_id', ..., true).
ALTER TABLE tenants          ENABLE ROW LEVEL SECURITY;
ALTER TABLE stores           ENABLE ROW LEVEL SECURITY;
ALTER TABLE users            ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_store_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE refresh_tokens   ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON tenants
    USING (id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON stores
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON users
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON user_store_roles
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);
CREATE POLICY tenant_isolation ON refresh_tokens
    USING (tenant_id = nullif(current_setting('app.tenant_id', true), '')::uuid);

-- Login mencari user lewat email sebelum tenant diketahui. Fungsi SECURITY DEFINER
-- adalah satu-satunya jalan lintas-tenant, dan hanya membaca satu baris berdasarkan email.
-- +goose StatementBegin
CREATE FUNCTION auth_find_user_by_email(p_email text)
RETURNS TABLE (id uuid, tenant_id uuid, name text, email text, password_hash text, status text)
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = public AS $$
    SELECT u.id, u.tenant_id, u.name, u.email, u.password_hash, u.status
    FROM users u WHERE u.email = p_email
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION auth_find_user_by_email(text) FROM PUBLIC;

GRANT SELECT ON plans TO posq_app;
GRANT SELECT, INSERT, UPDATE ON tenants, stores, users, user_store_roles, refresh_tokens TO posq_app;
GRANT EXECUTE ON FUNCTION auth_find_user_by_email(text) TO posq_app;

-- +goose Down
DROP FUNCTION IF EXISTS auth_find_user_by_email(text);
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_store_roles;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS stores;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS plans;

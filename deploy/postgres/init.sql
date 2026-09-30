-- Dijalankan sekali saat volume Postgres dibuat (docker-entrypoint-initdb.d).
-- Role aplikasi tidak memiliki BYPASSRLS dan bukan owner tabel, sehingga RLS berlaku (ADR-08).
-- Password di sini hanya untuk pengembangan lokal.
CREATE ROLE posq_app LOGIN PASSWORD 'posq_app_dev' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
GRANT CONNECT ON DATABASE posq TO posq_app;
GRANT USAGE ON SCHEMA public TO posq_app;

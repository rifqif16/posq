-- +goose Up
GRANT DELETE ON user_store_roles TO posq_app;

-- +goose Down
REVOKE DELETE ON user_store_roles FROM posq_app;

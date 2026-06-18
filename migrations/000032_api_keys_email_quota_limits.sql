-- +goose Up
ALTER TABLE api_keys ADD COLUMN email_quota_limits JSONB;

-- +goose Down
ALTER TABLE api_keys DROP COLUMN email_quota_limits;

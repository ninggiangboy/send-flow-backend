-- +goose Up
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    secret_hash TEXT NOT NULL,
    scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_workspace_status_created ON api_keys(workspace_id, status, created_at DESC);

-- +goose Down
DROP TABLE api_keys;

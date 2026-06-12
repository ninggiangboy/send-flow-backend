-- +goose Up

CREATE TABLE IF NOT EXISTS permission_registry (
    bit BIGINT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Permission bit: audit.read = 1<<25
INSERT INTO permission_registry (bit, name, description, created_at)
VALUES (1<<25, 'audit.read', 'View workspace audit logs', NOW())
ON CONFLICT (bit) DO NOTHING;

UPDATE roles r
SET permissions_mask = r.permissions_mask | (1<<25),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

CREATE TABLE workspace_settings (
    workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    settings_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_user_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT chk_workspace_settings_version CHECK (version >= 1)
);

CREATE TABLE audit_entries (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_user_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
    action_type TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id TEXT NOT NULL DEFAULT '',
    payload_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    request_id TEXT NOT NULL DEFAULT '',
    occurred_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_audit_entries_ws_occurred_id
    ON audit_entries(workspace_id, occurred_at DESC, id DESC);

CREATE INDEX idx_audit_entries_ws_actor_occurred
    ON audit_entries(workspace_id, actor_user_id, occurred_at DESC)
    WHERE actor_user_id IS NOT NULL;

CREATE INDEX idx_audit_entries_ws_action_occurred
    ON audit_entries(workspace_id, action_type, occurred_at DESC);

CREATE INDEX idx_audit_entries_ws_target_occurred
    ON audit_entries(workspace_id, target_type, target_id, occurred_at DESC)
    WHERE target_id <> '';

-- +goose Down
DROP INDEX IF EXISTS idx_audit_entries_ws_target_occurred;
DROP INDEX IF EXISTS idx_audit_entries_ws_action_occurred;
DROP INDEX IF EXISTS idx_audit_entries_ws_actor_occurred;
DROP INDEX IF EXISTS idx_audit_entries_ws_occurred_id;
DROP TABLE IF EXISTS audit_entries;
DROP TABLE IF EXISTS workspace_settings;

UPDATE roles r
SET permissions_mask = r.permissions_mask & ~(1<<25),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

DELETE FROM permission_registry WHERE bit = (1<<25);

-- +goose Up
CREATE TABLE suppression_entries (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    email_normalized TEXT NOT NULL,
    scope TEXT NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    removed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_suppression_active_entry ON suppression_entries(workspace_id, email_normalized, scope, reason) WHERE status = 'active';
CREATE INDEX idx_suppression_workspace_status_created ON suppression_entries(workspace_id, status, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS suppression_entries;

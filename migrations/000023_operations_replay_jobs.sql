-- +goose Up

-- Permission bits: operations.queue.read = 1<<22, operations.dlq.read = 1<<23, operations.replay.manage = 1<<24
UPDATE roles r
SET permissions_mask = r.permissions_mask | ((1<<22) | (1<<23) | (1<<24)),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

CREATE TABLE replay_jobs (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    requested_by_user_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    filter_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    result_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_replay_jobs_ws_created
    ON replay_jobs(workspace_id, created_at DESC);

CREATE INDEX idx_replay_jobs_ws_status_created
    ON replay_jobs(workspace_id, status, created_at DESC);

CREATE INDEX idx_replay_jobs_target
    ON replay_jobs(target_type, target_id);

-- Add workspace_id to dead_letter_records for workspace-scoped listing
ALTER TABLE dead_letter_records ADD COLUMN IF NOT EXISTS workspace_id TEXT NULL REFERENCES workspaces(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_dead_letter_records_workspace_id
    ON dead_letter_records(workspace_id, failed_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_dead_letter_records_workspace_id;
ALTER TABLE dead_letter_records DROP COLUMN IF EXISTS workspace_id;

DROP TABLE IF EXISTS replay_jobs;

UPDATE roles r
SET permissions_mask = r.permissions_mask & ~((1<<22) | (1<<23) | (1<<24)),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

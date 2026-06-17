-- +goose Up
-- +goose StatementBegin

-- === transactional_send_requests: additive columns for template/raw mode ===
ALTER TABLE transactional_send_requests
    ADD COLUMN IF NOT EXISTS mode TEXT NOT NULL DEFAULT 'template',
    ADD COLUMN IF NOT EXISTS subject TEXT,
    ADD COLUMN IF NOT EXISTS sender_name TEXT,
    ADD COLUMN IF NOT EXISTS source_api_key_id TEXT,
    ADD COLUMN IF NOT EXISTS request_hash TEXT,
    ADD COLUMN IF NOT EXISTS total_recipients INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS recipient_terminal_total INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS recipient_success_total INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS recipient_failure_total INT NOT NULL DEFAULT 0;

-- === messages: additive columns for per-recipient role and mode ===
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS subject TEXT,
    ADD COLUMN IF NOT EXISTS sender_name TEXT,
    ADD COLUMN IF NOT EXISTS recipient_role TEXT NOT NULL DEFAULT 'to',
    ADD COLUMN IF NOT EXISTS source_api_key_id TEXT,
    ADD COLUMN IF NOT EXISTS text_body TEXT,
    ADD COLUMN IF NOT EXISTS html_body TEXT;

-- === transactional_attachments: new table ===
CREATE TABLE IF NOT EXISTS transactional_attachments (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    transactional_request_id TEXT NOT NULL REFERENCES transactional_send_requests(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL,
    sha256_digest TEXT NOT NULL,
    disposition TEXT NOT NULL DEFAULT 'attachment',
    content_id TEXT,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tx_attachments_request ON transactional_attachments(workspace_id, transactional_request_id);
CREATE INDEX IF NOT EXISTS idx_tx_attachments_digest ON transactional_attachments(sha256_digest);

-- === message_events: immutable timeline table ===
CREATE TABLE IF NOT EXISTS message_events (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    transactional_request_id TEXT,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    reason_code TEXT,
    reason_message TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_msg_events_ws_msg ON message_events(workspace_id, message_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_msg_events_ws_request ON message_events(workspace_id, transactional_request_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_msg_events_ws_event_type ON message_events(workspace_id, event_type, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_msg_events_ws_created ON message_events(workspace_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS message_events;
DROP TABLE IF EXISTS transactional_attachments;

ALTER TABLE messages
    DROP COLUMN IF EXISTS html_body,
    DROP COLUMN IF EXISTS text_body,
    DROP COLUMN IF EXISTS source_api_key_id,
    DROP COLUMN IF EXISTS recipient_role,
    DROP COLUMN IF EXISTS sender_name,
    DROP COLUMN IF EXISTS subject;

ALTER TABLE transactional_send_requests
    DROP COLUMN IF EXISTS mode,
    DROP COLUMN IF EXISTS subject,
    DROP COLUMN IF EXISTS sender_name,
    DROP COLUMN IF EXISTS source_api_key_id,
    DROP COLUMN IF EXISTS request_hash,
    DROP COLUMN IF EXISTS total_recipients,
    DROP COLUMN IF EXISTS recipient_terminal_total,
    DROP COLUMN IF EXISTS recipient_success_total,
    DROP COLUMN IF EXISTS recipient_failure_total;

-- +goose StatementEnd

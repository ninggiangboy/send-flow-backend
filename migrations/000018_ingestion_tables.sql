-- +goose Up
CREATE TABLE provider_webhook_events (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    provider_event_id TEXT,
    provider_message_id TEXT,
    workspace_id TEXT NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    message_id TEXT NULL REFERENCES messages(id) ON DELETE SET NULL,
    event_type TEXT,
    payload_json JSONB NOT NULL,
    headers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    signature_valid BOOLEAN NOT NULL DEFAULT FALSE,
    received_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX uq_provider_webhook_events_dedupe
    ON provider_webhook_events(provider, provider_event_id)
    WHERE provider_event_id IS NOT NULL;

CREATE INDEX idx_provider_webhook_events_provider_message
    ON provider_webhook_events(provider, provider_message_id);

CREATE INDEX idx_provider_webhook_events_workspace_message_time
    ON provider_webhook_events(workspace_id, message_id, received_at DESC);

CREATE INDEX idx_provider_webhook_events_type_time
    ON provider_webhook_events(event_type, received_at DESC);

CREATE TABLE normalized_provider_events (
    id TEXT PRIMARY KEY,
    raw_event_id TEXT NOT NULL REFERENCES provider_webhook_events(id) ON DELETE CASCADE,
    workspace_id TEXT NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    message_id TEXT NULL REFERENCES messages(id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    provider_event_id TEXT,
    provider_message_id TEXT,
    event_type TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX uq_normalized_provider_events_dedupe
    ON normalized_provider_events(provider, provider_event_id, event_type)
    WHERE provider_event_id IS NOT NULL;

CREATE INDEX idx_normalized_provider_events_workspace_message_time
    ON normalized_provider_events(workspace_id, message_id, occurred_at DESC);

CREATE INDEX idx_normalized_provider_events_provider_message
    ON normalized_provider_events(provider, provider_message_id);

CREATE INDEX idx_normalized_provider_events_type_time
    ON normalized_provider_events(event_type, occurred_at DESC);

-- +goose Down
DROP TABLE normalized_provider_events;
DROP TABLE provider_webhook_events;

-- +goose Up
CREATE TABLE tracking_links (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    destination_url TEXT NOT NULL,
    link_type TEXT NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ
);

CREATE INDEX idx_tracking_links_workspace_message
    ON tracking_links(workspace_id, message_id);

CREATE INDEX idx_tracking_links_workspace_created
    ON tracking_links(workspace_id, created_at DESC);

CREATE TABLE tracking_events (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    tracking_link_id TEXT NULL REFERENCES tracking_links(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    source TEXT NOT NULL,
    source_event_id TEXT,
    provider TEXT,
    provider_event_id TEXT,
    provider_message_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_tracking_events_workspace_message_time
    ON tracking_events(workspace_id, message_id, occurred_at DESC);

CREATE INDEX idx_tracking_events_workspace_type_time
    ON tracking_events(workspace_id, event_type, occurred_at DESC);

CREATE INDEX idx_tracking_events_link_time
    ON tracking_events(tracking_link_id, occurred_at DESC)
    WHERE tracking_link_id IS NOT NULL;

CREATE UNIQUE INDEX uq_tracking_events_source_idempotency
    ON tracking_events(source, source_event_id, event_type)
    WHERE source_event_id IS NOT NULL;

-- +goose Down
DROP TABLE tracking_events;
DROP TABLE tracking_links;

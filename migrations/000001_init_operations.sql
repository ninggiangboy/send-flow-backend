-- +goose Up
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    workspace_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate_id ON outbox_events (aggregate_id);
CREATE INDEX IF NOT EXISTS idx_outbox_events_event_type ON outbox_events (event_type);
CREATE INDEX IF NOT EXISTS idx_outbox_events_workspace_id ON outbox_events (workspace_id);

CREATE TABLE IF NOT EXISTS processed_event_markers (
    consumer_name TEXT NOT NULL,
    event_id UUID NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (consumer_name, event_id)
);

CREATE TABLE IF NOT EXISTS dead_letter_records (
    id UUID PRIMARY KEY,
    source TEXT NOT NULL,
    event_id UUID,
    payload JSONB NOT NULL,
    error_message TEXT NOT NULL,
    failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retryable BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_dead_letter_records_source ON dead_letter_records (source);
CREATE INDEX IF NOT EXISTS idx_dead_letter_records_failed_at ON dead_letter_records (failed_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dead_letter_records;
DROP TABLE IF EXISTS processed_event_markers;
DROP TABLE IF EXISTS outbox_events;

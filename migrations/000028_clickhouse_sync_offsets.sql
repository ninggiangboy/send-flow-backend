-- +goose Up
CREATE TABLE clickhouse_sync_offsets (
    stream_name TEXT PRIMARY KEY,
    last_created_at TIMESTAMPTZ,
    last_fact_id TEXT NOT NULL DEFAULT '',
    last_synced_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS clickhouse_sync_offsets;

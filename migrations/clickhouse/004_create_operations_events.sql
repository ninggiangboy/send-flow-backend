CREATE TABLE IF NOT EXISTS operations_events (
    source_event_id String,
    source String,
    source_event_type String,
    operation_type String,
    status String,
    workspace_id String,
    error_type String,
    consumer String,
    target String,
    metadata_json String,
    occurred_at DateTime64(3),
    created_at DateTime64(3)
) ENGINE = ReplacingMergeTree()
ORDER BY (workspace_id, source, occurred_at, source_event_id);

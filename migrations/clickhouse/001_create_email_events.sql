CREATE TABLE IF NOT EXISTS email_events (
    source_event_id String,
    source_event_type String,
    workspace_id String,
    campaign_id String,
    message_id String,
    provider String,
    provider_message_id String,
    provider_event_id String,
    event_type String,
    recipient_domain String,
    occurred_at DateTime64(3),
    received_at DateTime64(3),
    metadata_json String,
    created_at DateTime64(3)
) ENGINE = ReplacingMergeTree()
ORDER BY (workspace_id, campaign_id, occurred_at, source_event_id);

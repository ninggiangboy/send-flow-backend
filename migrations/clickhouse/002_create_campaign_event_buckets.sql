CREATE TABLE IF NOT EXISTS campaign_event_buckets (
    workspace_id String,
    campaign_id String,
    bucket_start DateTime64(3),
    event_type String,
    cnt UInt64,
    last_occurred_at DateTime64(3)
) ENGINE = SummingMergeTree((cnt))
ORDER BY (workspace_id, campaign_id, bucket_start, event_type);

CREATE MATERIALIZED VIEW IF NOT EXISTS campaign_event_buckets_mv
TO campaign_event_buckets
AS SELECT
    workspace_id,
    campaign_id,
    toStartOfHour(occurred_at) AS bucket_start,
    event_type,
    count() AS cnt,
    max(occurred_at) AS last_occurred_at
FROM email_events
GROUP BY workspace_id, campaign_id, bucket_start, event_type;

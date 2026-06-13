CREATE TABLE IF NOT EXISTS deliverability_buckets (
    workspace_id String,
    provider String,
    recipient_domain String,
    bucket_start DateTime64(3),
    event_type String,
    cnt UInt64,
    last_occurred_at DateTime64(3)
) ENGINE = SummingMergeTree((cnt))
ORDER BY (workspace_id, provider, recipient_domain, bucket_start, event_type);

CREATE MATERIALIZED VIEW IF NOT EXISTS deliverability_buckets_mv
TO deliverability_buckets
AS SELECT
    workspace_id,
    provider,
    recipient_domain,
    toStartOfHour(occurred_at) AS bucket_start,
    event_type,
    count() AS cnt,
    max(occurred_at) AS last_occurred_at
FROM email_events
GROUP BY workspace_id, provider, recipient_domain, bucket_start, event_type;

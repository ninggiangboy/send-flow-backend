CREATE TABLE IF NOT EXISTS anomaly_signals (
    anomaly_id String,
    anomaly_type String,
    severity String,
    metric String,
    observed Float64,
    expected Float64,
    deviation Float64,
    workspace_id String,
    window_start DateTime64(3),
    window_end DateTime64(3),
    detected_at DateTime64(3)
) ENGINE = ReplacingMergeTree()
ORDER BY (workspace_id, window_start, anomaly_type, anomaly_id);

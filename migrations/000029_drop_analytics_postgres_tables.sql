-- +goose Up
DROP TABLE IF EXISTS analytics_event_facts;
DROP TABLE IF EXISTS campaign_delivery_summaries;
DROP TABLE IF EXISTS workspace_analytics_overviews;
DROP TABLE IF EXISTS deliverability_projections;
DROP TABLE IF EXISTS clickhouse_sync_offsets;

-- +goose Down
-- intentionally empty: tables moved to ClickHouse

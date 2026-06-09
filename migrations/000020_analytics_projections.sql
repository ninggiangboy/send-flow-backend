-- +goose Up
CREATE TABLE analytics_event_facts (
    id TEXT PRIMARY KEY,
    source_event_id UUID NOT NULL,
    source_event_type TEXT NOT NULL,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    campaign_id TEXT NULL REFERENCES campaigns(id) ON DELETE SET NULL,
    message_id TEXT NULL REFERENCES messages(id) ON DELETE SET NULL,
    provider TEXT,
    provider_message_id TEXT,
    provider_event_id TEXT,
    event_type TEXT NOT NULL,
    recipient_domain TEXT,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_analytics_event_facts_source_event
    ON analytics_event_facts(source_event_id);

CREATE INDEX idx_analytics_facts_ws_occurred
    ON analytics_event_facts(workspace_id, occurred_at DESC);

CREATE INDEX idx_analytics_facts_ws_event_occurred
    ON analytics_event_facts(workspace_id, event_type, occurred_at DESC);

CREATE INDEX idx_analytics_facts_ws_campaign_occurred
    ON analytics_event_facts(workspace_id, campaign_id, occurred_at DESC)
    WHERE campaign_id IS NOT NULL;

CREATE INDEX idx_analytics_facts_ws_message_occurred
    ON analytics_event_facts(workspace_id, message_id, occurred_at DESC)
    WHERE message_id IS NOT NULL;

CREATE INDEX idx_analytics_facts_ws_domain_occurred
    ON analytics_event_facts(workspace_id, recipient_domain, occurred_at DESC)
    WHERE recipient_domain IS NOT NULL;

CREATE TABLE campaign_delivery_summaries (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    queued_count BIGINT NOT NULL DEFAULT 0,
    accepted_count BIGINT NOT NULL DEFAULT 0,
    delivered_count BIGINT NOT NULL DEFAULT 0,
    bounced_count BIGINT NOT NULL DEFAULT 0,
    complained_count BIGINT NOT NULL DEFAULT 0,
    opened_count BIGINT NOT NULL DEFAULT 0,
    clicked_count BIGINT NOT NULL DEFAULT 0,
    unsubscribed_count BIGINT NOT NULL DEFAULT 0,
    retry_scheduled_count BIGINT NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, campaign_id)
);

CREATE INDEX idx_campaign_summaries_ws_updated
    ON campaign_delivery_summaries(workspace_id, last_updated_at DESC);

CREATE TABLE workspace_analytics_overviews (
    workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    queued_count BIGINT NOT NULL DEFAULT 0,
    accepted_count BIGINT NOT NULL DEFAULT 0,
    delivered_count BIGINT NOT NULL DEFAULT 0,
    bounced_count BIGINT NOT NULL DEFAULT 0,
    complained_count BIGINT NOT NULL DEFAULT 0,
    opened_count BIGINT NOT NULL DEFAULT 0,
    clicked_count BIGINT NOT NULL DEFAULT 0,
    unsubscribed_count BIGINT NOT NULL DEFAULT 0,
    retry_scheduled_count BIGINT NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE deliverability_projections (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT '',
    recipient_domain TEXT NOT NULL DEFAULT '',
    delivered_count BIGINT NOT NULL DEFAULT 0,
    bounced_count BIGINT NOT NULL DEFAULT 0,
    complained_count BIGINT NOT NULL DEFAULT 0,
    opened_count BIGINT NOT NULL DEFAULT 0,
    clicked_count BIGINT NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ,
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, provider, recipient_domain)
);

CREATE INDEX idx_deliverability_projections_ws_updated
    ON deliverability_projections(workspace_id, last_updated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS deliverability_projections;
DROP TABLE IF EXISTS workspace_analytics_overviews;
DROP TABLE IF EXISTS campaign_delivery_summaries;
DROP TABLE IF EXISTS analytics_event_facts;

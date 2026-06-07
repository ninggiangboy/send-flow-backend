-- +goose Up
CREATE TABLE campaigns (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    audience_type TEXT NOT NULL,
    audience_id TEXT,
    audience_contact_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    template_id TEXT NOT NULL,
    template_version_id TEXT,
    sender_domain_id TEXT NOT NULL,
    message_type TEXT NOT NULL,
    scheduled_at TIMESTAMPTZ,
    planned_recipients BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    cancelled_at TIMESTAMPTZ,
    paused_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_campaigns_workspace_status_scheduled ON campaigns(workspace_id, status, scheduled_at);
CREATE INDEX idx_campaigns_workspace_created ON campaigns(workspace_id, created_at DESC);
CREATE INDEX idx_campaigns_workspace_template ON campaigns(workspace_id, template_id);
CREATE INDEX idx_campaigns_workspace_sender ON campaigns(workspace_id, sender_domain_id);

CREATE TABLE campaign_message_candidates (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    contact_id TEXT NOT NULL,
    email_normalized TEXT NOT NULL,
    recipient_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX idx_candidates_campaign_contact ON campaign_message_candidates(campaign_id, contact_id);
CREATE INDEX idx_candidates_workspace_campaign_created ON campaign_message_candidates(workspace_id, campaign_id, created_at DESC);
CREATE INDEX idx_candidates_workspace_status_created ON campaign_message_candidates(workspace_id, status, created_at DESC);
CREATE INDEX idx_candidates_workspace_email ON campaign_message_candidates(workspace_id, email_normalized);

-- +goose Down
DROP TABLE IF EXISTS campaign_message_candidates;
DROP TABLE IF EXISTS campaigns;

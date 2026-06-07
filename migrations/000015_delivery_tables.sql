-- +goose Up
CREATE TABLE transactional_send_requests (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    idempotency_key TEXT,
    status TEXT NOT NULL,
    request_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_transactional_send_requests_idempotency ON transactional_send_requests(workspace_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    campaign_id TEXT,
    campaign_candidate_id TEXT,
    transactional_request_id TEXT REFERENCES transactional_send_requests(id) ON DELETE SET NULL,
    contact_id TEXT,
    recipient_email_normalized TEXT NOT NULL,
    recipient_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    template_id TEXT,
    template_version_id TEXT,
    sender_domain_id TEXT NOT NULL,
    message_type TEXT NOT NULL,
    source_type TEXT NOT NULL,
    status TEXT NOT NULL,
    scheduled_at TIMESTAMPTZ,
    queued_at TIMESTAMPTZ,
    processing_started_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    bounced_at TIMESTAMPTZ,
    complained_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    last_error_class TEXT,
    last_error_message TEXT,
    provider TEXT,
    provider_message_id TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX uq_messages_campaign_candidate ON messages(campaign_id, campaign_candidate_id) WHERE campaign_id IS NOT NULL AND campaign_candidate_id IS NOT NULL;
CREATE UNIQUE INDEX uq_messages_provider_ref ON messages(provider, provider_message_id) WHERE provider IS NOT NULL AND provider_message_id IS NOT NULL;
CREATE INDEX idx_messages_ws_status_scheduled ON messages(workspace_id, status, scheduled_at, created_at);
CREATE INDEX idx_messages_ws_campaign ON messages(workspace_id, campaign_id, created_at DESC);
CREATE INDEX idx_messages_ws_recipient ON messages(workspace_id, recipient_email_normalized, created_at DESC);
CREATE INDEX idx_messages_ws_provider ON messages(workspace_id, provider_message_id);
CREATE INDEX idx_messages_ws_tx_request ON messages(workspace_id, transactional_request_id);

CREATE TABLE delivery_attempts (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    attempt_no INT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    request_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_class TEXT,
    error_message TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_delivery_attempts_message_attempt ON delivery_attempts(message_id, attempt_no);
CREATE INDEX idx_delivery_attempts_ws_msg ON delivery_attempts(workspace_id, message_id, attempt_no DESC);
CREATE INDEX idx_delivery_attempts_ws_status ON delivery_attempts(workspace_id, status, started_at DESC);

CREATE TABLE retry_states (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ,
    last_error_class TEXT,
    last_error_message TEXT,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX uq_retry_states_message ON retry_states(message_id);
CREATE INDEX idx_retry_states_ws_status ON retry_states(workspace_id, status, next_attempt_at);

-- +goose Down
DROP TABLE IF EXISTS retry_states;
DROP TABLE IF EXISTS delivery_attempts;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS transactional_send_requests;

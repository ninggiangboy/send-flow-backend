-- +goose Up

-- Permission bits: webhook.manage = 1<<19, webhook.delivery.read = 1<<20, webhook.delivery.retry = 1<<21
UPDATE roles r
SET permissions_mask = r.permissions_mask | ((1<<19) | (1<<20) | (1<<21)),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

CREATE TABLE customer_webhooks (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    target_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    subscriptions_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    secret_hash TEXT NOT NULL,
    secret_hint TEXT NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1,
    created_by_user_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disabled_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_customer_webhooks_ws_status_created
    ON customer_webhooks(workspace_id, status, created_at DESC);

CREATE UNIQUE INDEX uq_customer_webhooks_ws_name_active
    ON customer_webhooks(workspace_id, name)
    WHERE status = 'active';

CREATE TABLE customer_webhook_deliveries (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    webhook_id TEXT NOT NULL REFERENCES customer_webhooks(id) ON DELETE CASCADE,
    source_event_id UUID NOT NULL,
    source_event_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    target_url TEXT NOT NULL,
    attempt_count BIGINT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NULL,
    last_attempt_at TIMESTAMPTZ NULL,
    last_status_code INTEGER NULL,
    last_error TEXT NOT NULL DEFAULT '',
    request_headers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_headers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    event_payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_webhook_deliveries_webhook_event
    ON customer_webhook_deliveries(webhook_id, source_event_id);

CREATE INDEX idx_webhook_deliveries_ws_created
    ON customer_webhook_deliveries(workspace_id, created_at DESC);

CREATE INDEX idx_webhook_deliveries_ws_status_created
    ON customer_webhook_deliveries(workspace_id, status, created_at DESC);

CREATE INDEX idx_webhook_deliveries_ws_webhook_created
    ON customer_webhook_deliveries(workspace_id, webhook_id, created_at DESC);

CREATE INDEX idx_webhook_deliveries_status_next_attempt
    ON customer_webhook_deliveries(status, next_attempt_at)
    WHERE status IN ('pending', 'retry_scheduled');

CREATE TABLE customer_webhook_delivery_attempts (
    id TEXT PRIMARY KEY,
    delivery_id TEXT NOT NULL REFERENCES customer_webhook_deliveries(id) ON DELETE CASCADE,
    attempt_number BIGINT NOT NULL,
    status TEXT NOT NULL,
    status_code INTEGER NULL,
    error TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    request_headers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_headers_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempted_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX uq_delivery_attempts_delivery_number
    ON customer_webhook_delivery_attempts(delivery_id, attempt_number);

-- +goose Down
DROP TABLE IF EXISTS customer_webhook_delivery_attempts;
DROP TABLE IF EXISTS customer_webhook_deliveries;
DROP TABLE IF EXISTS customer_webhooks;

UPDATE roles r
SET permissions_mask = r.permissions_mask & ~((1<<19) | (1<<20) | (1<<21)),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

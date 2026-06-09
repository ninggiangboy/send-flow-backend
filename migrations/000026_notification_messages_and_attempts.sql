-- +goose Up

-- Permission bit: notification.read = 1<<26
INSERT INTO permission_registry (bit, name, description, created_at)
VALUES (1<<26, 'notification.read', 'View notification status and manage alerts', NOW())
ON CONFLICT (bit) DO NOTHING;

UPDATE roles r
SET permissions_mask = r.permissions_mask | (1<<26),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

CREATE TABLE notification_messages (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    recipient_email TEXT NOT NULL,
    recipient_user_id TEXT NULL REFERENCES users(id) ON DELETE SET NULL,
    subject TEXT NOT NULL,
    body_text TEXT NOT NULL DEFAULT '',
    body_html TEXT NULL,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_notification_type CHECK (type IN ('welcome_email', 'invitation_email', 'system_alert')),
    CONSTRAINT chk_notification_status CHECK (status IN ('pending', 'queued', 'sending', 'sent', 'failed', 'retrying')),
    CONSTRAINT chk_notification_max_attempts CHECK (max_attempts >= 1 AND max_attempts <= 10),
    CONSTRAINT chk_notification_attempt_count CHECK (attempt_count >= 0)
);

CREATE TABLE notification_attempts (
    id TEXT PRIMARY KEY,
    notification_message_id TEXT NOT NULL REFERENCES notification_messages(id) ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'sending',
    provider TEXT NOT NULL,
    provider_message_id TEXT NULL,
    error_message TEXT NULL,
    attempted_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT chk_notification_attempt_status CHECK (status IN ('sending', 'sent', 'failed')),
    CONSTRAINT chk_notification_attempt_number CHECK (attempt_number >= 1 AND attempt_number <= 10),
    CONSTRAINT uq_notification_message_attempt UNIQUE (notification_message_id, attempt_number)
);

CREATE INDEX idx_notification_messages_ws_status_created
    ON notification_messages(workspace_id, status, created_at DESC);

CREATE INDEX idx_notification_messages_recipient_status
    ON notification_messages(recipient_user_id, status);

CREATE INDEX idx_notification_messages_pending_retry
    ON notification_messages(status, created_at ASC)
    WHERE status = 'retrying' AND attempt_count < max_attempts;

-- +goose Down
DROP TABLE IF EXISTS notification_attempts;
DROP TABLE IF EXISTS notification_messages;

UPDATE roles r
SET permissions_mask = r.permissions_mask & ~(1<<26),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

DELETE FROM permission_registry WHERE bit = (1<<26);

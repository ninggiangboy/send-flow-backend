-- +goose Up
-- +goose StatementBegin

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS reply_to TEXT,
    ADD COLUMN IF NOT EXISTS headers JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE messages
    DROP COLUMN IF EXISTS headers,
    DROP COLUMN IF EXISTS reply_to;

-- +goose StatementEnd

-- +goose Up
ALTER TABLE dead_letter_records ADD COLUMN IF NOT EXISTS source_event_type TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE dead_letter_records DROP COLUMN IF EXISTS source_event_type;

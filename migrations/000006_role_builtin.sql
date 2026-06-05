-- +goose Up
ALTER TABLE roles ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'custom';
ALTER TABLE roles ADD COLUMN IF NOT EXISTS builtin BOOLEAN NOT NULL DEFAULT false;
UPDATE roles SET builtin = true, type = CASE
	WHEN LOWER(name) = 'owner' THEN 'owner'
	WHEN LOWER(name) = 'member' THEN 'member'
	ELSE 'custom'
END;

-- +goose Down
ALTER TABLE roles DROP COLUMN IF EXISTS type;
ALTER TABLE roles DROP COLUMN IF EXISTS builtin;
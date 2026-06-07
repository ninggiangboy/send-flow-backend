-- +goose Up
-- Add new permission bits (1 << 5 through 1 << 13 = 16352) to all owner roles
UPDATE roles SET permissions_mask = permissions_mask | 16352 WHERE type = 'owner';

-- +goose Down
UPDATE roles SET permissions_mask = permissions_mask & ~16352 WHERE type = 'owner';

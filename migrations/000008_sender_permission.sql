-- +goose Up
-- Add sender.manage permission bit (1 << 4 = 16) to all owner roles
UPDATE roles SET permissions_mask = permissions_mask | 16 WHERE type = 'owner';

-- +goose Down
UPDATE roles SET permissions_mask = permissions_mask & ~16 WHERE type = 'owner';

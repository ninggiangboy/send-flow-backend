-- +goose Up
UPDATE roles r
SET permissions_mask = r.permissions_mask | (1<<18),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

-- +goose Down
UPDATE roles r
SET permissions_mask = r.permissions_mask & ~(1<<18),
    updated_at = NOW()
FROM workspaces w
WHERE r.workspace_id = w.id
  AND r.type = 'owner';

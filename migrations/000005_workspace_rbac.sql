-- +goose Up
CREATE TABLE IF NOT EXISTS roles (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    permissions_mask BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, name)
);
CREATE INDEX IF NOT EXISTS idx_roles_workspace ON roles(workspace_id);

CREATE TABLE IF NOT EXISTS membership_roles (
    membership_id TEXT NOT NULL REFERENCES workspace_memberships(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (membership_id, role_id)
);
CREATE INDEX IF NOT EXISTS idx_membership_roles_role ON membership_roles(role_id);

CREATE TABLE IF NOT EXISTS invitation_roles (
    invitation_id TEXT NOT NULL REFERENCES workspace_invitations(id) ON DELETE CASCADE,
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (invitation_id, role_id)
);
CREATE INDEX IF NOT EXISTS idx_invitation_roles_role ON invitation_roles(role_id);

INSERT INTO roles (id, workspace_id, name, permissions_mask, status, version, created_at, updated_at)
SELECT
    'role_owner_' || w.id,
    w.id,
    'owner',
    15,
    'active',
    1,
    COALESCE(w.created_at, NOW()),
    COALESCE(w.updated_at, NOW())
FROM workspaces w
ON CONFLICT (id) DO NOTHING;

INSERT INTO roles (id, workspace_id, name, permissions_mask, status, version, created_at, updated_at)
SELECT
    'role_member_' || w.id,
    w.id,
    'member',
    1,
    'active',
    1,
    COALESCE(w.created_at, NOW()),
    COALESCE(w.updated_at, NOW())
FROM workspaces w
ON CONFLICT (id) DO NOTHING;

INSERT INTO membership_roles (membership_id, role_id, created_at)
SELECT
    m.id,
    CASE
        WHEN LOWER(m.role) = 'owner' THEN 'role_owner_' || m.workspace_id
        ELSE 'role_member_' || m.workspace_id
    END,
    COALESCE(m.created_at, NOW())
FROM workspace_memberships m
ON CONFLICT (membership_id, role_id) DO NOTHING;

INSERT INTO invitation_roles (invitation_id, role_id, created_at)
SELECT
    i.id,
    CASE
        WHEN LOWER(i.role) = 'owner' THEN 'role_owner_' || i.workspace_id
        ELSE 'role_member_' || i.workspace_id
    END,
    COALESCE(i.created_at, NOW())
FROM workspace_invitations i
ON CONFLICT (invitation_id, role_id) DO NOTHING;

UPDATE workspace_memberships
SET role = CASE
    WHEN LOWER(role) = 'owner' THEN 'owner'
    ELSE 'member'
END
WHERE LOWER(role) IN ('owner', 'admin', 'member');

UPDATE workspace_invitations
SET role = CASE
    WHEN LOWER(role) = 'owner' THEN 'owner'
    ELSE 'member'
END
WHERE LOWER(role) IN ('owner', 'admin', 'member');

-- +goose Down
DROP TABLE IF EXISTS invitation_roles;
DROP TABLE IF EXISTS membership_roles;
DROP TABLE IF EXISTS roles;

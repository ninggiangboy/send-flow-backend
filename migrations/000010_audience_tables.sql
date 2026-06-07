-- +goose Up
CREATE TABLE contacts (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    email_normalized TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    status TEXT NOT NULL,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    attributes JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_contacts_workspace_email ON contacts(workspace_id, email_normalized);
CREATE INDEX idx_contacts_workspace_status_created ON contacts(workspace_id, status, created_at DESC);

CREATE TABLE audience_lists (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_lists_workspace_name ON audience_lists(workspace_id, name) WHERE archived_at IS NULL;

CREATE TABLE audience_list_memberships (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    list_id TEXT NOT NULL REFERENCES audience_lists(id) ON DELETE CASCADE,
    contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (workspace_id, list_id, contact_id)
);

CREATE INDEX idx_list_memberships_list ON audience_list_memberships(workspace_id, list_id);
CREATE INDEX idx_list_memberships_contact ON audience_list_memberships(workspace_id, contact_id);

CREATE TABLE segments (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    definition_json JSONB NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX idx_segments_workspace_name ON segments(workspace_id, name);
CREATE INDEX idx_segments_workspace_status ON segments(workspace_id, status);

CREATE TABLE audience_import_jobs (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_uri TEXT NOT NULL,
    dedupe_mode TEXT NOT NULL,
    status TEXT NOT NULL,
    processed_count BIGINT NOT NULL DEFAULT 0,
    created_count BIGINT NOT NULL DEFAULT 0,
    updated_count BIGINT NOT NULL DEFAULT 0,
    failed_count BIGINT NOT NULL DEFAULT 0,
    error_summary TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_import_jobs_workspace_created ON audience_import_jobs(workspace_id, created_at DESC);

CREATE TABLE audience_export_jobs (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    filters_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    selected_fields JSONB NOT NULL DEFAULT '[]'::jsonb,
    format TEXT NOT NULL,
    status TEXT NOT NULL,
    artifact_uri TEXT,
    error_summary TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_export_jobs_workspace_created ON audience_export_jobs(workspace_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audience_export_jobs;
DROP TABLE IF EXISTS audience_import_jobs;
DROP TABLE IF EXISTS segments;
DROP TABLE IF EXISTS audience_list_memberships;
DROP TABLE IF EXISTS audience_lists;
DROP TABLE IF EXISTS contacts;

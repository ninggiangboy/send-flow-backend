-- +goose Up
CREATE TABLE template_versions (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    template_id TEXT NOT NULL,
    version_number INT NOT NULL,
    subject TEXT NOT NULL,
    source_html TEXT NOT NULL,
    source_text TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX idx_template_versions_template_version ON template_versions(template_id, version_number);
CREATE INDEX idx_template_versions_template_id_version_desc ON template_versions(template_id, version_number DESC);

CREATE TABLE templates (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    subject TEXT NOT NULL,
    source_html TEXT NOT NULL,
    source_text TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    current_version_id TEXT REFERENCES template_versions(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    archived_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_templates_workspace_name ON templates(workspace_id, name);
CREATE INDEX idx_templates_workspace_status_created ON templates(workspace_id, status, created_at DESC);
CREATE INDEX idx_templates_workspace_name_idx ON templates(workspace_id, name);

CREATE TABLE template_render_snapshots (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    template_id TEXT,
    template_version_id TEXT REFERENCES template_versions(id) ON DELETE SET NULL,
    render_input_hash TEXT NOT NULL,
    subject TEXT,
    rendered_html TEXT,
    rendered_text TEXT,
    warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_render_snapshots_version_hash ON template_render_snapshots(workspace_id, template_version_id, render_input_hash);

-- +goose Down
DROP TABLE IF EXISTS template_render_snapshots;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS template_versions;

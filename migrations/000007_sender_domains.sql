-- +goose Up
CREATE TABLE sender_domains (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain TEXT NOT NULL,
    provider TEXT NOT NULL,
    status TEXT NOT NULL,
    verified_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(workspace_id, domain)
);

CREATE TABLE sender_domain_dns_records (
    id TEXT PRIMARY KEY,
    sender_domain_id TEXT NOT NULL REFERENCES sender_domains(id) ON DELETE CASCADE,
    record_type TEXT NOT NULL,
    host TEXT NOT NULL,
    expected_value TEXT NOT NULL,
    current_value TEXT,
    status TEXT NOT NULL,
    last_checked_at TIMESTAMPTZ,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sender_domains_workspace_id_status ON sender_domains(workspace_id, status);
CREATE INDEX idx_sender_domains_workspace_id_domain ON sender_domains(workspace_id, domain);
CREATE INDEX idx_sender_domain_dns_records_sender_domain_id ON sender_domain_dns_records(sender_domain_id);

-- +goose Down
DROP TABLE IF EXISTS sender_domain_dns_records;
DROP TABLE IF EXISTS sender_domains;

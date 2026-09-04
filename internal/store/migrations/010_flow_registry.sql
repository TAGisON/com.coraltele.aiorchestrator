-- Flow graph publish registry (P2.7 / M-A). Replaces desk_* conceptually;
-- desk DROP remains M-F (P2.13). One concern: CREATE flow family only.

CREATE TABLE IF NOT EXISTS flow (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    direction TEXT NOT NULL DEFAULT 'inbound',
    status TEXT NOT NULL DEFAULT 'draft',
    current_version INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS flow_tenant_idx ON flow (tenant_id);

CREATE TABLE IF NOT EXISTS flow_draft (
    flow_id TEXT PRIMARY KEY REFERENCES flow(id) ON DELETE CASCADE,
    doc JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS flow_version (
    flow_id TEXT NOT NULL REFERENCES flow(id) ON DELETE CASCADE,
    version INT NOT NULL,
    doc JSONB NOT NULL,
    content_hash TEXT NOT NULL,
    published_by TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (flow_id, version)
);

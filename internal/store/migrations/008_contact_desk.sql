-- Contact Desk vertical: desk registry, drafts, immutable versions, contact
-- attributes, skill ledger and compliance seams (CONTACT_DESK_POC_SOLUTION.md §4.4).

CREATE TABLE IF NOT EXISTS desk (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    name TEXT NOT NULL,
    direction TEXT NOT NULL DEFAULT 'inbound',
    purpose TEXT NOT NULL DEFAULT 'support',
    status TEXT NOT NULL DEFAULT 'draft',
    current_version INT NOT NULL DEFAULT 0,
    profile_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS desk_tenant_idx ON desk (tenant_id);

CREATE TABLE IF NOT EXISTS desk_draft (
    desk_id TEXT PRIMARY KEY REFERENCES desk(id) ON DELETE CASCADE,
    doc JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS desk_version (
    desk_id TEXT NOT NULL REFERENCES desk(id) ON DELETE CASCADE,
    version INT NOT NULL,
    doc JSONB NOT NULL,
    profile_id TEXT NOT NULL DEFAULT '',
    profile_version INT NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL DEFAULT '',
    published_by TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (desk_id, version)
);

CREATE TABLE IF NOT EXISTS session_attributes (
    session_id TEXT NOT NULL REFERENCES session(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    class TEXT NOT NULL DEFAULT 'none',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, key)
);

CREATE TABLE IF NOT EXISTS skill_invocation (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL DEFAULT '',
    skill TEXT NOT NULL,
    idempotency_key TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    args JSONB NOT NULL DEFAULT '{}'::jsonb,
    output JSONB NOT NULL DEFAULT '{}'::jsonb,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS skill_invocation_session_idx ON skill_invocation (session_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS skill_invocation_idem_idx
    ON skill_invocation (idempotency_key) WHERE idempotency_key <> '';

CREATE TABLE IF NOT EXISTS pii_access_audit (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    keys TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS pii_access_session_idx ON pii_access_audit (session_id, id);

CREATE TABLE IF NOT EXISTS erasure_request (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    subject_ref TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'all',
    status TEXT NOT NULL DEFAULT 'queued',
    requested_by TEXT NOT NULL DEFAULT '',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS consent_record (
    tenant_id TEXT NOT NULL,
    phone TEXT NOT NULL,
    state TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, phone)
);

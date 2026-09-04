-- Binding registry (P2.10 / M-B). Knowledge/CRM capabilities for Inform.
-- Do not DROP kb_* here (M-G / P2.13). One concern: CREATE binding only.

CREATE TABLE IF NOT EXISTS binding (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    config JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS binding_tenant_kind_idx
    ON binding (tenant_id, kind);

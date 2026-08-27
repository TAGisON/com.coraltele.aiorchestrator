-- Phase E: analytics_event + postcall error column (ANALYTICS_AND_POSTCALL.md)

CREATE TABLE IF NOT EXISTS analytics_event (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    profile_id TEXT,
    session_id TEXT,
    metric TEXT NOT NULL,
    value DOUBLE PRECISION NOT NULL DEFAULT 1,
    dimensions JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS analytics_event_session_idx ON analytics_event (session_id);
CREATE INDEX IF NOT EXISTS analytics_event_tenant_idx ON analytics_event (tenant_id);

ALTER TABLE postcall_job ADD COLUMN IF NOT EXISTS error_message TEXT;

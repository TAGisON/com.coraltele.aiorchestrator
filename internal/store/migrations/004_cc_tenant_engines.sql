-- CC Phase 1: tenant system engines + session gateway_binding pin

CREATE TABLE IF NOT EXISTS tenant_engines (
    tenant_id TEXT PRIMARY KEY,
    listen_id TEXT NOT NULL,
    think_id TEXT NOT NULL,
    speak_id TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE session
    ADD COLUMN IF NOT EXISTS gateway_binding JSONB;

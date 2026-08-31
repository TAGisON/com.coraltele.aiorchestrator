-- Boot/runtime settings: vendor credentials + optional string settings (not in properties).

CREATE TABLE IF NOT EXISTS gateway_credentials (
    tenant_id TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    extra JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, gateway_id)
);

CREATE TABLE IF NOT EXISTS system_settings (
    tenant_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, key)
);

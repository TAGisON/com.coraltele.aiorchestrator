-- Caller preferences: remember per-ANI language across calls (SOLUTION §16.3 /
-- customer_memory). Keyed by tenant + normalised ANI — not a parallel user master.

CREATE TABLE IF NOT EXISTS caller_preference (
    tenant_id TEXT NOT NULL,
    ani TEXT NOT NULL,
    preferred_language TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, ani)
);

CREATE INDEX IF NOT EXISTS caller_preference_lang_idx
    ON caller_preference (tenant_id, preferred_language)
    WHERE preferred_language <> '';

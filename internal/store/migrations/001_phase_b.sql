-- Phase B durable platform tables (PLATFORM_FIRST / SOLUTION §17)

CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS profile (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    display_name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS profile_version (
    profile_id TEXT NOT NULL REFERENCES profile (id),
    version INT NOT NULL,
    document JSONB NOT NULL,
    published_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, version)
);

CREATE TABLE IF NOT EXISTS session (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    profile_id TEXT NOT NULL,
    profile_version INT NOT NULL,
    clock TEXT NOT NULL,
    state TEXT NOT NULL,
    owner_instance TEXT,
    canonical_sample_rate_hz INT NOT NULL,
    coral_user_id TEXT,
    caller JSONB,
    recording_ref TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (profile_id, profile_version) REFERENCES profile_version (profile_id, version)
);

CREATE INDEX IF NOT EXISTS session_tenant_idx ON session (tenant_id);
CREATE INDEX IF NOT EXISTS session_state_idx ON session (state);

CREATE TABLE IF NOT EXISTS audit_event (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT,
    tenant_id TEXT,
    event_type TEXT NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_event_session_idx ON audit_event (session_id);

CREATE TABLE IF NOT EXISTS playback_job (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    file_uri TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    profile_version INT NOT NULL,
    state TEXT NOT NULL,
    lease_owner TEXT,
    leased_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (profile_id, profile_version) REFERENCES profile_version (profile_id, version)
);

CREATE TABLE IF NOT EXISTS postcall_job (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES session (id),
    profile_id TEXT NOT NULL,
    profile_version INT NOT NULL,
    state TEXT NOT NULL,
    lease_owner TEXT,
    leased_until TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (profile_id, profile_version) REFERENCES profile_version (profile_id, version)
);

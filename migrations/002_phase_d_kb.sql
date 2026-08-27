-- Phase D KB document + chunk tables (INTEGRATION.md ingest)

CREATE TABLE IF NOT EXISTS kb_document (
    id TEXT PRIMARY KEY,
    tenant_id TEXT,
    collection TEXT NOT NULL,
    uri TEXT,
    status TEXT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS kb_document_tenant_collection_idx ON kb_document (tenant_id, collection);

CREATE TABLE IF NOT EXISTS kb_chunk (
    id BIGSERIAL PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES kb_document (id) ON DELETE CASCADE,
    tenant_id TEXT,
    collection TEXT NOT NULL,
    ordinal INT NOT NULL,
    text TEXT NOT NULL,
    source_uri TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS kb_chunk_collection_idx ON kb_chunk (tenant_id, collection);

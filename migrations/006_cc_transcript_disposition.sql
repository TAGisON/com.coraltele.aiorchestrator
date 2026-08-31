-- CC Phase 5: durable transcript turns + session disposition suggestion
-- Mirror of internal/store/migrations/006_cc_transcript_disposition.sql

CREATE TABLE IF NOT EXISTS transcript_turn (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES session(id),
    seq INT NOT NULL,
    role TEXT NOT NULL,
    text TEXT NOT NULL,
    turn_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, seq),
    UNIQUE (session_id, turn_id, role)
);

CREATE INDEX IF NOT EXISTS transcript_turn_session_idx ON transcript_turn (session_id, seq);

CREATE TABLE IF NOT EXISTS session_disposition (
    session_id TEXT PRIMARY KEY REFERENCES session(id),
    suggestion TEXT NOT NULL,
    template_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'postcall_worker',
    final TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

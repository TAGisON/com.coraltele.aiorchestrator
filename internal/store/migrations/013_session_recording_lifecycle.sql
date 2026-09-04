-- Session recording lifecycle (P2.5 / M-Cr). Pointer remains recording_ref (001).
-- No PCM in database columns. Transcript expand is M-D — not this file.

ALTER TABLE session
    ADD COLUMN IF NOT EXISTS recording_started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS recording_stopped_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS recording_stop_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS recording_bytes BIGINT;

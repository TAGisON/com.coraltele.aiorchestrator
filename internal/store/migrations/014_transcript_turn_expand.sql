-- Transcript event expand (P2.3 / M-D). Keep table name transcript_turn.
-- One concern: add columns, backfill, drop pair UNIQUE, nullable turn_id.
-- Keep UNIQUE (session_id, seq). No PCM in payload.

ALTER TABLE transcript_turn
    ADD COLUMN IF NOT EXISTS event_kind TEXT NOT NULL DEFAULT 'utterance',
    ADD COLUMN IF NOT EXISTS actionable BOOLEAN,
    ADD COLUMN IF NOT EXISTS actionable_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS node_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS edge_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS language TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payload JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE transcript_turn
SET event_kind = 'user_final',
    actionable = true,
    actionable_reason = 'legacy_import'
WHERE role = 'user' AND event_kind = 'utterance';

UPDATE transcript_turn
SET event_kind = 'bot_utterance'
WHERE role = 'assistant' AND event_kind = 'utterance';

UPDATE transcript_turn
SET event_kind = 'note'
WHERE role = 'system' AND event_kind = 'utterance';

ALTER TABLE transcript_turn
    DROP CONSTRAINT IF EXISTS transcript_turn_session_id_turn_id_role_key;

ALTER TABLE transcript_turn
    ALTER COLUMN turn_id DROP NOT NULL;

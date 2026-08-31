-- CC Phase 2: session language lock fields

ALTER TABLE session
    ADD COLUMN IF NOT EXISTS detected_language TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS active_language TEXT NOT NULL DEFAULT '';

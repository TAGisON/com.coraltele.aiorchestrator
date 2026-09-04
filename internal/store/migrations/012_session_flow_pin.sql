-- Session flow pins (P2.7 / M-C). Nullable until runtime requires published flow.
-- Recording lifecycle columns are M-Cr (P2.5) — not this file.

ALTER TABLE session
    ADD COLUMN IF NOT EXISTS flow_id TEXT,
    ADD COLUMN IF NOT EXISTS flow_version INT;

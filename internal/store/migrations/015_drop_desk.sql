-- DROP desk family (P2.13 / M-F). Replaced by flow_* (010).
-- Order: desk_version, then desk_draft, then desk. Do not DROP kb_* here (M-G).

DROP TABLE IF EXISTS desk_version;
DROP TABLE IF EXISTS desk_draft;
DROP TABLE IF EXISTS desk;

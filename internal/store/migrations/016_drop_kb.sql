-- DROP kb family (P2.13 / M-G). Replaced by binding (011); Go readers removed in M-E.
-- Order: kb_chunk (FK child), then kb_document. Do not DROP desk_* (M-F) or compliance (M-H).

DROP TABLE IF EXISTS kb_chunk;
DROP TABLE IF EXISTS kb_document;

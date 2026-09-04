-- DROP skill/compliance family (P2.13 / M-H). Store APIs removed in M-E; unused-confirm 2026-09-05.
-- Order matches P2.13 list. Do not DROP desk_* (M-F) or kb_* (M-G). Do not edit 008 history.

DROP TABLE IF EXISTS skill_invocation;
DROP TABLE IF EXISTS pii_access_audit;
DROP TABLE IF EXISTS erasure_request;
DROP TABLE IF EXISTS consent_record;

-- 003_governance.down.sql
-- Rollback governance schema (MySQL version)

USE obs_platform;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS decision;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS prompt_hash;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS session_id;

DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS budgets;

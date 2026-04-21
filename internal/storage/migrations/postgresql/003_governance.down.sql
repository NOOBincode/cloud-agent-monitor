-- 003_governance.down.sql
-- Rollback governance schema

DROP TRIGGER IF EXISTS update_policies_updated_at ON obs_platform.policies;
DROP TRIGGER IF EXISTS update_budgets_updated_at ON obs_platform.budgets;
ALTER TABLE obs_platform.audit_logs DROP COLUMN IF EXISTS decision;
ALTER TABLE obs_platform.audit_logs DROP COLUMN IF EXISTS prompt_hash;
ALTER TABLE obs_platform.audit_logs DROP COLUMN IF EXISTS session_id;
DROP TABLE IF EXISTS obs_platform.policies;
DROP TABLE IF EXISTS obs_platform.budgets;
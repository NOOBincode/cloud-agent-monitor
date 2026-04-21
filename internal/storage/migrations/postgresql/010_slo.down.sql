-- 010_slo.down.sql
-- Rollback SLO schema

DROP TRIGGER IF EXISTS update_slis_updated_at ON obs_platform.slis;
DROP TRIGGER IF EXISTS update_slos_updated_at ON obs_platform.slos;
DROP TABLE IF EXISTS obs_platform.burn_rate_alerts;
DROP TABLE IF EXISTS obs_platform.error_budget_history;
DROP TABLE IF EXISTS obs_platform.slis;
DROP TABLE IF EXISTS obs_platform.slos;
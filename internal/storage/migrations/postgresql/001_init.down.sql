-- 001_init.down.sql
-- Rollback initial schema

DROP TRIGGER IF EXISTS update_query_templates_updated_at ON obs_platform.query_templates;
DROP TRIGGER IF EXISTS update_services_updated_at ON obs_platform.services;
DROP FUNCTION IF EXISTS obs_platform.update_updated_at_column();
DROP TABLE IF EXISTS obs_platform.audit_logs;
DROP TABLE IF EXISTS obs_platform.query_templates;
DROP TABLE IF EXISTS obs_platform.services;
DROP SCHEMA IF EXISTS obs_platform;

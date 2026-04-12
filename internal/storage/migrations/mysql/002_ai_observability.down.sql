-- 002_ai_observability.down.sql
-- Rollback AI observability schema (MySQL version)

USE obs_platform;
DROP TABLE IF EXISTS tool_calls;
DROP TABLE IF EXISTS ai_sessions;
DROP TABLE IF EXISTS prompt_templates;
DROP TABLE IF EXISTS ai_models;

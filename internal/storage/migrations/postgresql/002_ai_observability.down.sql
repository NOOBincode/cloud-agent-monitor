-- 002_ai_observability.down.sql
-- Rollback AI observability schema

DROP TRIGGER IF EXISTS update_prompt_templates_updated_at ON obs_platform.prompt_templates;
DROP TRIGGER IF EXISTS update_ai_models_updated_at ON obs_platform.ai_models;
DROP TABLE IF EXISTS obs_platform.prompt_templates;
DROP TABLE IF EXISTS obs_platform.tool_calls;
DROP TABLE IF EXISTS obs_platform.ai_sessions;
DROP TABLE IF EXISTS obs_platform.ai_models;

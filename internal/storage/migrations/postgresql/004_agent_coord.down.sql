-- 004_agent_coord.down.sql
-- Rollback agent coordination schema

DROP TRIGGER IF EXISTS update_agents_updated_at ON obs_platform.agents;
DROP TABLE IF EXISTS obs_platform.agent_events;
DROP TABLE IF EXISTS obs_platform.agent_commands;
DROP TABLE IF EXISTS obs_platform.agents;
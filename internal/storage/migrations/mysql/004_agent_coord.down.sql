-- 004_agent_coord.down.sql
-- Rollback agent coordination schema (MySQL version)

USE obs_platform;
DROP TABLE IF EXISTS agent_events;
DROP TABLE IF EXISTS agent_commands;
DROP TABLE IF EXISTS agents;

-- 008_alerting.down.sql
-- Rollback alert routing module tables

USE obs_platform;

-- Drop tables in reverse order of creation
DROP TABLE IF EXISTS alert_records;
DROP TABLE IF EXISTS alert_notifications;
DROP TABLE IF EXISTS alert_noise_records;
DROP TABLE IF EXISTS alert_operations;

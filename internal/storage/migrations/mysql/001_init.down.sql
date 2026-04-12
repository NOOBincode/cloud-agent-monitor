-- 001_init.down.sql
-- Rollback initial schema (MySQL version)

USE obs_platform;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS query_templates;
DROP TABLE IF EXISTS services;

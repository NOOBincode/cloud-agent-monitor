-- 006_user.down.sql
-- Rollback user schema

DROP TRIGGER IF EXISTS update_roles_updated_at ON obs_platform.roles;
DROP TRIGGER IF EXISTS update_users_updated_at ON obs_platform.users;
DELETE FROM obs_platform.user_roles;
DELETE FROM obs_platform.roles;
DELETE FROM obs_platform.users;
DROP TABLE IF EXISTS obs_platform.login_logs;
DROP TABLE IF EXISTS obs_platform.user_roles;
DROP TABLE IF EXISTS obs_platform.roles;
DROP TABLE IF EXISTS obs_platform.users;
-- 006_user.down.sql
-- Revert user authentication and authorization schema
USE obs_platform;
DROP TABLE IF EXISTS `login_logs`;
DROP TABLE IF EXISTS `user_roles`;
DROP TABLE IF EXISTS `roles`;
DROP TABLE IF EXISTS `users`;

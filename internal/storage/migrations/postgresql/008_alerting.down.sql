-- 008_alerting.down.sql
-- Rollback alerting schema

DROP TRIGGER IF EXISTS update_alert_noise_records_updated_at ON obs_platform.alert_noise_records;
DROP TRIGGER IF EXISTS update_alert_operations_updated_at ON obs_platform.alert_operations;
DROP TABLE IF EXISTS obs_platform.alert_records;
DROP TABLE IF EXISTS obs_platform.alert_notifications;
DROP TABLE IF EXISTS obs_platform.alert_noise_records;
DROP TABLE IF EXISTS obs_platform.alert_operations;
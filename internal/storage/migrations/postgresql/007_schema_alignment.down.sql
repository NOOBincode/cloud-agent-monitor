-- 007_schema_alignment.down.sql
-- Rollback schema alignment

DROP TRIGGER IF EXISTS update_api_keys_updated_at ON obs_platform.api_keys;
DROP TABLE IF EXISTS obs_platform.cost_records;
DROP TABLE IF EXISTS obs_platform.api_keys;
DROP TABLE IF EXISTS obs_platform.service_labels;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS health_check_details;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS last_health_check_at;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS health_status;
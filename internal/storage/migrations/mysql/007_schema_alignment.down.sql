-- 007_schema_alignment.down.sql
-- Revert schema alignment changes

USE obs_platform;

-- 1. Remove health check fields from services
ALTER TABLE services 
    DROP COLUMN health_status,
    DROP COLUMN last_health_check_at,
    DROP COLUMN health_check_details,
    DROP COLUMN deleted_at;

-- 2. Drop service_labels table
DROP TABLE service_labels;

-- 3. Drop api_keys table
DROP TABLE api_keys;

-- 4. Drop cost_records table
DROP TABLE cost_records;

-- 5. Revert prompt_templates changes
ALTER TABLE prompt_templates 
    DROP COLUMN variables,
    DROP COLUMN version,
    DROP COLUMN labels,
    DROP COLUMN is_active;

-- 012_genai_schema_fix.down.sql
-- Rollback GenAI schema fixes

-- Re-add redundant columns to ai_sessions
ALTER TABLE obs_platform.ai_sessions
    ADD COLUMN IF NOT EXISTS operation VARCHAR(50),
    ADD COLUMN IF NOT EXISTS completion_tokens INTEGER;

-- Re-add redundant columns to tool_calls
ALTER TABLE obs_platform.tool_calls
    ADD COLUMN IF NOT EXISTS tool_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS tool_type VARCHAR(50);

-- Remove added OTel columns
ALTER TABLE obs_platform.ai_sessions
    DROP COLUMN IF EXISTS gen_ai_server_address,
    DROP COLUMN IF EXISTS gen_ai_server_port,
    DROP COLUMN IF EXISTS gen_ai_request_encoding_formats,
    DROP COLUMN IF EXISTS gen_ai_response_error_code;

ALTER TABLE obs_platform.inference_requests
    DROP COLUMN IF EXISTS gen_ai_system;

ALTER TABLE obs_platform.gpu_metrics
    DROP COLUMN IF EXISTS memory_total_mb;

-- Drop agent tables
DROP TABLE IF EXISTS obs_platform.agent_tool_permissions;
DROP TABLE IF EXISTS obs_platform.agent_tool_pools;

-- 012_genai_schema_fix.up.sql
-- Fix GenAI schema audit issues: remove redundant columns, add missing OTel fields

-- ============================================================
-- P1: ai_sessions - deprecate redundant non-gen_ai_ prefixed columns
-- ============================================================

ALTER TABLE obs_platform.ai_sessions
    DROP COLUMN IF EXISTS operation,
    DROP COLUMN IF EXISTS prompt_tokens,
    DROP COLUMN IF EXISTS completion_tokens,
    DROP COLUMN IF EXISTS total_tokens;

-- ============================================================
-- P2: tool_calls - deprecate redundant tool_name / tool_type
-- ============================================================

ALTER TABLE obs_platform.tool_calls
    DROP COLUMN IF EXISTS tool_name,
    DROP COLUMN IF EXISTS tool_type;

-- ============================================================
-- P3: ai_sessions - add missing OTel GenAI v1.37+ fields
-- ============================================================

ALTER TABLE obs_platform.ai_sessions
    ADD COLUMN IF NOT EXISTS gen_ai_server_address VARCHAR(255),
    ADD COLUMN IF NOT EXISTS gen_ai_server_port INTEGER,
    ADD COLUMN IF NOT EXISTS gen_ai_request_encoding_formats JSONB,
    ADD COLUMN IF NOT EXISTS gen_ai_response_error_code VARCHAR(50);

COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_server_address IS 'OTel: gen_ai.server.address (inference endpoint host)';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_server_port IS 'OTel: gen_ai.server.port';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_request_encoding_formats IS 'OTel: gen_ai.request.encoding_formats (for embeddings)';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_response_error_code IS 'OTel: gen_ai.response.error_code (e.g. rate_limited, context_length_exceeded)';

-- ============================================================
-- P3: inference_requests - add gen_ai.system for direct query
-- ============================================================

ALTER TABLE obs_platform.inference_requests
    ADD COLUMN IF NOT EXISTS gen_ai_system VARCHAR(50);

COMMENT ON COLUMN obs_platform.inference_requests.gen_ai_system IS 'OTel: gen_ai.system (vllm, triton, tgi, openai-compatible) — denormalized from inference_services.engine';

-- ============================================================
-- P3: gpu_metrics - add memory_total_mb for self-contained snapshots
-- ============================================================

ALTER TABLE obs_platform.gpu_metrics
    ADD COLUMN IF NOT EXISTS memory_total_mb INTEGER;

COMMENT ON COLUMN obs_platform.gpu_metrics.memory_total_mb IS 'DCGM: DCGM_FI_DEV_FB_TOTAL — total GPU memory in MB (snapshot-level, decoupled from gpu_nodes)';

-- ============================================================
-- Agent tool pool & permission tables
-- ============================================================

CREATE TABLE IF NOT EXISTS obs_platform.agent_tool_pools (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    categories JSONB DEFAULT '[]',
    tool_names JSONB DEFAULT '[]',
    keywords JSONB DEFAULT '[]',
    priority INTEGER DEFAULT 5,
    max_tools INTEGER DEFAULT 10,
    is_builtin BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS obs_platform.agent_tool_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    allowed BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_tool_role UNIQUE (tool_name, role)
);

CREATE INDEX idx_tool_permissions_tool ON obs_platform.agent_tool_permissions(tool_name);
CREATE INDEX idx_tool_permissions_role ON obs_platform.agent_tool_permissions(role);

-- Trigger
CREATE TRIGGER update_agent_tool_pools_updated_at BEFORE UPDATE ON obs_platform.agent_tool_pools
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_agent_tool_permissions_updated_at BEFORE UPDATE ON obs_platform.agent_tool_permissions
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

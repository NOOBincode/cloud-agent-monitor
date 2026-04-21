-- 007_schema_alignment.up.sql
-- Align database schema with Go models (PostgreSQL 17 version)

-- ============================================================
-- 1. Fix services table - add health check fields
-- ============================================================
ALTER TABLE obs_platform.services 
    ADD COLUMN health_status VARCHAR(20) DEFAULT 'unknown',
    ADD COLUMN last_health_check_at TIMESTAMP(3) DEFAULT NULL,
    ADD COLUMN health_check_details TEXT DEFAULT NULL,
    ADD COLUMN deleted_at TIMESTAMP(3) DEFAULT NULL;

CREATE INDEX idx_services_health_status ON obs_platform.services(health_status);
CREATE INDEX idx_services_deleted_at ON obs_platform.services(deleted_at);

COMMENT ON COLUMN obs_platform.services.health_status IS 'Health status: healthy, unhealthy, unknown';
COMMENT ON COLUMN obs_platform.services.last_health_check_at IS 'Last health check timestamp';
COMMENT ON COLUMN obs_platform.services.health_check_details IS 'Health check details';

-- ============================================================
-- 2. Create service_labels table for indexed label queries
-- ============================================================
CREATE TABLE obs_platform.service_labels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL,
    key VARCHAR(255) NOT NULL,
    value VARCHAR(255) NOT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT uk_service_label UNIQUE (service_id, key, value),
    CONSTRAINT fk_service_labels_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id) ON DELETE CASCADE
);

CREATE INDEX idx_service_labels_service ON obs_platform.service_labels(service_id);
CREATE INDEX idx_service_labels_key ON obs_platform.service_labels(key);
CREATE INDEX idx_service_labels_key_value ON obs_platform.service_labels(key, value);

-- ============================================================
-- 3. Create api_keys table (linked to users)
-- ============================================================
CREATE TABLE obs_platform.api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    prefix VARCHAR(20) NOT NULL,
    permissions JSONB DEFAULT NULL,
    expires_at TIMESTAMP(3) DEFAULT NULL,
    last_used_at TIMESTAMP(3) DEFAULT NULL,
    is_active BOOLEAN DEFAULT true,
    tenant_id UUID DEFAULT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at TIMESTAMP(3) DEFAULT NULL,
    CONSTRAINT uk_api_keys_key UNIQUE (key),
    CONSTRAINT fk_api_keys_user FOREIGN KEY (user_id) REFERENCES obs_platform.users(id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_user_id ON obs_platform.api_keys(user_id);
CREATE INDEX idx_api_keys_is_active ON obs_platform.api_keys(is_active);
CREATE INDEX idx_api_keys_tenant_id ON obs_platform.api_keys(tenant_id);
CREATE INDEX idx_api_keys_deleted_at ON obs_platform.api_keys(deleted_at);

-- ============================================================
-- 4. Create cost_records table
-- ============================================================
CREATE TABLE obs_platform.cost_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID DEFAULT NULL,
    service_id UUID DEFAULT NULL,
    user_id VARCHAR(255) DEFAULT NULL,
    team_id VARCHAR(255) DEFAULT NULL,
    model_id UUID DEFAULT NULL,
    cost_type VARCHAR(20) NOT NULL,
    cost_usd DECIMAL(20,6) NOT NULL,
    input_tokens INTEGER DEFAULT NULL,
    output_tokens INTEGER DEFAULT NULL,
    total_tokens INTEGER DEFAULT NULL,
    gpu_hours DECIMAL(10,6) DEFAULT NULL,
    cpu_hours DECIMAL(10,6) DEFAULT NULL,
    memory_gb_hours DECIMAL(10,6) DEFAULT NULL,
    resource_id VARCHAR(255) DEFAULT NULL,
    region VARCHAR(50) DEFAULT NULL,
    incurred_at TIMESTAMP(3) NOT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT fk_cost_records_session FOREIGN KEY (session_id) REFERENCES obs_platform.ai_sessions(id) ON DELETE SET NULL,
    CONSTRAINT fk_cost_records_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id) ON DELETE SET NULL,
    CONSTRAINT fk_cost_records_model FOREIGN KEY (model_id) REFERENCES obs_platform.ai_models(id) ON DELETE SET NULL
);

CREATE INDEX idx_cost_records_session ON obs_platform.cost_records(session_id);
CREATE INDEX idx_cost_records_service ON obs_platform.cost_records(service_id);
CREATE INDEX idx_cost_records_user ON obs_platform.cost_records(user_id);
CREATE INDEX idx_cost_records_incurred ON obs_platform.cost_records(incurred_at DESC);

COMMENT ON COLUMN obs_platform.cost_records.cost_type IS 'token, compute, storage, network';

-- ============================================================
-- 5. Ensure prompt_templates table has correct structure
-- ============================================================
ALTER TABLE obs_platform.prompt_templates 
    ADD COLUMN IF NOT EXISTS variables JSONB DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1,
    ADD COLUMN IF NOT EXISTS labels JSONB DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON obs_platform.api_keys
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
-- 001_init.up.sql
-- Initial schema for obs_platform (PostgreSQL 17 version)

-- Create schema if not exists
CREATE SCHEMA IF NOT EXISTS obs_platform;

-- Services table (service catalog)
CREATE TABLE obs_platform.services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    environment VARCHAR(50) NOT NULL DEFAULT 'local',
    labels JSONB DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Query templates table (pre-stored queries)
CREATE TABLE obs_platform.query_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    query TEXT NOT NULL,
    description TEXT,
    labels JSONB DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_service_name UNIQUE (service_id, name),
    CONSTRAINT fk_query_templates_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id) ON DELETE CASCADE
);

-- Audit log table
CREATE TABLE obs_platform.audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor VARCHAR(255) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    tenant_id UUID,
    metadata JSONB DEFAULT NULL,
    allowed BOOLEAN NOT NULL DEFAULT true,
    reason TEXT
);

-- Create indexes
CREATE INDEX idx_services_environment ON obs_platform.services(environment);
CREATE INDEX idx_query_templates_service ON obs_platform.query_templates(service_id);
CREATE INDEX idx_query_templates_type ON obs_platform.query_templates(type);
CREATE INDEX idx_audit_logs_timestamp ON obs_platform.audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_actor ON obs_platform.audit_logs(actor);
CREATE INDEX idx_audit_logs_action ON obs_platform.audit_logs(action);
CREATE INDEX idx_audit_logs_tenant ON obs_platform.audit_logs(tenant_id);

-- Create trigger function for auto-updating updated_at
CREATE OR REPLACE FUNCTION obs_platform.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to tables with updated_at
CREATE TRIGGER update_services_updated_at BEFORE UPDATE ON obs_platform.services
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_query_templates_updated_at BEFORE UPDATE ON obs_platform.query_templates
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

-- Comments
COMMENT ON TABLE obs_platform.services IS 'Service catalog';
COMMENT ON TABLE obs_platform.query_templates IS 'Pre-stored query templates';
COMMENT ON TABLE obs_platform.audit_logs IS 'Audit trail for all operations';

-- 003_governance.up.sql
-- Governance and audit schema extension (PostgreSQL 17 version)

-- Budgets table
CREATE TABLE obs_platform.budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID,
    name VARCHAR(255) NOT NULL,
    daily_limit DECIMAL(20,4),
    monthly_limit DECIMAL(20,4),
    alert_threshold DECIMAL(5,2) DEFAULT 0.80,
    currency VARCHAR(10) DEFAULT 'USD',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_budgets_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id)
);

-- Policies table
CREATE TABLE obs_platform.policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,
    description TEXT,
    rules JSONB DEFAULT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Extend audit_logs
ALTER TABLE obs_platform.audit_logs ADD COLUMN session_id UUID;
ALTER TABLE obs_platform.audit_logs ADD COLUMN prompt_hash VARCHAR(64);
ALTER TABLE obs_platform.audit_logs ADD COLUMN decision JSONB;

-- Indexes
CREATE INDEX idx_budgets_service ON obs_platform.budgets(service_id);
CREATE INDEX idx_policies_type ON obs_platform.policies(type);
CREATE INDEX idx_audit_logs_session ON obs_platform.audit_logs(session_id);

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_budgets_updated_at BEFORE UPDATE ON obs_platform.budgets
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_policies_updated_at BEFORE UPDATE ON obs_platform.policies
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
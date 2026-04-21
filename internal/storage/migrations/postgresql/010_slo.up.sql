-- 010_slo.up.sql
-- SLO/SLA Management Tables (PostgreSQL 17 version)

CREATE TABLE IF NOT EXISTS obs_platform.slos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    service_id UUID NOT NULL,
    target DECIMAL(5,2) NOT NULL DEFAULT 99.90,
    "window" VARCHAR(50) NOT NULL DEFAULT '30d',
    warning_burn DECIMAL(10,2) NOT NULL DEFAULT 2.00,
    critical_burn DECIMAL(10,2) NOT NULL DEFAULT 10.00,
    status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    current_value DECIMAL(10,4) DEFAULT 0,
    burn_rate DECIMAL(10,4) DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    CONSTRAINT fk_slo_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id) ON DELETE CASCADE
);

CREATE INDEX idx_slo_service ON obs_platform.slos(service_id);
CREATE INDEX idx_slo_status ON obs_platform.slos(status);
CREATE INDEX idx_slo_deleted ON obs_platform.slos(deleted_at);

CREATE TABLE IF NOT EXISTS obs_platform.slis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slo_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    query TEXT NOT NULL,
    threshold DECIMAL(20,4) DEFAULT 0,
    unit VARCHAR(50) DEFAULT '',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    CONSTRAINT fk_sli_slo FOREIGN KEY (slo_id) REFERENCES obs_platform.slos(id) ON DELETE CASCADE
);

CREATE INDEX idx_sli_slo ON obs_platform.slis(slo_id);
CREATE INDEX idx_sli_type ON obs_platform.slis(type);
CREATE INDEX idx_sli_deleted ON obs_platform.slis(deleted_at);

CREATE TABLE IF NOT EXISTS obs_platform.error_budget_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slo_id UUID NOT NULL,
    total DECIMAL(10,4) NOT NULL,
    remaining DECIMAL(10,4) NOT NULL,
    consumed DECIMAL(10,4) NOT NULL,
    percentage DECIMAL(10,4) NOT NULL,
    burn_rate DECIMAL(10,4) NOT NULL,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_ebh_slo FOREIGN KEY (slo_id) REFERENCES obs_platform.slos(id) ON DELETE CASCADE
);

CREATE INDEX idx_ebh_slo ON obs_platform.error_budget_history(slo_id);
CREATE INDEX idx_ebh_time ON obs_platform.error_budget_history(recorded_at);

CREATE TABLE IF NOT EXISTS obs_platform.burn_rate_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slo_id UUID NOT NULL,
    slo_name VARCHAR(255) NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    current_rate DECIMAL(10,4) NOT NULL,
    threshold DECIMAL(10,4) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    "window" VARCHAR(50) NOT NULL,
    fired_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP NULL,
    CONSTRAINT fk_bra_slo FOREIGN KEY (slo_id) REFERENCES obs_platform.slos(id) ON DELETE CASCADE
);

CREATE INDEX idx_bra_slo ON obs_platform.burn_rate_alerts(slo_id);
CREATE INDEX idx_bra_severity ON obs_platform.burn_rate_alerts(severity);
CREATE INDEX idx_bra_fired ON obs_platform.burn_rate_alerts(fired_at);

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_slos_updated_at BEFORE UPDATE ON obs_platform.slos
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_slis_updated_at BEFORE UPDATE ON obs_platform.slis
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
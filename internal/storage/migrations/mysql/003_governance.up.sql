-- 003_governance.up.sql
-- Governance and audit schema extension (MySQL version)

-- Budgets table
USE obs_platform;
CREATE TABLE budgets (
    id CHAR(36) PRIMARY KEY,
    service_id CHAR(36),
    name VARCHAR(255) NOT NULL,
    daily_limit DECIMAL(20,4),
    monthly_limit DECIMAL(20,4),
    alert_threshold DECIMAL(5,2) DEFAULT 0.80,
    currency VARCHAR(10) DEFAULT 'USD',
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (service_id) REFERENCES services(id)
);

-- Policies table
CREATE TABLE policies (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,
    description TEXT,
    rules JSON DEFAULT NULL,
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Extend audit_logs
ALTER TABLE audit_logs ADD COLUMN session_id CHAR(36);
ALTER TABLE audit_logs ADD COLUMN prompt_hash VARCHAR(64);
ALTER TABLE audit_logs ADD COLUMN decision JSON;

-- Indexes
CREATE INDEX idx_budgets_service ON budgets(service_id);
CREATE INDEX idx_policies_type ON policies(type);
CREATE INDEX idx_audit_logs_session ON audit_logs(session_id);

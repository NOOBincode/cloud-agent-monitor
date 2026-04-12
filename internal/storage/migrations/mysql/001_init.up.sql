-- 001_init.up.sql
-- Initial schema for obs_platform (MySQL version)

-- Services table (service catalog)
USE obs_platform;
CREATE TABLE services (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    environment VARCHAR(50) NOT NULL DEFAULT 'local',
    labels JSON DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Query templates table (pre-stored queries)
CREATE TABLE query_templates (
    id CHAR(36) PRIMARY KEY,
    service_id CHAR(36),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    query TEXT NOT NULL,
    description TEXT,
    labels JSON DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_service_name (service_id, name),
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
);

-- Audit log table
CREATE TABLE audit_logs (
    id CHAR(36) PRIMARY KEY,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor VARCHAR(255) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(255),
    tenant_id CHAR(36),
    metadata JSON DEFAULT NULL,
    allowed BOOLEAN NOT NULL DEFAULT true,
    reason TEXT
);

-- Create indexes
CREATE INDEX idx_services_environment ON services(environment);
CREATE INDEX idx_query_templates_service ON query_templates(service_id);
CREATE INDEX idx_query_templates_type ON query_templates(type);
CREATE INDEX idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_tenant ON audit_logs(tenant_id);

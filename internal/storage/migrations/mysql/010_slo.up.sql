-- SLO/SLA Management Tables
-- Creates tables for SLO definitions, SLI configurations, and error budget tracking

CREATE TABLE IF NOT EXISTS slos (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    service_id CHAR(36) NOT NULL,
    target DECIMAL(5,2) NOT NULL DEFAULT 99.90,
    window VARCHAR(50) NOT NULL DEFAULT '30d',
    warning_burn DECIMAL(10,2) NOT NULL DEFAULT 2.00,
    critical_burn DECIMAL(10,2) NOT NULL DEFAULT 10.00,
    status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    current_value DECIMAL(10,4) DEFAULT 0,
    burn_rate DECIMAL(10,4) DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    INDEX idx_slo_service (service_id),
    INDEX idx_slo_status (status),
    INDEX idx_slo_deleted (deleted_at),
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS slis (
    id CHAR(36) PRIMARY KEY,
    slo_id CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    query TEXT NOT NULL,
    threshold DECIMAL(20,4) DEFAULT 0,
    unit VARCHAR(50) DEFAULT '',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    INDEX idx_sli_slo (slo_id),
    INDEX idx_sli_type (type),
    INDEX idx_sli_deleted (deleted_at),
    FOREIGN KEY (slo_id) REFERENCES slos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS error_budget_history (
    id CHAR(36) PRIMARY KEY,
    slo_id CHAR(36) NOT NULL,
    total DECIMAL(10,4) NOT NULL,
    remaining DECIMAL(10,4) NOT NULL,
    consumed DECIMAL(10,4) NOT NULL,
    percentage DECIMAL(10,4) NOT NULL,
    burn_rate DECIMAL(10,4) NOT NULL,
    recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_ebh_slo (slo_id),
    INDEX idx_ebh_time (recorded_at),
    FOREIGN KEY (slo_id) REFERENCES slos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS burn_rate_alerts (
    id CHAR(36) PRIMARY KEY,
    slo_id CHAR(36) NOT NULL,
    slo_name VARCHAR(255) NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    current_rate DECIMAL(10,4) NOT NULL,
    threshold DECIMAL(10,4) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    window VARCHAR(50) NOT NULL,
    fired_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP NULL,
    INDEX idx_bra_slo (slo_id),
    INDEX idx_bra_severity (severity),
    INDEX idx_bra_fired (fired_at),
    FOREIGN KEY (slo_id) REFERENCES slos(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

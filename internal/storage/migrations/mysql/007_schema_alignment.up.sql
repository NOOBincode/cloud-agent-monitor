-- 007_schema_alignment.up.sql
-- Align database schema with Go models
-- Note: Run this after clearing database or on fresh installation

USE obs_platform;

-- ============================================================
-- 1. Fix services table - add health check fields
-- ============================================================
ALTER TABLE services 
    ADD COLUMN health_status VARCHAR(20) DEFAULT 'unknown' COMMENT 'Health status: healthy, unhealthy, unknown',
    ADD COLUMN last_health_check_at DATETIME(3) DEFAULT NULL COMMENT 'Last health check timestamp',
    ADD COLUMN health_check_details TEXT DEFAULT NULL COMMENT 'Health check details',
    ADD COLUMN deleted_at DATETIME(3) DEFAULT NULL;

CREATE INDEX idx_services_health_status ON services(health_status);
CREATE INDEX idx_services_deleted_at ON services(deleted_at);

-- ============================================================
-- 2. Create service_labels table for indexed label queries
-- ============================================================
CREATE TABLE service_labels (
    id CHAR(36) PRIMARY KEY,
    service_id CHAR(36) NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    `value` VARCHAR(255) NOT NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_service_labels_service (service_id),
    INDEX idx_service_labels_key (`key`),
    INDEX idx_service_labels_key_value (`key`, `value`),
    UNIQUE INDEX uk_service_label (service_id, `key`, `value`),
    CONSTRAINT fk_service_labels_service FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 3. Create api_keys table (linked to users)
-- Note: This runs after 006_user.up.sql
-- ============================================================
CREATE TABLE api_keys (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    prefix VARCHAR(20) NOT NULL,
    permissions JSON DEFAULT NULL,
    expires_at DATETIME(3) DEFAULT NULL,
    last_used_at DATETIME(3) DEFAULT NULL,
    is_active TINYINT(1) DEFAULT 1,
    tenant_id CHAR(36) DEFAULT NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) DEFAULT NULL,
    UNIQUE INDEX idx_api_keys_key (`key`),
    INDEX idx_api_keys_user_id (user_id),
    INDEX idx_api_keys_is_active (is_active),
    INDEX idx_api_keys_tenant_id (tenant_id),
    INDEX idx_api_keys_deleted_at (deleted_at),
    CONSTRAINT fk_api_keys_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 4. Create cost_records table (moved from 003 and 005)
-- ============================================================
CREATE TABLE cost_records (
    id CHAR(36) PRIMARY KEY,
    session_id CHAR(36) DEFAULT NULL,
    service_id CHAR(36) DEFAULT NULL,
    user_id VARCHAR(255) DEFAULT NULL,
    team_id VARCHAR(255) DEFAULT NULL,
    model_id CHAR(36) DEFAULT NULL,
    cost_type VARCHAR(20) NOT NULL COMMENT 'token, compute, storage, network',
    cost_usd DECIMAL(20,6) NOT NULL,
    input_tokens INT DEFAULT NULL,
    output_tokens INT DEFAULT NULL,
    total_tokens INT DEFAULT NULL,
    gpu_hours DECIMAL(10,6) DEFAULT NULL,
    cpu_hours DECIMAL(10,6) DEFAULT NULL,
    memory_gb_hours DECIMAL(10,6) DEFAULT NULL,
    resource_id VARCHAR(255) DEFAULT NULL,
    region VARCHAR(50) DEFAULT NULL,
    incurred_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    INDEX idx_cost_records_session (session_id),
    INDEX idx_cost_records_service (service_id),
    INDEX idx_cost_records_user (user_id),
    INDEX idx_cost_records_incurred (incurred_at DESC),
    CONSTRAINT fk_cost_records_session FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE SET NULL,
    CONSTRAINT fk_cost_records_service FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE SET NULL,
    CONSTRAINT fk_cost_records_model FOREIGN KEY (model_id) REFERENCES ai_models(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================
-- 5. Ensure prompt_templates table has correct structure
-- ============================================================
ALTER TABLE prompt_templates 
    MODIFY COLUMN template TEXT NOT NULL,
    ADD COLUMN variables JSON DEFAULT NULL,
    ADD COLUMN version INT DEFAULT 1,
    ADD COLUMN labels JSON DEFAULT NULL,
    ADD COLUMN is_active TINYINT(1) DEFAULT TRUE;

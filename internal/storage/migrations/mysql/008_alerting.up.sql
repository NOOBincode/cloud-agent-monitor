-- 008_alerting.up.sql
-- Alert routing module tables
-- Creates tables for alert operations, noise analysis, notifications, and records

USE obs_platform;

-- ============================================================
-- 1. Alert Operations Table - Audit trail for all alert operations
-- ============================================================
CREATE TABLE alert_operations (
    id CHAR(36) PRIMARY KEY,
    alert_fingerprint VARCHAR(64) NOT NULL COMMENT 'Alert fingerprint for deduplication',
    operation_type VARCHAR(20) NOT NULL COMMENT 'send, acknowledge, silence, unsilence, resolve',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' COMMENT 'pending, success, failed, retrying',
    
    user_id CHAR(36) NOT NULL COMMENT 'User who performed the operation',
    tenant_id CHAR(36) DEFAULT NULL COMMENT 'Tenant ID for multi-tenancy',
    
    alert_labels JSON DEFAULT NULL COMMENT 'Alert labels at operation time',
    request_data TEXT DEFAULT NULL COMMENT 'Original request data',
    response_data TEXT DEFAULT NULL COMMENT 'Response data from Alertmanager',
    error_message TEXT DEFAULT NULL COMMENT 'Error message if failed',
    
    retry_count INT DEFAULT 0 COMMENT 'Number of retry attempts',
    max_retries INT DEFAULT 3 COMMENT 'Maximum retry attempts',
    
    ip_address VARCHAR(45) DEFAULT NULL COMMENT 'Client IP address',
    user_agent VARCHAR(500) DEFAULT NULL COMMENT 'Client user agent',
    
    processed_at DATETIME(3) DEFAULT NULL COMMENT 'When operation was processed',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    
    INDEX idx_alert_operations_fingerprint (alert_fingerprint),
    INDEX idx_alert_operations_type (operation_type),
    INDEX idx_alert_operations_status (status),
    INDEX idx_alert_operations_user (user_id),
    INDEX idx_alert_operations_tenant (tenant_id),
    INDEX idx_alert_operations_created (created_at),
    
    CONSTRAINT fk_alert_operations_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Alert operation audit trail';

-- ============================================================
-- 2. Alert Noise Records Table - Noise analysis for alerts
-- ============================================================
CREATE TABLE alert_noise_records (
    id CHAR(36) PRIMARY KEY,
    alert_fingerprint VARCHAR(64) NOT NULL COMMENT 'Unique alert fingerprint',
    alert_name VARCHAR(255) NOT NULL COMMENT 'Alert name from labels',
    alert_labels JSON DEFAULT NULL COMMENT 'Alert labels',
    
    fire_count INT DEFAULT 1 COMMENT 'Number of times alert fired',
    resolve_count INT DEFAULT 0 COMMENT 'Number of times alert resolved',
    noise_score DECIMAL(5,2) DEFAULT 0.00 COMMENT 'Calculated noise score (0-1)',
    
    is_noisy TINYINT(1) DEFAULT 0 COMMENT 'Flag for noisy alerts',
    is_high_risk TINYINT(1) DEFAULT 0 COMMENT 'Flag for high risk alerts',
    
    silence_suggested TINYINT(1) DEFAULT 0 COMMENT 'Suggestion to silence this alert',
    silence_until DATETIME(3) DEFAULT NULL COMMENT 'Silence expiration time',
    
    last_fired_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT 'Last fire timestamp',
    last_resolved_at DATETIME(3) DEFAULT NULL COMMENT 'Last resolve timestamp',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    
    UNIQUE INDEX idx_alert_noise_fingerprint (alert_fingerprint),
    INDEX idx_alert_noise_name (alert_name),
    INDEX idx_alert_noise_noisy (is_noisy),
    INDEX idx_alert_noise_high_risk (is_high_risk),
    INDEX idx_alert_noise_score (noise_score)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Alert noise analysis records';

-- ============================================================
-- 3. Alert Notifications Table - Notification history
-- ============================================================
CREATE TABLE alert_notifications (
    id CHAR(36) PRIMARY KEY,
    alert_fingerprint VARCHAR(64) NOT NULL COMMENT 'Alert fingerprint',
    
    notification_type VARCHAR(50) NOT NULL COMMENT 'email, slack, webhook, etc.',
    recipient VARCHAR(255) NOT NULL COMMENT 'Notification recipient',
    
    status VARCHAR(20) NOT NULL COMMENT 'pending, sent, failed',
    error_message TEXT DEFAULT NULL COMMENT 'Error message if failed',
    
    sent_at DATETIME(3) DEFAULT NULL COMMENT 'When notification was sent',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    INDEX idx_alert_notifications_fingerprint (alert_fingerprint),
    INDEX idx_alert_notifications_type (notification_type),
    INDEX idx_alert_notifications_status (status),
    INDEX idx_alert_notifications_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Alert notification history';

-- ============================================================
-- 4. Alert Records Table - Persistent storage for all alerts
-- ============================================================
CREATE TABLE alert_records (
    id CHAR(36) PRIMARY KEY,
    fingerprint VARCHAR(64) NOT NULL COMMENT 'Alert fingerprint for deduplication',
    
    labels JSON DEFAULT NULL COMMENT 'Alert labels',
    annotations JSON DEFAULT NULL COMMENT 'Alert annotations',
    
    status VARCHAR(20) NOT NULL COMMENT 'firing, resolved',
    severity VARCHAR(20) NOT NULL DEFAULT 'info' COMMENT 'critical, warning, info',
    
    starts_at DATETIME(3) NOT NULL COMMENT 'Alert start time',
    ends_at DATETIME(3) DEFAULT NULL COMMENT 'Alert end time',
    duration BIGINT DEFAULT 0 COMMENT 'Duration in seconds',
    
    source VARCHAR(50) DEFAULT 'alertmanager' COMMENT 'Source: alertmanager, api',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    INDEX idx_alert_records_fingerprint (fingerprint),
    INDEX idx_alert_records_status (status),
    INDEX idx_alert_records_severity (severity),
    INDEX idx_alert_records_starts (starts_at),
    INDEX idx_alert_records_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Persistent alert records';

-- ============================================================
-- 5. Partitioning for alert_records (optional, for high volume)
-- ============================================================
-- Note: Uncomment below for production with high alert volume
-- This creates monthly partitions for the current and next 3 months

-- DELIMITER //
-- CREATE PROCEDURE create_alert_records_partitions()
-- BEGIN
--     DECLARE partition_date DATE;
--     DECLARE partition_name VARCHAR(20);
--     DECLARE i INT DEFAULT 0;
--     
--     WHILE i < 4 DO
--         SET partition_date = DATE_ADD(DATE_FORMAT(NOW(), '%Y-%m-01'), INTERVAL i MONTH);
--         SET partition_name = CONCAT('p', DATE_FORMAT(partition_date, '%Y%m'));
--         
--         SET @sql = CONCAT('ALTER TABLE alert_records ADD PARTITION (
--             PARTITION ', partition_name, ' VALUES LESS THAN (TO_DAYS(''',
--             DATE_FORMAT(DATE_ADD(partition_date, INTERVAL 1 MONTH), '%Y-%m-%d'), '''))
--         )');
--         
--         PREPARE stmt FROM @sql;
--         EXECUTE stmt;
--         DEALLOCATE PREPARE stmt;
--         
--         SET i = i + 1;
--     END WHILE;
-- END //
-- DELIMITER ;
--
-- CALL create_alert_records_partitions();
-- DROP PROCEDURE create_alert_records_partitions;

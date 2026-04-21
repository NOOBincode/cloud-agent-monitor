-- 008_alerting.up.sql
-- Alert routing module tables (PostgreSQL 17 version)

-- ============================================================
-- 1. Alert Operations Table - Audit trail for all alert operations
-- ============================================================
CREATE TABLE obs_platform.alert_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_fingerprint VARCHAR(64) NOT NULL,
    operation_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    
    user_id UUID NOT NULL,
    tenant_id UUID DEFAULT NULL,
    
    alert_labels JSONB DEFAULT NULL,
    request_data TEXT DEFAULT NULL,
    response_data TEXT DEFAULT NULL,
    error_message TEXT DEFAULT NULL,
    
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    
    ip_address VARCHAR(45) DEFAULT NULL,
    user_agent VARCHAR(500) DEFAULT NULL,
    
    processed_at TIMESTAMP(3) DEFAULT NULL,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    CONSTRAINT fk_alert_operations_user FOREIGN KEY (user_id) REFERENCES obs_platform.users(id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_operations_fingerprint ON obs_platform.alert_operations(alert_fingerprint);
CREATE INDEX idx_alert_operations_type ON obs_platform.alert_operations(operation_type);
CREATE INDEX idx_alert_operations_status ON obs_platform.alert_operations(status);
CREATE INDEX idx_alert_operations_user ON obs_platform.alert_operations(user_id);
CREATE INDEX idx_alert_operations_tenant ON obs_platform.alert_operations(tenant_id);
CREATE INDEX idx_alert_operations_created ON obs_platform.alert_operations(created_at);

COMMENT ON TABLE obs_platform.alert_operations IS 'Alert operation audit trail';
COMMENT ON COLUMN obs_platform.alert_operations.operation_type IS 'send, acknowledge, silence, unsilence, resolve';
COMMENT ON COLUMN obs_platform.alert_operations.status IS 'pending, success, failed, retrying';

-- ============================================================
-- 2. Alert Noise Records Table - Noise analysis for alerts
-- ============================================================
CREATE TABLE obs_platform.alert_noise_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_fingerprint VARCHAR(64) NOT NULL,
    alert_name VARCHAR(255) NOT NULL,
    alert_labels JSONB DEFAULT NULL,
    
    fire_count INTEGER DEFAULT 1,
    resolve_count INTEGER DEFAULT 0,
    noise_score DECIMAL(5,2) DEFAULT 0.00,
    
    is_noisy BOOLEAN DEFAULT false,
    is_high_risk BOOLEAN DEFAULT false,
    
    silence_suggested BOOLEAN DEFAULT false,
    silence_until TIMESTAMP(3) DEFAULT NULL,
    
    last_fired_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    last_resolved_at TIMESTAMP(3) DEFAULT NULL,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    CONSTRAINT uk_alert_noise_fingerprint UNIQUE (alert_fingerprint)
);

CREATE INDEX idx_alert_noise_name ON obs_platform.alert_noise_records(alert_name);
CREATE INDEX idx_alert_noise_noisy ON obs_platform.alert_noise_records(is_noisy);
CREATE INDEX idx_alert_noise_high_risk ON obs_platform.alert_noise_records(is_high_risk);
CREATE INDEX idx_alert_noise_score ON obs_platform.alert_noise_records(noise_score);

COMMENT ON TABLE obs_platform.alert_noise_records IS 'Alert noise analysis records';

-- ============================================================
-- 3. Alert Notifications Table - Notification history
-- ============================================================
CREATE TABLE obs_platform.alert_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_fingerprint VARCHAR(64) NOT NULL,
    
    notification_type VARCHAR(50) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    
    status VARCHAR(20) NOT NULL,
    error_message TEXT DEFAULT NULL,
    
    sent_at TIMESTAMP(3) DEFAULT NULL,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);

CREATE INDEX idx_alert_notifications_fingerprint ON obs_platform.alert_notifications(alert_fingerprint);
CREATE INDEX idx_alert_notifications_type ON obs_platform.alert_notifications(notification_type);
CREATE INDEX idx_alert_notifications_status ON obs_platform.alert_notifications(status);
CREATE INDEX idx_alert_notifications_created ON obs_platform.alert_notifications(created_at);

COMMENT ON TABLE obs_platform.alert_notifications IS 'Alert notification history';

-- ============================================================
-- 4. Alert Records Table - Persistent storage for all alerts
-- ============================================================
CREATE TABLE obs_platform.alert_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint VARCHAR(64) NOT NULL,
    
    labels JSONB DEFAULT NULL,
    annotations JSONB DEFAULT NULL,
    
    status VARCHAR(20) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'info',
    
    starts_at TIMESTAMP(3) NOT NULL,
    ends_at TIMESTAMP(3) DEFAULT NULL,
    duration BIGINT DEFAULT 0,
    
    source VARCHAR(50) DEFAULT 'alertmanager',
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);

CREATE INDEX idx_alert_records_fingerprint ON obs_platform.alert_records(fingerprint);
CREATE INDEX idx_alert_records_status ON obs_platform.alert_records(status);
CREATE INDEX idx_alert_records_severity ON obs_platform.alert_records(severity);
CREATE INDEX idx_alert_records_starts ON obs_platform.alert_records(starts_at);
CREATE INDEX idx_alert_records_created ON obs_platform.alert_records(created_at);

COMMENT ON TABLE obs_platform.alert_records IS 'Persistent alert records';
COMMENT ON COLUMN obs_platform.alert_records.source IS 'Source: alertmanager, api';

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_alert_operations_updated_at BEFORE UPDATE ON obs_platform.alert_operations
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_alert_noise_records_updated_at BEFORE UPDATE ON obs_platform.alert_noise_records
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
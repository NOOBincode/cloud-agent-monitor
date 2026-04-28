-- 005_ai_infra_extension.up.sql
-- AI Infrastructure extension: GPU, Inference, Queue, Cost Governance (PostgreSQL 17 version)

-- ============================================================
-- Module 1: GPU Observability
-- ============================================================

-- GPU nodes registry
CREATE TABLE obs_platform.gpu_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    node_name VARCHAR(255) NOT NULL,
    gpu_index INTEGER NOT NULL,
    gpu_uuid VARCHAR(255) NOT NULL,
    gpu_model VARCHAR(255) NOT NULL,
    gpu_memory_total_mb INTEGER NOT NULL,
    mig_enabled BOOLEAN DEFAULT false,
    mig_profile VARCHAR(50),
    
    k8s_node_name VARCHAR(255),
    k8s_pod_name VARCHAR(255),
    namespace VARCHAR(255),
    
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    health_status VARCHAR(20),
    last_health_check TIMESTAMP,
    
    labels JSONB DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uk_gpu_uuid UNIQUE (gpu_uuid),
    CONSTRAINT uk_node_gpu UNIQUE (node_name, gpu_index)
);

COMMENT ON COLUMN obs_platform.gpu_nodes.node_name IS 'Kubernetes node name or hostname';
COMMENT ON COLUMN obs_platform.gpu_nodes.gpu_index IS 'GPU index on the node (0-based)';
COMMENT ON COLUMN obs_platform.gpu_nodes.gpu_uuid IS 'NVIDIA GPU UUID';
COMMENT ON COLUMN obs_platform.gpu_nodes.gpu_model IS 'GPU model (e.g., NVIDIA A100-SXM4-80GB)';
COMMENT ON COLUMN obs_platform.gpu_nodes.gpu_memory_total_mb IS 'Total GPU memory in MB';
COMMENT ON COLUMN obs_platform.gpu_nodes.mig_enabled IS 'Multi-Instance GPU enabled';
COMMENT ON COLUMN obs_platform.gpu_nodes.status IS 'active, maintenance, failed';

-- GPU metrics snapshots
CREATE TABLE obs_platform.gpu_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gpu_node_id UUID NOT NULL,
    
    gpu_utilization DECIMAL(5,2),
    memory_utilization DECIMAL(5,2),
    memory_used_mb INTEGER,
    memory_free_mb INTEGER,
    memory_total_mb INTEGER,
    
    power_usage_w DECIMAL(6,2),
    power_draw_w DECIMAL(6,2),
    temperature_c INTEGER,
    
    sm_clock_mhz INTEGER,
    memory_clock_mhz INTEGER,
    
    pcie_rx_throughput_mb DECIMAL(10,2),
    pcie_tx_throughput_mb DECIMAL(10,2),
    
    nvlink_rx_bytes BIGINT,
    nvlink_tx_bytes BIGINT,
    
    xid_errors JSONB,
    ecc_errors JSONB,
    
    collected_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_gpu_metrics_node FOREIGN KEY (gpu_node_id) REFERENCES obs_platform.gpu_nodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_gpu_metrics_node_time ON obs_platform.gpu_metrics(gpu_node_id, collected_at DESC);
CREATE INDEX idx_gpu_metrics_collected ON obs_platform.gpu_metrics(collected_at DESC);

-- GPU alerts
CREATE TABLE obs_platform.gpu_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gpu_node_id UUID NOT NULL,
    
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    alert_name VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    
    xid_code INTEGER,
    metric_value DECIMAL(20,6),
    threshold DECIMAL(20,6),
    
    status VARCHAR(20) NOT NULL DEFAULT 'firing',
    resolved_at TIMESTAMP,
    
    fired_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_gpu_alerts_node FOREIGN KEY (gpu_node_id) REFERENCES obs_platform.gpu_nodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_gpu_alerts_node ON obs_platform.gpu_alerts(gpu_node_id);
CREATE INDEX idx_gpu_alerts_fired ON obs_platform.gpu_alerts(fired_at DESC);
CREATE INDEX idx_gpu_alerts_status ON obs_platform.gpu_alerts(status);

-- ============================================================
-- Module 2: Inference Service Observability
-- ============================================================

-- Inference services registry
CREATE TABLE obs_platform.inference_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID,
    
    name VARCHAR(255) NOT NULL,
    engine VARCHAR(50) NOT NULL,
    version VARCHAR(50),
    
    model_name VARCHAR(255) NOT NULL,
    model_version VARCHAR(50),
    model_framework VARCHAR(50),
    
    deployment_type VARCHAR(20) NOT NULL,
    replicas INTEGER DEFAULT 1,
    gpu_per_replica INTEGER DEFAULT 1,
    
    config JSONB DEFAULT NULL,
    
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    endpoint_url VARCHAR(500),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_inference_services_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id),
    CONSTRAINT uk_inference_service_name UNIQUE (name)
);

-- Inference request tracking
CREATE TABLE obs_platform.inference_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inference_service_id UUID NOT NULL,
    session_id UUID,
    
    trace_id VARCHAR(64),
    span_id VARCHAR(64),
    
    request_id VARCHAR(255),
    model_name VARCHAR(255),
    gen_ai_system VARCHAR(50),
    
    ttft_ms INTEGER,
    tpot_ms INTEGER,
    e2e_latency_ms INTEGER,
    
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    
    tokens_per_second DECIMAL(10,2),
    
    queue_time_ms INTEGER,
    queue_position INTEGER,
    
    gpu_memory_used_mb INTEGER,
    gpu_utilization DECIMAL(5,2),
    
    batch_size INTEGER,
    
    status VARCHAR(20) NOT NULL,
    error_type VARCHAR(100),
    error_message TEXT,
    
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_inference_requests_service FOREIGN KEY (inference_service_id) REFERENCES obs_platform.inference_services(id) ON DELETE CASCADE,
    CONSTRAINT fk_inference_requests_session FOREIGN KEY (session_id) REFERENCES obs_platform.ai_sessions(id) ON DELETE SET NULL
);

CREATE INDEX idx_inference_requests_service ON obs_platform.inference_requests(inference_service_id);
CREATE INDEX idx_inference_requests_trace ON obs_platform.inference_requests(trace_id);
CREATE INDEX idx_inference_requests_started ON obs_platform.inference_requests(started_at DESC);
CREATE INDEX idx_inference_requests_status ON obs_platform.inference_requests(status);

COMMENT ON COLUMN obs_platform.inference_requests.gen_ai_system IS 'OTel: gen_ai.system (vllm, triton, tgi, openai-compatible)';

-- Model versions management
CREATE TABLE obs_platform.model_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inference_service_id UUID NOT NULL,
    
    version VARCHAR(50) NOT NULL,
    model_path VARCHAR(500),
    
    model_size_mb INTEGER,
    parameters_count BIGINT,
    quantization VARCHAR(50),
    
    avg_ttft_ms INTEGER,
    avg_tpot_ms INTEGER,
    max_throughput_tokens DECIMAL(10,2),
    
    status VARCHAR(20) NOT NULL DEFAULT 'staging',
    is_default BOOLEAN DEFAULT false,
    
    deployed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_model_versions_service FOREIGN KEY (inference_service_id) REFERENCES obs_platform.inference_services(id) ON DELETE CASCADE,
    CONSTRAINT uk_service_version UNIQUE (inference_service_id, version)
);

-- ============================================================
-- Module 3: Queue & Scheduling Observability
-- ============================================================

-- Queue jobs tracking
CREATE TABLE obs_platform.queue_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    job_name VARCHAR(255) NOT NULL,
    job_type VARCHAR(50) NOT NULL,
    queue_name VARCHAR(255),
    
    k8s_namespace VARCHAR(255),
    k8s_job_name VARCHAR(255),
    k8s_uid VARCHAR(255),
    
    gpu_count INTEGER,
    cpu_cores DECIMAL(10,2),
    memory_mb INTEGER,
    
    priority INTEGER,
    scheduler VARCHAR(50),
    queue_position INTEGER,
    
    submitted_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    queue_wait_time_ms BIGINT,
    execution_time_ms BIGINT,
    
    status VARCHAR(20) NOT NULL,
    retry_count INTEGER DEFAULT 0,
    error_message TEXT,
    
    estimated_cost_usd DECIMAL(20,6),
    actual_cost_usd DECIMAL(20,6),
    
    labels JSONB DEFAULT NULL,
    annotations JSONB DEFAULT NULL,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_queue_jobs_status ON obs_platform.queue_jobs(status);
CREATE INDEX idx_queue_jobs_queue ON obs_platform.queue_jobs(queue_name);
CREATE INDEX idx_queue_jobs_submitted ON obs_platform.queue_jobs(submitted_at DESC);
CREATE INDEX idx_queue_jobs_namespace ON obs_platform.queue_jobs(k8s_namespace);

-- Resource allocations
CREATE TABLE obs_platform.resource_allocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID,
    
    resource_type VARCHAR(50) NOT NULL,
    resource_name VARCHAR(255),
    
    requested DECIMAL(20,6),
    allocated DECIMAL(20,6),
    used DECIMAL(20,6),
    unit VARCHAR(20),
    
    node_name VARCHAR(255),
    gpu_index INTEGER,
    
    allocated_at TIMESTAMP NOT NULL,
    released_at TIMESTAMP,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_resource_allocations_job FOREIGN KEY (job_id) REFERENCES obs_platform.queue_jobs(id) ON DELETE CASCADE
);

CREATE INDEX idx_resource_allocations_job ON obs_platform.resource_allocations(job_id);
CREATE INDEX idx_resource_allocations_time ON obs_platform.resource_allocations(allocated_at DESC);

-- ============================================================
-- Module 4: Cost Governance
-- ============================================================

-- Cost budgets
CREATE TABLE obs_platform.cost_budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    scope_type VARCHAR(20) NOT NULL,
    scope_id VARCHAR(255),
    scope_name VARCHAR(255),
    
    period VARCHAR(20) NOT NULL,
    
    token_limit BIGINT,
    cost_limit_usd DECIMAL(20,6),
    request_limit BIGINT,
    
    current_tokens BIGINT DEFAULT 0,
    current_cost_usd DECIMAL(20,6) DEFAULT 0,
    current_requests BIGINT DEFAULT 0,
    
    alert_threshold_50 BOOLEAN DEFAULT false,
    alert_threshold_80 BOOLEAN DEFAULT true,
    alert_threshold_100 BOOLEAN DEFAULT true,
    
    action_on_exceed VARCHAR(20),
    downgrade_model VARCHAR(255),
    
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    
    period_start TIMESTAMP NOT NULL,
    period_end TIMESTAMP NOT NULL,
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uk_budget_scope_period UNIQUE (scope_type, scope_id, period, period_start)
);

CREATE INDEX idx_cost_budgets_status ON obs_platform.cost_budgets(status);
CREATE INDEX idx_cost_budgets_period ON obs_platform.cost_budgets(period_start, period_end);

-- Budget alerts
CREATE TABLE obs_platform.budget_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    budget_id UUID NOT NULL,
    
    threshold_pct INTEGER NOT NULL,
    alert_type VARCHAR(20) NOT NULL,
    
    current_tokens BIGINT,
    current_cost_usd DECIMAL(20,6),
    current_requests BIGINT,
    usage_percentage DECIMAL(5,2),
    
    notified BOOLEAN DEFAULT false,
    notified_at TIMESTAMP,
    notification_channels JSONB,
    
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    acknowledged_at TIMESTAMP,
    acknowledged_by VARCHAR(255),
    
    triggered_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_budget_alerts_budget FOREIGN KEY (budget_id) REFERENCES obs_platform.cost_budgets(id) ON DELETE CASCADE
);

CREATE INDEX idx_budget_alerts_budget ON obs_platform.budget_alerts(budget_id);
CREATE INDEX idx_budget_alerts_triggered ON obs_platform.budget_alerts(triggered_at DESC);
CREATE INDEX idx_budget_alerts_status ON obs_platform.budget_alerts(status);

-- ============================================================
-- Module 5: Security & Audit Enhancement
-- ============================================================

-- Prompt audit logs
CREATE TABLE obs_platform.prompt_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID,
    
    user_id VARCHAR(255),
    service_id UUID,
    
    prompt_hash VARCHAR(255),
    prompt_length INTEGER,
    
    injection_detected BOOLEAN DEFAULT false,
    injection_type VARCHAR(50),
    injection_confidence DECIMAL(5,2),
    
    pii_detected BOOLEAN DEFAULT false,
    pii_types JSONB,
    
    policy_violation BOOLEAN DEFAULT false,
    policy_violation_type VARCHAR(50),
    
    action_taken VARCHAR(20),
    action_reason TEXT,
    
    audited_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_prompt_audit_session FOREIGN KEY (session_id) REFERENCES obs_platform.ai_sessions(id) ON DELETE SET NULL,
    CONSTRAINT fk_prompt_audit_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id) ON DELETE SET NULL
);

CREATE INDEX idx_prompt_audit_session ON obs_platform.prompt_audit_logs(session_id);
CREATE INDEX idx_prompt_audit_user ON obs_platform.prompt_audit_logs(user_id);
CREATE INDEX idx_prompt_audit_audited ON obs_platform.prompt_audit_logs(audited_at DESC);
CREATE INDEX idx_prompt_audit_injection ON obs_platform.prompt_audit_logs(injection_detected);
CREATE INDEX idx_prompt_audit_pii ON obs_platform.prompt_audit_logs(pii_detected);

-- Tool execution logs
CREATE TABLE obs_platform.tool_execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_call_id UUID,
    session_id UUID,
    
    agent_id VARCHAR(255),
    user_id VARCHAR(255),
    
    tool_name VARCHAR(255) NOT NULL,
    tool_type VARCHAR(50) NOT NULL,
    
    is_whitelisted BOOLEAN DEFAULT true,
    permission_check_passed BOOLEAN DEFAULT true,
    rate_limit_exceeded BOOLEAN DEFAULT false,
    
    data_sources_accessed JSONB,
    sensitive_data_accessed BOOLEAN DEFAULT false,
    data_classification VARCHAR(20),
    
    execution_status VARCHAR(20) NOT NULL,
    error_type VARCHAR(100),
    error_message TEXT,
    
    duration_ms INTEGER,
    
    audit_hash VARCHAR(255),
    
    executed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_tool_exec_call FOREIGN KEY (tool_call_id) REFERENCES obs_platform.tool_calls(id) ON DELETE SET NULL,
    CONSTRAINT fk_tool_exec_session FOREIGN KEY (session_id) REFERENCES obs_platform.ai_sessions(id) ON DELETE SET NULL
);

CREATE INDEX idx_tool_exec_session ON obs_platform.tool_execution_logs(session_id);
CREATE INDEX idx_tool_exec_tool ON obs_platform.tool_execution_logs(tool_name);
CREATE INDEX idx_tool_exec_agent ON obs_platform.tool_execution_logs(agent_id);
CREATE INDEX idx_tool_exec_executed ON obs_platform.tool_execution_logs(executed_at DESC);
CREATE INDEX idx_tool_exec_status ON obs_platform.tool_execution_logs(execution_status);

-- Additional indexes
CREATE INDEX idx_gpu_nodes_status ON obs_platform.gpu_nodes(status);
CREATE INDEX idx_gpu_nodes_k8s ON obs_platform.gpu_nodes(k8s_node_name);
CREATE INDEX idx_inference_services_status ON obs_platform.inference_services(status);
CREATE INDEX idx_inference_services_engine ON obs_platform.inference_services(engine);

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_gpu_nodes_updated_at BEFORE UPDATE ON obs_platform.gpu_nodes
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_inference_services_updated_at BEFORE UPDATE ON obs_platform.inference_services
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_model_versions_updated_at BEFORE UPDATE ON obs_platform.model_versions
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_queue_jobs_updated_at BEFORE UPDATE ON obs_platform.queue_jobs
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_cost_budgets_updated_at BEFORE UPDATE ON obs_platform.cost_budgets
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
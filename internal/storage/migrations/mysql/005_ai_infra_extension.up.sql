-- 005_ai_infra_extension.up.sql
-- AI Infrastructure extension: GPU, Inference, Queue, Cost Governance
-- Complements the AI Observability schema with infrastructure-level metrics

-- ============================================================
-- Module 1: GPU Observability
-- ============================================================

-- GPU nodes registry
USE obs_platform;
CREATE TABLE gpu_nodes (
    id CHAR(36) PRIMARY KEY,
    node_name VARCHAR(255) NOT NULL COMMENT 'Kubernetes node name or hostname',
    gpu_index INT NOT NULL COMMENT 'GPU index on the node (0-based)',
    gpu_uuid VARCHAR(255) NOT NULL COMMENT 'NVIDIA GPU UUID',
    gpu_model VARCHAR(255) NOT NULL COMMENT 'GPU model (e.g., NVIDIA A100-SXM4-80GB)',
    gpu_memory_total_mb INT NOT NULL COMMENT 'Total GPU memory in MB',
    mig_enabled BOOLEAN DEFAULT false COMMENT 'Multi-Instance GPU enabled',
    mig_profile VARCHAR(50) COMMENT 'MIG profile (e.g., MIG 1g.10gb)',
    
    -- Kubernetes context
    k8s_node_name VARCHAR(255) COMMENT 'Kubernetes node name',
    k8s_pod_name VARCHAR(255) COMMENT 'Pod using this GPU (if dedicated)',
    namespace VARCHAR(255) COMMENT 'Kubernetes namespace',
    
    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active' COMMENT 'active, maintenance, failed',
    health_status VARCHAR(20) COMMENT 'healthy, degraded, critical',
    last_health_check DATETIME,
    
    -- Metadata
    labels JSON DEFAULT NULL COMMENT 'Custom labels',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_gpu_uuid (gpu_uuid),
    UNIQUE KEY uk_node_gpu (node_name, gpu_index)
);

-- GPU metrics snapshots (collected from DCGM Exporter)
CREATE TABLE gpu_metrics (
    id CHAR(36) PRIMARY KEY,
    gpu_node_id CHAR(36) NOT NULL,
    
    -- DCGM Field IDs (https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/field-ids.html)
    -- Compute & Memory
    gpu_utilization DECIMAL(5,2) COMMENT 'DCGM_FI_DEV_GPU_UTIL (0-100%)',
    memory_utilization DECIMAL(5,2) COMMENT 'DCGM_FI_DEV_MEM_COPY_UTIL (0-100%)',
    memory_used_mb INT COMMENT 'DCGM_FI_DEV_FB_USED',
    memory_free_mb INT COMMENT 'DCGM_FI_DEV_FB_FREE',
    
    -- Power & Temperature
    power_usage_w DECIMAL(6,2) COMMENT 'DCGM_FI_DEV_POWER_USAGE (Watts)',
    power_draw_w DECIMAL(6,2) COMMENT 'DCGM_FI_DEV_POWER_DRAW (Watts)',
    temperature_c INT COMMENT 'DCGM_FI_DEV_GPU_TEMP (Celsius)',
    
    -- Clock & Performance
    sm_clock_mhz INT COMMENT 'DCGM_FI_DEV_SM_CLOCK (Streaming Multiprocessor)',
    memory_clock_mhz INT COMMENT 'DCGM_FI_DEV_MEM_CLOCK',
    
    -- PCIe Throughput
    pcie_rx_throughput_mb DECIMAL(10,2) COMMENT 'DCGM_FI_DEV_PCIE_RX_THROUGHPUT',
    pcie_tx_throughput_mb DECIMAL(10,2) COMMENT 'DCGM_FI_DEV_PCIE_TX_THROUGHPUT',
    
    -- NVLink (if applicable)
    nvlink_rx_bytes BIGINT COMMENT 'NVLink receive bytes',
    nvlink_tx_bytes BIGINT COMMENT 'NVLink transmit bytes',
    
    -- Errors
    xid_errors JSON COMMENT 'DCGM_FI_DEV_XID_ERRORS (array of error codes)',
    ecc_errors JSON COMMENT 'ECC error counts',
    
    -- Timestamp
    collected_at DATETIME NOT NULL COMMENT 'Metric collection timestamp',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (gpu_node_id) REFERENCES gpu_nodes(id) ON DELETE CASCADE,
    INDEX idx_gpu_metrics_node_time (gpu_node_id, collected_at DESC),
    INDEX idx_gpu_metrics_collected (collected_at DESC)
);

-- GPU alerts and events
CREATE TABLE gpu_alerts (
    id CHAR(36) PRIMARY KEY,
    gpu_node_id CHAR(36) NOT NULL,
    
    -- Alert info
    alert_type VARCHAR(50) NOT NULL COMMENT 'xid_error, temperature, memory, power, ecc',
    severity VARCHAR(20) NOT NULL COMMENT 'critical, warning, info',
    alert_name VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    
    -- Context
    xid_code INT COMMENT 'NVIDIA XID error code (if applicable)',
    metric_value DECIMAL(20,6) COMMENT 'Metric value that triggered the alert',
    threshold DECIMAL(20,6) COMMENT 'Threshold that was exceeded',
    
    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'firing' COMMENT 'firing, resolved',
    resolved_at DATETIME,
    
    -- Timestamps
    fired_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (gpu_node_id) REFERENCES gpu_nodes(id) ON DELETE CASCADE,
    INDEX idx_gpu_alerts_node (gpu_node_id),
    INDEX idx_gpu_alerts_fired (fired_at DESC),
    INDEX idx_gpu_alerts_status (status)
);

-- ============================================================
-- Module 2: Inference Service Observability
-- ============================================================

-- Inference services registry
CREATE TABLE inference_services (
    id CHAR(36) PRIMARY KEY,
    service_id CHAR(36) COMMENT 'Reference to services table',
    
    -- Service info
    name VARCHAR(255) NOT NULL COMMENT 'Service name (e.g., vllm-chat, triton-resnet)',
    engine VARCHAR(50) NOT NULL COMMENT 'vllm, triton, tensorrt, tgi, openai-compatible',
    version VARCHAR(50) COMMENT 'Service version',
    
    -- Model info
    model_name VARCHAR(255) NOT NULL COMMENT 'Model name',
    model_version VARCHAR(50) COMMENT 'Model version',
    model_framework VARCHAR(50) COMMENT 'pytorch, tensorflow, onnx, tensorrt',
    
    -- Deployment
    deployment_type VARCHAR(20) NOT NULL COMMENT 'deployment, statefulset, pod',
    replicas INT DEFAULT 1,
    gpu_per_replica INT DEFAULT 1,
    
    -- Configuration
    config JSON DEFAULT NULL COMMENT 'Engine configuration',
    
    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active' COMMENT 'active, degraded, inactive',
    endpoint_url VARCHAR(500) COMMENT 'Inference endpoint URL',
    
    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    FOREIGN KEY (service_id) REFERENCES services(id),
    UNIQUE KEY uk_inference_service_name (name)
);

-- Inference request tracking (TTFT/TPOT metrics)
CREATE TABLE inference_requests (
    id CHAR(36) PRIMARY KEY,
    inference_service_id CHAR(36) NOT NULL,
    session_id CHAR(36) COMMENT 'Link to ai_sessions (if applicable)',
    
    -- Request context
    trace_id VARCHAR(64) COMMENT 'OTel trace.id',
    span_id VARCHAR(64) COMMENT 'OTel span.id',
    
    -- Request details
    request_id VARCHAR(255) COMMENT 'Request ID from inference engine',
    model_name VARCHAR(255) COMMENT 'Model used for inference',
    
    -- Performance metrics (vLLM/Triton metrics)
    ttft_ms INT COMMENT 'Time to First Token (milliseconds)',
    tpot_ms INT COMMENT 'Time per Output Token (milliseconds)',
    e2e_latency_ms INT COMMENT 'End-to-end latency (milliseconds)',
    
    -- Token metrics
    prompt_tokens INT COMMENT 'Input tokens',
    completion_tokens INT COMMENT 'Output tokens',
    total_tokens INT COMMENT 'Total tokens',
    
    -- Throughput
    tokens_per_second DECIMAL(10,2) COMMENT 'Generation throughput',
    
    -- Queue & Scheduling
    queue_time_ms INT COMMENT 'Time spent in queue',
    queue_position INT COMMENT 'Position in queue when request arrived',
    
    -- GPU utilization during request
    gpu_memory_used_mb INT COMMENT 'GPU memory used during inference',
    gpu_utilization DECIMAL(5,2) COMMENT 'GPU utilization during inference',
    
    -- Batch info (for batch inference)
    batch_size INT COMMENT 'Batch size (if batched)',
    
    -- Status
    status VARCHAR(20) NOT NULL COMMENT 'success, error, timeout, cancelled',
    error_type VARCHAR(100) COMMENT 'Error type (if failed)',
    error_message TEXT COMMENT 'Error message (if failed)',
    
    -- Timestamps
    started_at DATETIME NOT NULL COMMENT 'Request start time',
    completed_at DATETIME COMMENT 'Request completion time',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (inference_service_id) REFERENCES inference_services(id) ON DELETE CASCADE,
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE SET NULL,
    INDEX idx_inference_requests_service (inference_service_id),
    INDEX idx_inference_requests_trace (trace_id),
    INDEX idx_inference_requests_started (started_at DESC),
    INDEX idx_inference_requests_status (status)
);

-- Model versions management
CREATE TABLE model_versions (
    id CHAR(36) PRIMARY KEY,
    inference_service_id CHAR(36) NOT NULL,
    
    -- Version info
    version VARCHAR(50) NOT NULL COMMENT 'Model version',
    model_path VARCHAR(500) COMMENT 'Model path or URI',
    
    -- Model metadata
    model_size_mb INT COMMENT 'Model size in MB',
    parameters_count BIGINT COMMENT 'Number of parameters',
    quantization VARCHAR(50) COMMENT 'Quantization type (fp16, int8, int4, gptq, awq)',
    
    -- Performance benchmarks
    avg_ttft_ms INT COMMENT 'Average TTFT from benchmarks',
    avg_tpot_ms INT COMMENT 'Average TPOT from benchmarks',
    max_throughput_tokens DECIMAL(10,2) COMMENT 'Max throughput (tokens/s)',
    
    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'staging' COMMENT 'staging, production, deprecated',
    is_default BOOLEAN DEFAULT false COMMENT 'Default version for the service',
    
    -- Timestamps
    deployed_at DATETIME COMMENT 'Deployment timestamp',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    FOREIGN KEY (inference_service_id) REFERENCES inference_services(id) ON DELETE CASCADE,
    UNIQUE KEY uk_service_version (inference_service_id, version)
);

-- ============================================================
-- Module 3: Queue & Scheduling Observability
-- ============================================================

-- Queue jobs tracking
CREATE TABLE queue_jobs (
    id CHAR(36) PRIMARY KEY,
    
    -- Job identification
    job_name VARCHAR(255) NOT NULL COMMENT 'Job name',
    job_type VARCHAR(50) NOT NULL COMMENT 'training, inference, batch, evaluation',
    queue_name VARCHAR(255) COMMENT 'Queue name (e.g., volcano queue)',
    
    -- Kubernetes context
    k8s_namespace VARCHAR(255) COMMENT 'Kubernetes namespace',
    k8s_job_name VARCHAR(255) COMMENT 'Kubernetes Job name',
    k8s_uid VARCHAR(255) COMMENT 'Kubernetes UID',
    
    -- Resource requirements
    gpu_count INT COMMENT 'Requested GPU count',
    cpu_cores DECIMAL(10,2) COMMENT 'Requested CPU cores',
    memory_mb INT COMMENT 'Requested memory in MB',
    
    -- Scheduling
    priority INT COMMENT 'Job priority',
    scheduler VARCHAR(50) COMMENT 'Scheduler name (volcano, default-scheduler)',
    queue_position INT COMMENT 'Position in queue',
    
    -- Timing
    submitted_at DATETIME COMMENT 'Job submission time',
    started_at DATETIME COMMENT 'Job start time',
    completed_at DATETIME COMMENT 'Job completion time',
    queue_wait_time_ms BIGINT COMMENT 'Time spent in queue (milliseconds)',
    execution_time_ms BIGINT COMMENT 'Execution time (milliseconds)',
    
    -- Status
    status VARCHAR(20) NOT NULL COMMENT 'pending, running, succeeded, failed, cancelled',
    retry_count INT DEFAULT 0,
    error_message TEXT COMMENT 'Error message (if failed)',
    
    -- Cost
    estimated_cost_usd DECIMAL(20,6) COMMENT 'Estimated cost',
    actual_cost_usd DECIMAL(20,6) COMMENT 'Actual cost',
    
    -- Metadata
    labels JSON DEFAULT NULL COMMENT 'Job labels',
    annotations JSON DEFAULT NULL COMMENT 'Job annotations',
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_queue_jobs_status (status),
    INDEX idx_queue_jobs_queue (queue_name),
    INDEX idx_queue_jobs_submitted (submitted_at DESC),
    INDEX idx_queue_jobs_namespace (k8s_namespace)
);

-- Resource allocations
CREATE TABLE resource_allocations (
    id CHAR(36) PRIMARY KEY,
    job_id CHAR(36) COMMENT 'Reference to queue_jobs',
    
    -- Resource type
    resource_type VARCHAR(50) NOT NULL COMMENT 'gpu, cpu, memory, storage',
    resource_name VARCHAR(255) COMMENT 'Resource identifier',
    
    -- Allocation
    requested DECIMAL(20,6) COMMENT 'Requested amount',
    allocated DECIMAL(20,6) COMMENT 'Allocated amount',
    used DECIMAL(20,6) COMMENT 'Actually used amount',
    unit VARCHAR(20) COMMENT 'Unit (cores, mb, gb, count)',
    
    -- Node assignment
    node_name VARCHAR(255) COMMENT 'Node where resource is allocated',
    gpu_index INT COMMENT 'GPU index (if resource_type=gpu)',
    
    -- Time window
    allocated_at DATETIME NOT NULL,
    released_at DATETIME,
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (job_id) REFERENCES queue_jobs(id) ON DELETE CASCADE,
    INDEX idx_resource_allocations_job (job_id),
    INDEX idx_resource_allocations_time (allocated_at DESC)
);

-- ============================================================
-- Module 4: Cost Governance
-- ============================================================

-- Cost budgets
CREATE TABLE cost_budgets (
    id CHAR(36) PRIMARY KEY,
    
    -- Budget scope
    scope_type VARCHAR(20) NOT NULL COMMENT 'global, service, user, team, model',
    scope_id VARCHAR(255) COMMENT 'ID of the scope entity',
    scope_name VARCHAR(255) COMMENT 'Human-readable scope name',
    
    -- Budget period
    period VARCHAR(20) NOT NULL COMMENT 'daily, weekly, monthly, quarterly',
    
    -- Limits
    token_limit BIGINT COMMENT 'Token limit',
    cost_limit_usd DECIMAL(20,6) COMMENT 'Cost limit in USD',
    request_limit BIGINT COMMENT 'Request count limit',
    
    -- Current usage (updated periodically)
    current_tokens BIGINT DEFAULT 0,
    current_cost_usd DECIMAL(20,6) DEFAULT 0,
    current_requests BIGINT DEFAULT 0,
    
    -- Alert thresholds
    alert_threshold_50 BOOLEAN DEFAULT false COMMENT 'Alert at 50%',
    alert_threshold_80 BOOLEAN DEFAULT true COMMENT 'Alert at 80%',
    alert_threshold_100 BOOLEAN DEFAULT true COMMENT 'Alert at 100%',
    
    -- Actions
    action_on_exceed VARCHAR(20) COMMENT 'throttle, downgrade, block, notify',
    downgrade_model VARCHAR(255) COMMENT 'Model to downgrade to (if action=downgrade)',
    
    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active' COMMENT 'active, exceeded, paused',
    
    -- Period tracking
    period_start DATETIME NOT NULL,
    period_end DATETIME NOT NULL,
    
    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_budget_scope_period (scope_type, scope_id, period, period_start),
    INDEX idx_cost_budgets_status (status),
    INDEX idx_cost_budgets_period (period_start, period_end)
);

-- Budget alerts
CREATE TABLE budget_alerts (
    id CHAR(36) PRIMARY KEY,
    budget_id CHAR(36) NOT NULL,
    
    -- Alert info
    threshold_pct INT NOT NULL COMMENT 'Threshold percentage (50, 80, 100)',
    alert_type VARCHAR(20) NOT NULL COMMENT 'warning, critical, exceeded',
    
    -- Usage at alert time
    current_tokens BIGINT,
    current_cost_usd DECIMAL(20,6),
    current_requests BIGINT,
    usage_percentage DECIMAL(5,2) COMMENT 'Actual usage percentage',
    
    -- Notification
    notified BOOLEAN DEFAULT false,
    notified_at DATETIME,
    notification_channels JSON COMMENT 'Email, Slack, etc.',
    
    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'active' COMMENT 'active, acknowledged, resolved',
    acknowledged_at DATETIME,
    acknowledged_by VARCHAR(255),
    
    -- Timestamps
    triggered_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (budget_id) REFERENCES cost_budgets(id) ON DELETE CASCADE,
    INDEX idx_budget_alerts_budget (budget_id),
    INDEX idx_budget_alerts_triggered (triggered_at DESC),
    INDEX idx_budget_alerts_status (status)
);

-- ============================================================
-- Module 5: Security & Audit Enhancement
-- ============================================================

-- Prompt audit logs (for security and compliance)
CREATE TABLE prompt_audit_logs (
    id CHAR(36) PRIMARY KEY,
    session_id CHAR(36) COMMENT 'Reference to ai_sessions',
    
    -- Audit context
    user_id VARCHAR(255) COMMENT 'User who sent the prompt',
    service_id CHAR(36) COMMENT 'Service used',
    
    -- Prompt content (hashed for privacy)
    prompt_hash VARCHAR(255) COMMENT 'SHA-256 hash of prompt',
    prompt_length INT COMMENT 'Prompt length in characters',
    
    -- Security checks
    injection_detected BOOLEAN DEFAULT false COMMENT 'Prompt injection detected',
    injection_type VARCHAR(50) COMMENT 'Type of injection (if detected)',
    injection_confidence DECIMAL(5,2) COMMENT 'Confidence score (0-100)',
    
    -- PII detection
    pii_detected BOOLEAN DEFAULT false COMMENT 'PII detected in prompt',
    pii_types JSON COMMENT 'Types of PII detected (email, phone, ssn, etc.)',
    
    -- Content policy
    policy_violation BOOLEAN DEFAULT false COMMENT 'Content policy violation',
    policy_violation_type VARCHAR(50) COMMENT 'Type of violation',
    
    -- Action taken
    action_taken VARCHAR(20) COMMENT 'allowed, blocked, sanitized, flagged',
    action_reason TEXT COMMENT 'Reason for action',
    
    -- Timestamps
    audited_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE SET NULL,
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE SET NULL,
    INDEX idx_prompt_audit_session (session_id),
    INDEX idx_prompt_audit_user (user_id),
    INDEX idx_prompt_audit_audited (audited_at DESC),
    INDEX idx_prompt_audit_injection (injection_detected),
    INDEX idx_prompt_audit_pii (pii_detected)
);

-- Tool execution logs (enhanced version for security)
CREATE TABLE tool_execution_logs (
    id CHAR(36) PRIMARY KEY,
    tool_call_id CHAR(36) COMMENT 'Reference to tool_calls',
    session_id CHAR(36) COMMENT 'Reference to ai_sessions',
    
    -- Execution context
    agent_id VARCHAR(255) COMMENT 'Agent ID that executed the tool',
    user_id VARCHAR(255) COMMENT 'User ID on behalf of whom tool was executed',
    
    -- Tool info
    tool_name VARCHAR(255) NOT NULL,
    tool_type VARCHAR(50) NOT NULL,
    
    -- Security checks
    is_whitelisted BOOLEAN DEFAULT true COMMENT 'Tool is in whitelist',
    permission_check_passed BOOLEAN DEFAULT true COMMENT 'Permission check result',
    rate_limit_exceeded BOOLEAN DEFAULT false COMMENT 'Rate limit exceeded',
    
    -- Data access
    data_sources_accessed JSON COMMENT 'List of data sources accessed',
    sensitive_data_accessed BOOLEAN DEFAULT false COMMENT 'Sensitive data was accessed',
    data_classification VARCHAR(20) COMMENT 'public, internal, confidential, restricted',
    
    -- Execution result
    execution_status VARCHAR(20) NOT NULL COMMENT 'success, error, blocked, timeout',
    error_type VARCHAR(100),
    error_message TEXT,
    
    -- Performance
    duration_ms INT,
    
    -- Audit trail
    audit_hash VARCHAR(255) COMMENT 'Hash of execution for integrity verification',
    
    -- Timestamps
    executed_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (tool_call_id) REFERENCES tool_calls(id) ON DELETE SET NULL,
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE SET NULL,
    INDEX idx_tool_exec_session (session_id),
    INDEX idx_tool_exec_tool (tool_name),
    INDEX idx_tool_exec_agent (agent_id),
    INDEX idx_tool_exec_executed (executed_at DESC),
    INDEX idx_tool_exec_status (execution_status)
);

-- ============================================================
-- Indexes for all new tables
-- ============================================================

-- Additional indexes for GPU tables
CREATE INDEX idx_gpu_nodes_status ON gpu_nodes(status);
CREATE INDEX idx_gpu_nodes_k8s ON gpu_nodes(k8s_node_name);

-- Additional indexes for inference tables
CREATE INDEX idx_inference_services_status ON inference_services(status);
CREATE INDEX idx_inference_services_engine ON inference_services(engine);

-- 002_ai_observability.up.sql
-- AI Observability schema extension (MySQL version)
-- Aligned with OpenTelemetry GenAI Semantic Conventions v1.37+

-- AI models configuration
USE obs_platform;
CREATE TABLE ai_models (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL COMMENT 'OTel: gen_ai.system (openai, anthropic, vertex_ai, bedrock)',
    model_id VARCHAR(255) NOT NULL COMMENT 'OTel: gen_ai.request.model',
    config JSON DEFAULT NULL,
    cost_per_input_token DECIMAL(20,10),
    cost_per_output_token DECIMAL(20,10),
    enabled BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- AI sessions tracking (aligned with OTel GenAI Semantic Conventions)
CREATE TABLE ai_sessions (
    id CHAR(36) PRIMARY KEY,
    
    -- OpenTelemetry Trace Context
    trace_id VARCHAR(64) NOT NULL COMMENT 'OTel: trace.id',
    span_id VARCHAR(64) COMMENT 'OTel: span.id',
    parent_span_id VARCHAR(64) COMMENT 'OTel: parent.span.id',
    
    -- Service Context
    service_id CHAR(36) COMMENT 'Reference to services table',
    model_id CHAR(36) COMMENT 'Reference to ai_models table',
    
    -- OTel GenAI Semantic Conventions - System
    gen_ai_system VARCHAR(50) COMMENT 'OTel: gen_ai.system (openai, anthropic, vertex_ai, bedrock, ollama)',
    
    -- OTel GenAI Semantic Conventions - Operation
    gen_ai_operation_name VARCHAR(50) NOT NULL COMMENT 'OTel: gen_ai.operation.name (chat, completion, embeddings)',
    
    -- OTel GenAI Semantic Conventions - Request
    gen_ai_request_model VARCHAR(255) COMMENT 'OTel: gen_ai.request.model (requested model)',
    gen_ai_request_max_tokens INT COMMENT 'OTel: gen_ai.request.max_tokens',
    gen_ai_request_temperature DECIMAL(3,2) COMMENT 'OTel: gen_ai.request.temperature',
    gen_ai_request_top_p DECIMAL(3,2) COMMENT 'OTel: gen_ai.request.top_p',
    gen_ai_request_presence_penalty DECIMAL(3,2) COMMENT 'OTel: gen_ai.request.presence_penalty',
    gen_ai_request_frequency_penalty DECIMAL(3,2) COMMENT 'OTel: gen_ai.request.frequency_penalty',
    gen_ai_request_stop_sequences JSON COMMENT 'OTel: gen_ai.request.stop_sequences',
    
    -- OTel GenAI Semantic Conventions - Response
    gen_ai_response_model VARCHAR(255) COMMENT 'OTel: gen_ai.response.model (actual model used)',
    gen_ai_response_finish_reasons JSON COMMENT 'OTel: gen_ai.response.finish_reasons',
    gen_ai_response_id VARCHAR(255) COMMENT 'OTel: gen_ai.response.id',
    
    -- OTel GenAI Semantic Conventions - Usage
    gen_ai_usage_input_tokens INT COMMENT 'OTel: gen_ai.usage.input_tokens',
    gen_ai_usage_output_tokens INT COMMENT 'OTel: gen_ai.usage.output_tokens',
    gen_ai_usage_total_tokens INT COMMENT 'OTel: gen_ai.usage.total_tokens (computed)',
    
    -- Legacy fields (for backward compatibility, will be deprecated)
    operation VARCHAR(50) COMMENT 'Legacy: use gen_ai_operation_name instead',
    prompt_tokens INT COMMENT 'Legacy: use gen_ai_usage_input_tokens instead',
    completion_tokens INT COMMENT 'Legacy: use gen_ai_usage_output_tokens instead',
    total_tokens INT COMMENT 'Legacy: use gen_ai_usage_total_tokens instead',
    
    -- Performance Metrics
    duration_ms INT COMMENT 'Total operation duration in milliseconds',
    ttft_ms INT COMMENT 'Time to First Token (for streaming)',
    tpot_ms INT COMMENT 'Time per Output Token (for streaming)',
    
    -- Status & Error
    status VARCHAR(20) NOT NULL COMMENT 'OTel: status.code (success, error, throttled)',
    error_type VARCHAR(100) COMMENT 'OTel: error.type',
    error_message TEXT COMMENT 'OTel: error.message',
    
    -- OTel Resource Attributes
    resource_attributes JSON COMMENT 'OTel: resource.attributes (service.name, service.version, etc.)',
    
    -- Additional Metadata
    user_id VARCHAR(255) COMMENT 'End user identifier for attribution',
    session_type VARCHAR(50) COMMENT 'Type: chat, agent, batch, evaluation',
    metadata JSON DEFAULT NULL COMMENT 'Additional metadata',
    
    -- Cost (computed)
    cost_usd DECIMAL(20,6) COMMENT 'Computed cost in USD',
    
    -- Timestamps
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (service_id) REFERENCES services(id),
    FOREIGN KEY (model_id) REFERENCES ai_models(id)
);

-- Tool calls recording (aligned with OTel GenAI Semantic Conventions)
CREATE TABLE tool_calls (
    id CHAR(36) PRIMARY KEY,
    session_id CHAR(36) NOT NULL,
    span_id VARCHAR(64) NOT NULL COMMENT 'OTel: span.id',
    parent_span_id VARCHAR(64) COMMENT 'OTel: parent.span.id (link to ai_sessions.span_id)',
    
    -- OTel GenAI Tool Attributes
    gen_ai_tool_name VARCHAR(255) NOT NULL COMMENT 'OTel: gen_ai.tool.name',
    gen_ai_tool_type VARCHAR(50) NOT NULL COMMENT 'OTel: gen_ai.tool.type (function, code_interpreter, retrieval)',
    gen_ai_tool_description TEXT COMMENT 'OTel: gen_ai.tool.description',
    gen_ai_tool_call_id VARCHAR(255) COMMENT 'OTel: gen_ai.tool.call.id',
    
    -- Legacy fields (for backward compatibility)
    tool_name VARCHAR(255) COMMENT 'Legacy: use gen_ai_tool_name instead',
    tool_type VARCHAR(50) COMMENT 'Legacy: use gen_ai_tool_type instead',
    
    -- Input/Output
    arguments JSON DEFAULT NULL COMMENT 'OTel: gen_ai.tool.call.arguments',
    result JSON DEFAULT NULL COMMENT 'OTel: gen_ai.tool.result',
    
    -- Status & Error
    status VARCHAR(20) NOT NULL COMMENT 'OTel: status.code (success, error)',
    error_type VARCHAR(100) COMMENT 'OTel: error.type',
    error_message TEXT COMMENT 'OTel: error.message',
    
    -- Performance
    duration_ms INT COMMENT 'Tool execution duration in milliseconds',
    
    -- Security & Audit
    is_approved BOOLEAN DEFAULT true COMMENT 'Whether the tool call was approved by policy',
    approval_reason VARCHAR(255) COMMENT 'Reason for approval or rejection',
    
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (session_id) REFERENCES ai_sessions(id) ON DELETE CASCADE
);

-- Prompt templates
CREATE TABLE prompt_templates (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    template TEXT NOT NULL,
    variables JSON DEFAULT NULL COMMENT 'Template variables schema',
    version INT DEFAULT 1,
    labels JSON DEFAULT NULL COMMENT 'Labels for categorization',
    is_active BOOLEAN DEFAULT true,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Indexes for ai_sessions
CREATE INDEX idx_ai_sessions_trace ON ai_sessions(trace_id);
CREATE INDEX idx_ai_sessions_span ON ai_sessions(span_id);
CREATE INDEX idx_ai_sessions_service ON ai_sessions(service_id);
CREATE INDEX idx_ai_sessions_model ON ai_sessions(model_id);
CREATE INDEX idx_ai_sessions_created ON ai_sessions(created_at DESC);
CREATE INDEX idx_ai_sessions_status ON ai_sessions(status);
CREATE INDEX idx_ai_sessions_gen_ai_system ON ai_sessions(gen_ai_system);
CREATE INDEX idx_ai_sessions_gen_ai_operation ON ai_sessions(gen_ai_operation_name);
CREATE INDEX idx_ai_sessions_user ON ai_sessions(user_id);
CREATE INDEX idx_ai_sessions_session_type ON ai_sessions(session_type);

-- Indexes for tool_calls
CREATE INDEX idx_tool_calls_session ON tool_calls(session_id);
CREATE INDEX idx_tool_calls_span ON tool_calls(span_id);
CREATE INDEX idx_tool_calls_name ON tool_calls(gen_ai_tool_name);
CREATE INDEX idx_tool_calls_type ON tool_calls(gen_ai_tool_type);
CREATE INDEX idx_tool_calls_status ON tool_calls(status);

-- Indexes for ai_models
CREATE INDEX idx_ai_models_provider_model ON ai_models(provider, model_id);
CREATE INDEX idx_ai_models_enabled ON ai_models(enabled);

-- Indexes for prompt_templates
CREATE INDEX idx_prompt_templates_name ON prompt_templates(name);
CREATE INDEX idx_prompt_templates_active ON prompt_templates(is_active);

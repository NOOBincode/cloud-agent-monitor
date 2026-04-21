-- 002_ai_observability.up.sql
-- AI Observability schema extension (PostgreSQL 17 version)
-- Aligned with OpenTelemetry GenAI Semantic Conventions v1.37+

-- AI models configuration
CREATE TABLE obs_platform.ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    config JSONB DEFAULT NULL,
    cost_per_input_token DECIMAL(20,10),
    cost_per_output_token DECIMAL(20,10),
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON COLUMN obs_platform.ai_models.provider IS 'OTel: gen_ai.system (openai, anthropic, vertex_ai, bedrock)';
COMMENT ON COLUMN obs_platform.ai_models.model_id IS 'OTel: gen_ai.request.model';

-- AI sessions tracking (aligned with OTel GenAI Semantic Conventions)
CREATE TABLE obs_platform.ai_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    trace_id VARCHAR(64) NOT NULL,
    span_id VARCHAR(64),
    parent_span_id VARCHAR(64),
    
    service_id UUID,
    model_id UUID,
    
    gen_ai_system VARCHAR(50),
    gen_ai_operation_name VARCHAR(50) NOT NULL,
    
    gen_ai_request_model VARCHAR(255),
    gen_ai_request_max_tokens INTEGER,
    gen_ai_request_temperature DECIMAL(3,2),
    gen_ai_request_top_p DECIMAL(3,2),
    gen_ai_request_presence_penalty DECIMAL(3,2),
    gen_ai_request_frequency_penalty DECIMAL(3,2),
    gen_ai_request_stop_sequences JSONB,
    
    gen_ai_response_model VARCHAR(255),
    gen_ai_response_finish_reasons JSONB,
    gen_ai_response_id VARCHAR(255),
    
    gen_ai_usage_input_tokens INTEGER,
    gen_ai_usage_output_tokens INTEGER,
    gen_ai_usage_total_tokens INTEGER,
    
    operation VARCHAR(50),
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    
    duration_ms INTEGER,
    ttft_ms INTEGER,
    tpot_ms INTEGER,
    
    status VARCHAR(20) NOT NULL,
    error_type VARCHAR(100),
    error_message TEXT,
    
    resource_attributes JSONB,
    
    user_id VARCHAR(255),
    session_type VARCHAR(50),
    metadata JSONB DEFAULT NULL,
    
    cost_usd DECIMAL(20,6),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_ai_sessions_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id),
    CONSTRAINT fk_ai_sessions_model FOREIGN KEY (model_id) REFERENCES obs_platform.ai_models(id)
);

COMMENT ON COLUMN obs_platform.ai_sessions.trace_id IS 'OTel: trace.id';
COMMENT ON COLUMN obs_platform.ai_sessions.span_id IS 'OTel: span.id';
COMMENT ON COLUMN obs_platform.ai_sessions.parent_span_id IS 'OTel: parent.span.id';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_system IS 'OTel: gen_ai.system (openai, anthropic, vertex_ai, bedrock, ollama)';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_operation_name IS 'OTel: gen_ai.operation.name (chat, completion, embeddings)';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_request_model IS 'OTel: gen_ai.request.model (requested model)';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_request_max_tokens IS 'OTel: gen_ai.request.max_tokens';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_response_model IS 'OTel: gen_ai.response.model (actual model used)';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_response_finish_reasons IS 'OTel: gen_ai.response.finish_reasons';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_usage_input_tokens IS 'OTel: gen_ai.usage.input_tokens';
COMMENT ON COLUMN obs_platform.ai_sessions.gen_ai_usage_output_tokens IS 'OTel: gen_ai.usage.output_tokens';
COMMENT ON COLUMN obs_platform.ai_sessions.status IS 'OTel: status.code (success, error, throttled)';
COMMENT ON COLUMN obs_platform.ai_sessions.resource_attributes IS 'OTel: resource.attributes (service.name, service.version, etc.)';

-- Tool calls recording (aligned with OTel GenAI Semantic Conventions)
CREATE TABLE obs_platform.tool_calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL,
    span_id VARCHAR(64) NOT NULL,
    parent_span_id VARCHAR(64),
    
    gen_ai_tool_name VARCHAR(255) NOT NULL,
    gen_ai_tool_type VARCHAR(50) NOT NULL,
    gen_ai_tool_description TEXT,
    gen_ai_tool_call_id VARCHAR(255),
    
    tool_name VARCHAR(255),
    tool_type VARCHAR(50),
    
    arguments JSONB DEFAULT NULL,
    result JSONB DEFAULT NULL,
    
    status VARCHAR(20) NOT NULL,
    error_type VARCHAR(100),
    error_message TEXT,
    
    duration_ms INTEGER,
    
    is_approved BOOLEAN DEFAULT true,
    approval_reason VARCHAR(255),
    
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT fk_tool_calls_session FOREIGN KEY (session_id) REFERENCES obs_platform.ai_sessions(id) ON DELETE CASCADE
);

COMMENT ON COLUMN obs_platform.tool_calls.span_id IS 'OTel: span.id';
COMMENT ON COLUMN obs_platform.tool_calls.parent_span_id IS 'OTel: parent.span.id (link to ai_sessions.span_id)';
COMMENT ON COLUMN obs_platform.tool_calls.gen_ai_tool_name IS 'OTel: gen_ai.tool.name';
COMMENT ON COLUMN obs_platform.tool_calls.gen_ai_tool_type IS 'OTel: gen_ai.tool.type (function, code_interpreter, retrieval)';
COMMENT ON COLUMN obs_platform.tool_calls.gen_ai_tool_description IS 'OTel: gen_ai.tool.description';
COMMENT ON COLUMN obs_platform.tool_calls.gen_ai_tool_call_id IS 'OTel: gen_ai.tool.call.id';
COMMENT ON COLUMN obs_platform.tool_calls.arguments IS 'OTel: gen_ai.tool.call.arguments';
COMMENT ON COLUMN obs_platform.tool_calls.result IS 'OTel: gen_ai.tool.result';
COMMENT ON COLUMN obs_platform.tool_calls.status IS 'OTel: status.code (success, error)';
COMMENT ON COLUMN obs_platform.tool_calls.is_approved IS 'Whether the tool call was approved by policy';

-- Prompt templates
CREATE TABLE obs_platform.prompt_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    template TEXT NOT NULL,
    variables JSONB DEFAULT NULL,
    version INTEGER DEFAULT 1,
    labels JSONB DEFAULT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON COLUMN obs_platform.prompt_templates.variables IS 'Template variables schema';
COMMENT ON COLUMN obs_platform.prompt_templates.labels IS 'Labels for categorization';

-- Indexes for ai_sessions
CREATE INDEX idx_ai_sessions_trace ON obs_platform.ai_sessions(trace_id);
CREATE INDEX idx_ai_sessions_span ON obs_platform.ai_sessions(span_id);
CREATE INDEX idx_ai_sessions_service ON obs_platform.ai_sessions(service_id);
CREATE INDEX idx_ai_sessions_model ON obs_platform.ai_sessions(model_id);
CREATE INDEX idx_ai_sessions_created ON obs_platform.ai_sessions(created_at DESC);
CREATE INDEX idx_ai_sessions_status ON obs_platform.ai_sessions(status);
CREATE INDEX idx_ai_sessions_gen_ai_system ON obs_platform.ai_sessions(gen_ai_system);
CREATE INDEX idx_ai_sessions_gen_ai_operation ON obs_platform.ai_sessions(gen_ai_operation_name);
CREATE INDEX idx_ai_sessions_user ON obs_platform.ai_sessions(user_id);
CREATE INDEX idx_ai_sessions_session_type ON obs_platform.ai_sessions(session_type);

-- Indexes for tool_calls
CREATE INDEX idx_tool_calls_session ON obs_platform.tool_calls(session_id);
CREATE INDEX idx_tool_calls_span ON obs_platform.tool_calls(span_id);
CREATE INDEX idx_tool_calls_name ON obs_platform.tool_calls(gen_ai_tool_name);
CREATE INDEX idx_tool_calls_type ON obs_platform.tool_calls(gen_ai_tool_type);
CREATE INDEX idx_tool_calls_status ON obs_platform.tool_calls(status);

-- Indexes for ai_models
CREATE INDEX idx_ai_models_provider_model ON obs_platform.ai_models(provider, model_id);
CREATE INDEX idx_ai_models_enabled ON obs_platform.ai_models(enabled);

-- Indexes for prompt_templates
CREATE INDEX idx_prompt_templates_name ON obs_platform.prompt_templates(name);
CREATE INDEX idx_prompt_templates_active ON obs_platform.prompt_templates(is_active);

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_ai_models_updated_at BEFORE UPDATE ON obs_platform.ai_models
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_prompt_templates_updated_at BEFORE UPDATE ON obs_platform.prompt_templates
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

-- 004_agent_coord.up.sql
-- Agent coordination schema (PostgreSQL 17 version)

-- Agents registry
CREATE TABLE obs_platform.agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    service_id UUID,
    version VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'offline',
    last_heartbeat TIMESTAMP,
    config_version INTEGER DEFAULT 1,
    config JSONB DEFAULT NULL,
    capabilities JSONB DEFAULT NULL,
    metadata JSONB DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_agents_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id)
);

-- Agent command queue
CREATE TABLE obs_platform.agent_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID,
    command VARCHAR(100) NOT NULL,
    params JSONB DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    sent_at TIMESTAMP,
    acked_at TIMESTAMP,
    completed_at TIMESTAMP,
    result JSONB DEFAULT NULL,
    CONSTRAINT fk_agent_commands_agent FOREIGN KEY (agent_id) REFERENCES obs_platform.agents(id) ON DELETE CASCADE
);

-- Agent events log
CREATE TABLE obs_platform.agent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB DEFAULT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_agent_events_agent FOREIGN KEY (agent_id) REFERENCES obs_platform.agents(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_agents_service ON obs_platform.agents(service_id);
CREATE INDEX idx_agents_status ON obs_platform.agents(status);
CREATE INDEX idx_agents_last_heartbeat ON obs_platform.agents(last_heartbeat DESC);
CREATE INDEX idx_agent_commands_agent ON obs_platform.agent_commands(agent_id);
CREATE INDEX idx_agent_commands_status ON obs_platform.agent_commands(status);
CREATE INDEX idx_agent_events_agent ON obs_platform.agent_events(agent_id);
CREATE INDEX idx_agent_events_type ON obs_platform.agent_events(event_type);
CREATE INDEX idx_agent_events_created ON obs_platform.agent_events(created_at DESC);

-- Trigger for auto-updating updated_at
CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON obs_platform.agents
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
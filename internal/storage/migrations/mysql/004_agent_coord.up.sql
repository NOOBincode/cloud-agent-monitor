-- 004_agent_coord.up.sql
-- Agent coordination schema (MySQL version)

-- Agents registry
USE obs_platform;
CREATE TABLE agents (
    id CHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    service_id CHAR(36),
    version VARCHAR(50),
    status VARCHAR(20) NOT NULL DEFAULT 'offline',
    last_heartbeat DATETIME,
    config_version INT DEFAULT 1,
    config JSON DEFAULT NULL,
    capabilities JSON DEFAULT NULL,
    metadata JSON DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (service_id) REFERENCES services(id)
);

-- Agent command queue
CREATE TABLE agent_commands (
    id CHAR(36) PRIMARY KEY,
    agent_id CHAR(36),
    command VARCHAR(100) NOT NULL,
    params JSON DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    sent_at DATETIME,
    acked_at DATETIME,
    completed_at DATETIME,
    result JSON DEFAULT NULL,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

-- Agent events log
CREATE TABLE agent_events (
    id CHAR(36) PRIMARY KEY,
    agent_id CHAR(36),
    event_type VARCHAR(100) NOT NULL,
    event_data JSON DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_agents_service ON agents(service_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_last_heartbeat ON agents(last_heartbeat DESC);
CREATE INDEX idx_agent_commands_agent ON agent_commands(agent_id);
CREATE INDEX idx_agent_commands_status ON agent_commands(status);
CREATE INDEX idx_agent_events_agent ON agent_events(agent_id);
CREATE INDEX idx_agent_events_type ON agent_events(event_type);
CREATE INDEX idx_agent_events_created ON agent_events(created_at DESC);

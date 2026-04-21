-- 011_topology.up.sql
-- Topology module database schema (PostgreSQL 17 version)

-- Service nodes table
CREATE TABLE IF NOT EXISTS obs_platform.topology_service_nodes (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    namespace VARCHAR(255) NOT NULL,
    environment VARCHAR(50) DEFAULT 'default',
    status VARCHAR(20) DEFAULT 'unknown',
    labels JSONB,
    request_rate DOUBLE PRECISION DEFAULT 0,
    error_rate DOUBLE PRECISION DEFAULT 0,
    latency_p99 DOUBLE PRECISION DEFAULT 0,
    latency_p95 DOUBLE PRECISION DEFAULT 0,
    latency_p50 DOUBLE PRECISION DEFAULT 0,
    pod_count INTEGER DEFAULT 0,
    ready_pods INTEGER DEFAULT 0,
    service_type VARCHAR(50),
    maintainer VARCHAR(255),
    team VARCHAR(255),
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    CONSTRAINT uk_name_namespace UNIQUE (name, namespace)
);

CREATE INDEX idx_topology_nodes_name ON obs_platform.topology_service_nodes(name);
CREATE INDEX idx_topology_nodes_namespace ON obs_platform.topology_service_nodes(namespace);
CREATE INDEX idx_topology_nodes_environment ON obs_platform.topology_service_nodes(environment);
CREATE INDEX idx_topology_nodes_status ON obs_platform.topology_service_nodes(status);

COMMENT ON TABLE obs_platform.topology_service_nodes IS 'Service topology nodes';

-- Call edges table
CREATE TABLE IF NOT EXISTS obs_platform.topology_call_edges (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL,
    target_id UUID NOT NULL,
    edge_type VARCHAR(50) DEFAULT 'http',
    is_direct BOOLEAN DEFAULT true,
    confidence DOUBLE PRECISION DEFAULT 1.0,
    protocol VARCHAR(50),
    method VARCHAR(50),
    request_rate DOUBLE PRECISION DEFAULT 0,
    error_rate DOUBLE PRECISION DEFAULT 0,
    latency_p99 DOUBLE PRECISION DEFAULT 0,
    target_instances JSONB,
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    CONSTRAINT uk_source_target UNIQUE (source_id, target_id)
);

CREATE INDEX idx_topology_edges_source ON obs_platform.topology_call_edges(source_id);
CREATE INDEX idx_topology_edges_target ON obs_platform.topology_call_edges(target_id);
CREATE INDEX idx_topology_edges_type ON obs_platform.topology_call_edges(edge_type);

COMMENT ON TABLE obs_platform.topology_call_edges IS 'Service call edges';

-- Network nodes table
CREATE TABLE IF NOT EXISTS obs_platform.topology_network_nodes (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    layer VARCHAR(50) NOT NULL,
    ip_address VARCHAR(50),
    cidr VARCHAR(50),
    ports JSONB,
    namespace VARCHAR(255),
    pod_name VARCHAR(255),
    node_name VARCHAR(255),
    zone VARCHAR(100),
    data_center VARCHAR(100),
    connections BIGINT DEFAULT 0,
    bytes_in BIGINT DEFAULT 0,
    bytes_out BIGINT DEFAULT 0,
    packet_loss DOUBLE PRECISION DEFAULT 0,
    latency DOUBLE PRECISION DEFAULT 0,
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
);

CREATE INDEX idx_topology_net_nodes_name ON obs_platform.topology_network_nodes(name);
CREATE INDEX idx_topology_net_nodes_type ON obs_platform.topology_network_nodes(type);
CREATE INDEX idx_topology_net_nodes_layer ON obs_platform.topology_network_nodes(layer);
CREATE INDEX idx_topology_net_nodes_ip ON obs_platform.topology_network_nodes(ip_address);
CREATE INDEX idx_topology_net_nodes_node ON obs_platform.topology_network_nodes(node_name);
CREATE INDEX idx_topology_net_nodes_zone ON obs_platform.topology_network_nodes(zone);

COMMENT ON TABLE obs_platform.topology_network_nodes IS 'Network topology nodes';

-- Network edges table
CREATE TABLE IF NOT EXISTS obs_platform.topology_network_edges (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id UUID NOT NULL,
    target_id UUID NOT NULL,
    source_ip VARCHAR(50),
    target_ip VARCHAR(50),
    source_port INTEGER,
    target_port INTEGER,
    protocol VARCHAR(20),
    bytes_sent BIGINT DEFAULT 0,
    bytes_received BIGINT DEFAULT 0,
    packets_sent BIGINT DEFAULT 0,
    packets_lost BIGINT DEFAULT 0,
    connection_count INTEGER DEFAULT 0,
    established INTEGER DEFAULT 0,
    time_wait INTEGER DEFAULT 0,
    close_wait INTEGER DEFAULT 0,
    updated_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    
    CONSTRAINT uk_net_source_target UNIQUE (source_id, target_id)
);

CREATE INDEX idx_topology_net_edges_source ON obs_platform.topology_network_edges(source_id);
CREATE INDEX idx_topology_net_edges_target ON obs_platform.topology_network_edges(target_id);

COMMENT ON TABLE obs_platform.topology_network_edges IS 'Network connection edges';

-- Topology changes table
CREATE TABLE IF NOT EXISTS obs_platform.topology_changes (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    change_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    entity_name VARCHAR(255),
    description TEXT,
    before_state JSONB,
    after_state JSONB
);

CREATE INDEX idx_topology_changes_timestamp ON obs_platform.topology_changes(timestamp);
CREATE INDEX idx_topology_changes_type ON obs_platform.topology_changes(change_type);
CREATE INDEX idx_topology_changes_entity_type ON obs_platform.topology_changes(entity_type);
CREATE INDEX idx_topology_changes_entity_id ON obs_platform.topology_changes(entity_id);

COMMENT ON TABLE obs_platform.topology_changes IS 'Topology change records';

-- Topology snapshots table
CREATE TABLE IF NOT EXISTS obs_platform.topology_snapshots (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    graph_type VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP(3) NOT NULL,
    hash VARCHAR(64),
    nodes TEXT,
    edges TEXT
);

CREATE INDEX idx_topology_snapshots_type_time ON obs_platform.topology_snapshots(graph_type, timestamp);
CREATE INDEX idx_topology_snapshots_timestamp ON obs_platform.topology_snapshots(timestamp);

COMMENT ON TABLE obs_platform.topology_snapshots IS 'Topology snapshots';

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_topology_service_nodes_updated_at BEFORE UPDATE ON obs_platform.topology_service_nodes
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_topology_call_edges_updated_at BEFORE UPDATE ON obs_platform.topology_call_edges
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_topology_network_nodes_updated_at BEFORE UPDATE ON obs_platform.topology_network_nodes
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_topology_network_edges_updated_at BEFORE UPDATE ON obs_platform.topology_network_edges
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
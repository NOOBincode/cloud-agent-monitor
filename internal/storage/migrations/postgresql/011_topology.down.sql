-- 011_topology.down.sql
-- Rollback topology schema

DROP TRIGGER IF EXISTS update_topology_network_edges_updated_at ON obs_platform.topology_network_edges;
DROP TRIGGER IF EXISTS update_topology_network_nodes_updated_at ON obs_platform.topology_network_nodes;
DROP TRIGGER IF EXISTS update_topology_call_edges_updated_at ON obs_platform.topology_call_edges;
DROP TRIGGER IF EXISTS update_topology_service_nodes_updated_at ON obs_platform.topology_service_nodes;
DROP TABLE IF EXISTS obs_platform.topology_snapshots;
DROP TABLE IF EXISTS obs_platform.topology_changes;
DROP TABLE IF EXISTS obs_platform.topology_network_edges;
DROP TABLE IF EXISTS obs_platform.topology_network_nodes;
DROP TABLE IF EXISTS obs_platform.topology_call_edges;
DROP TABLE IF EXISTS obs_platform.topology_service_nodes;
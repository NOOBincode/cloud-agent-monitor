-- 011_topology.down.sql
-- 拓扑模块数据库表结构回滚

DROP TABLE IF EXISTS `topology_snapshots`;
DROP TABLE IF EXISTS `topology_changes`;
DROP TABLE IF EXISTS `topology_network_edges`;
DROP TABLE IF EXISTS `topology_network_nodes`;
DROP TABLE IF EXISTS `topology_call_edges`;
DROP TABLE IF EXISTS `topology_service_nodes`;

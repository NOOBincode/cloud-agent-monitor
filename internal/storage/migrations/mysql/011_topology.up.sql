-- 011_topology.up.sql
-- 拓扑模块数据库表结构

-- 服务节点表
CREATE TABLE IF NOT EXISTS `topology_service_nodes` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `namespace` VARCHAR(255) NOT NULL,
    `environment` VARCHAR(50) DEFAULT 'default',
    `status` VARCHAR(20) DEFAULT 'unknown',
    `labels` JSON,
    `request_rate` DOUBLE DEFAULT 0,
    `error_rate` DOUBLE DEFAULT 0,
    `latency_p99` DOUBLE DEFAULT 0,
    `latency_p95` DOUBLE DEFAULT 0,
    `latency_p50` DOUBLE DEFAULT 0,
    `pod_count` INT DEFAULT 0,
    `ready_pods` INT DEFAULT 0,
    `service_type` VARCHAR(50),
    `maintainer` VARCHAR(255),
    `team` VARCHAR(255),
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    
    INDEX `idx_name` (`name`),
    INDEX `idx_namespace` (`namespace`),
    INDEX `idx_environment` (`environment`),
    INDEX `idx_status` (`status`),
    UNIQUE INDEX `idx_name_namespace` (`name`, `namespace`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='服务拓扑节点';

-- 调用边表
CREATE TABLE IF NOT EXISTS `topology_call_edges` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `source_id` CHAR(36) NOT NULL,
    `target_id` CHAR(36) NOT NULL,
    `edge_type` VARCHAR(50) DEFAULT 'http',
    `is_direct` TINYINT(1) DEFAULT 1,
    `confidence` DOUBLE DEFAULT 1.0,
    `protocol` VARCHAR(50),
    `method` VARCHAR(50),
    `request_rate` DOUBLE DEFAULT 0,
    `error_rate` DOUBLE DEFAULT 0,
    `latency_p99` DOUBLE DEFAULT 0,
    `target_instances` JSON,
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    
    INDEX `idx_source_id` (`source_id`),
    INDEX `idx_target_id` (`target_id`),
    INDEX `idx_edge_type` (`edge_type`),
    UNIQUE INDEX `idx_source_target` (`source_id`, `target_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='服务调用边';

-- 网络节点表
CREATE TABLE IF NOT EXISTS `topology_network_nodes` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `name` VARCHAR(255) NOT NULL,
    `type` VARCHAR(50) NOT NULL,
    `layer` VARCHAR(50) NOT NULL,
    `ip_address` VARCHAR(50),
    `cidr` VARCHAR(50),
    `ports` JSON,
    `namespace` VARCHAR(255),
    `pod_name` VARCHAR(255),
    `node_name` VARCHAR(255),
    `zone` VARCHAR(100),
    `data_center` VARCHAR(100),
    `connections` BIGINT DEFAULT 0,
    `bytes_in` BIGINT DEFAULT 0,
    `bytes_out` BIGINT DEFAULT 0,
    `packet_loss` DOUBLE DEFAULT 0,
    `latency` DOUBLE DEFAULT 0,
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    
    INDEX `idx_name` (`name`),
    INDEX `idx_type` (`type`),
    INDEX `idx_layer` (`layer`),
    INDEX `idx_ip_address` (`ip_address`),
    INDEX `idx_node_name` (`node_name`),
    INDEX `idx_zone` (`zone`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='网络拓扑节点';

-- 网络边表
CREATE TABLE IF NOT EXISTS `topology_network_edges` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `source_id` CHAR(36) NOT NULL,
    `target_id` CHAR(36) NOT NULL,
    `source_ip` VARCHAR(50),
    `target_ip` VARCHAR(50),
    `source_port` INT,
    `target_port` INT,
    `protocol` VARCHAR(20),
    `bytes_sent` BIGINT DEFAULT 0,
    `bytes_received` BIGINT DEFAULT 0,
    `packets_sent` BIGINT DEFAULT 0,
    `packets_lost` BIGINT DEFAULT 0,
    `connection_count` INT DEFAULT 0,
    `established` INT DEFAULT 0,
    `time_wait` INT DEFAULT 0,
    `close_wait` INT DEFAULT 0,
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    
    INDEX `idx_source_id` (`source_id`),
    INDEX `idx_target_id` (`target_id`),
    UNIQUE INDEX `idx_source_target` (`source_id`, `target_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='网络连接边';

-- 拓扑变化记录表
CREATE TABLE IF NOT EXISTS `topology_changes` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `timestamp` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `change_type` VARCHAR(50) NOT NULL,
    `entity_type` VARCHAR(50) NOT NULL,
    `entity_id` CHAR(36) NOT NULL,
    `entity_name` VARCHAR(255),
    `description` TEXT,
    `before_state` JSON,
    `after_state` JSON,
    
    INDEX `idx_timestamp` (`timestamp`),
    INDEX `idx_change_type` (`change_type`),
    INDEX `idx_entity_type` (`entity_type`),
    INDEX `idx_entity_id` (`entity_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='拓扑变化记录';

-- 拓扑快照表（用于历史查询）
CREATE TABLE IF NOT EXISTS `topology_snapshots` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `graph_type` VARCHAR(50) NOT NULL,
    `timestamp` DATETIME(3) NOT NULL,
    `hash` VARCHAR(64),
    `nodes` LONGTEXT,
    `edges` LONGTEXT,
    
    INDEX `idx_graph_type_timestamp` (`graph_type`, `timestamp`),
    INDEX `idx_timestamp` (`timestamp`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='拓扑快照';

-- 创建分区表（按时间分区，保留最近30天）
-- 注意：MySQL 8.0 支持原生分区，如果是旧版本需要手动管理

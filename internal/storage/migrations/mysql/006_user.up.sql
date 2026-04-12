-- 006_user.up.sql
-- User authentication and authorization schema (MySQL version)

USE obs_platform;

-- 用户表
CREATE TABLE IF NOT EXISTS `users` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `username` VARCHAR(255) NOT NULL,
    `email` VARCHAR(255) DEFAULT NULL,
    `password_hash` VARCHAR(255) DEFAULT NULL,
    `display_name` VARCHAR(255) DEFAULT NULL,
    `is_active` TINYINT(1) DEFAULT 1,
    `tenant_id` CHAR(36) DEFAULT NULL,
    `password_reset_token` VARCHAR(255) DEFAULT NULL,
    `password_reset_expires` DATETIME(3) DEFAULT NULL,
    `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    `deleted_at` DATETIME(3) DEFAULT NULL,
    UNIQUE INDEX `idx_users_username` (`username`),
    UNIQUE INDEX `idx_users_email` (`email`),
    INDEX `idx_users_is_active` (`is_active`),
    INDEX `idx_users_tenant_id` (`tenant_id`),
    INDEX `idx_users_password_reset_token` (`password_reset_token`),
    INDEX `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 角色表
CREATE TABLE IF NOT EXISTS `roles` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `name` VARCHAR(100) NOT NULL,
    `description` TEXT,
    `is_system` TINYINT(1) DEFAULT 0,
    `permissions` JSON,
    `tenant_id` CHAR(36) DEFAULT NULL,
    `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    `deleted_at` DATETIME(3) DEFAULT NULL,
    UNIQUE INDEX `idx_roles_name` (`name`),
    INDEX `idx_roles_is_system` (`is_system`),
    INDEX `idx_roles_tenant_id` (`tenant_id`),
    INDEX `idx_roles_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 用户角色关联表
CREATE TABLE IF NOT EXISTS `user_roles` (
    `user_id` CHAR(36) NOT NULL,
    `role_id` CHAR(36) NOT NULL,
    `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`user_id`, `role_id`),
    INDEX `idx_user_roles_role_id` (`role_id`),
    CONSTRAINT `fk_user_roles_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT `fk_user_roles_role` FOREIGN KEY (`role_id`) REFERENCES `roles`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 登录日志表
CREATE TABLE IF NOT EXISTS `login_logs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) DEFAULT NULL,
    `username` VARCHAR(255) DEFAULT NULL,
    `ip_address` VARCHAR(45) DEFAULT NULL,
    `user_agent` VARCHAR(500) DEFAULT NULL,
    `success` TINYINT(1) DEFAULT 0,
    `reason` VARCHAR(255) DEFAULT NULL,
    `created_at` DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),
    INDEX `idx_login_logs_user_id` (`user_id`),
    INDEX `idx_login_logs_username` (`username`),
    INDEX `idx_login_logs_success` (`success`),
    INDEX `idx_login_logs_created_at` (`created_at`),
    CONSTRAINT `fk_login_logs_user` FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 初始化系统角色
INSERT INTO `roles` (`id`, `name`, `description`, `is_system`, `permissions`, `created_at`, `updated_at`) VALUES
(UUID(), 'admin', '系统管理员', 1, '{"service_read":true,"service_write":true,"service_delete":true,"config_read":true,"config_write":true,"audit_read":true,"user_read":true,"user_write":true,"admin":true}', NOW(3), NOW(3)),
(UUID(), 'operator', '运维人员', 1, '{"service_read":true,"service_write":true,"config_read":true,"audit_read":true,"admin":false}', NOW(3), NOW(3)),
(UUID(), 'viewer', '只读用户', 1, '{"service_read":true,"config_read":true,"admin":false}', NOW(3), NOW(3));

-- 初始化管理员用户 (密码: Admin@123)
INSERT INTO `users` (`id`, `username`, `email`, `password_hash`, `display_name`, `is_active`, `created_at`, `updated_at`) VALUES
(UUID(), 'admin', 'admin@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZRGdjGj/n3.iW8jY9D6aV3xV9xKCe', '系统管理员', 1, NOW(3), NOW(3));

-- 关联管理员角色
INSERT INTO `user_roles` (`user_id`, `role_id`, `created_at`) 
SELECT u.id, r.id, NOW(3) FROM users u, roles r WHERE u.username = 'admin' AND r.name = 'admin';

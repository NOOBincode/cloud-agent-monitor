-- 006_user.up.sql
-- User authentication and authorization schema (PostgreSQL 17 version)

-- Users table
CREATE TABLE IF NOT EXISTS obs_platform.users (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL,
    email VARCHAR(255) DEFAULT NULL,
    password_hash VARCHAR(255) DEFAULT NULL,
    display_name VARCHAR(255) DEFAULT NULL,
    is_active BOOLEAN DEFAULT true,
    tenant_id UUID DEFAULT NULL,
    password_reset_token VARCHAR(255) DEFAULT NULL,
    password_reset_expires TIMESTAMP(3) DEFAULT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at TIMESTAMP(3) DEFAULT NULL,
    CONSTRAINT uk_users_username UNIQUE (username),
    CONSTRAINT uk_users_email UNIQUE (email)
);

CREATE INDEX idx_users_is_active ON obs_platform.users(is_active);
CREATE INDEX idx_users_tenant_id ON obs_platform.users(tenant_id);
CREATE INDEX idx_users_password_reset_token ON obs_platform.users(password_reset_token);
CREATE INDEX idx_users_deleted_at ON obs_platform.users(deleted_at);

-- Roles table
CREATE TABLE IF NOT EXISTS obs_platform.roles (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT false,
    permissions JSONB,
    tenant_id UUID DEFAULT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    updated_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at TIMESTAMP(3) DEFAULT NULL,
    CONSTRAINT uk_roles_name UNIQUE (name)
);

CREATE INDEX idx_roles_is_system ON obs_platform.roles(is_system);
CREATE INDEX idx_roles_tenant_id ON obs_platform.roles(tenant_id);
CREATE INDEX idx_roles_deleted_at ON obs_platform.roles(deleted_at);

-- User roles association table
CREATE TABLE IF NOT EXISTS obs_platform.user_roles (
    user_id UUID NOT NULL,
    role_id UUID NOT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (user_id, role_id),
    CONSTRAINT fk_user_roles_user FOREIGN KEY (user_id) REFERENCES obs_platform.users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_roles_role FOREIGN KEY (role_id) REFERENCES obs_platform.roles(id) ON DELETE CASCADE
);

CREATE INDEX idx_user_roles_role_id ON obs_platform.user_roles(role_id);

-- Login logs table
CREATE TABLE IF NOT EXISTS obs_platform.login_logs (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID DEFAULT NULL,
    username VARCHAR(255) DEFAULT NULL,
    ip_address VARCHAR(45) DEFAULT NULL,
    user_agent VARCHAR(500) DEFAULT NULL,
    success BOOLEAN DEFAULT false,
    reason VARCHAR(255) DEFAULT NULL,
    created_at TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3),
    CONSTRAINT fk_login_logs_user FOREIGN KEY (user_id) REFERENCES obs_platform.users(id) ON DELETE SET NULL
);

CREATE INDEX idx_login_logs_user_id ON obs_platform.login_logs(user_id);
CREATE INDEX idx_login_logs_username ON obs_platform.login_logs(username);
CREATE INDEX idx_login_logs_success ON obs_platform.login_logs(success);
CREATE INDEX idx_login_logs_created_at ON obs_platform.login_logs(created_at);

-- Initialize system roles
INSERT INTO obs_platform.roles (id, name, description, is_system, permissions, created_at, updated_at) VALUES
(gen_random_uuid(), 'admin', 'System Administrator', true, '{"service_read":true,"service_write":true,"service_delete":true,"config_read":true,"config_write":true,"audit_read":true,"user_read":true,"user_write":true,"admin":true}', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
(gen_random_uuid(), 'operator', 'Operator', true, '{"service_read":true,"service_write":true,"config_read":true,"audit_read":true,"admin":false}', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3)),
(gen_random_uuid(), 'viewer', 'Read-only User', true, '{"service_read":true,"config_read":true,"admin":false}', CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3));

-- Initialize admin user (password: Admin@123)
INSERT INTO obs_platform.users (id, username, email, password_hash, display_name, is_active, created_at, updated_at) VALUES
(gen_random_uuid(), 'admin', 'admin@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZRGdjGj/n3.iW8jY9D6aV3xV9xKCe', 'System Administrator', true, CURRENT_TIMESTAMP(3), CURRENT_TIMESTAMP(3));

-- Associate admin role
INSERT INTO obs_platform.user_roles (user_id, role_id, created_at) 
SELECT u.id, r.id, CURRENT_TIMESTAMP(3) FROM obs_platform.users u, obs_platform.roles r WHERE u.username = 'admin' AND r.name = 'admin';

-- Triggers for auto-updating updated_at
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON obs_platform.users
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();

CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON obs_platform.roles
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
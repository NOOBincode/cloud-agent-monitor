-- Add new fields to services table
ALTER TABLE services ADD COLUMN endpoint VARCHAR(500) DEFAULT '' AFTER labels;
ALTER TABLE services ADD COLUMN openapi_spec LONGTEXT AFTER endpoint;
ALTER TABLE services ADD COLUMN maintainer VARCHAR(255) DEFAULT '' AFTER health_check_details;
ALTER TABLE services ADD COLUMN team VARCHAR(255) DEFAULT '' AFTER maintainer;
ALTER TABLE services ADD COLUMN documentation_url VARCHAR(500) DEFAULT '' AFTER team;
ALTER TABLE services ADD COLUMN repository_url VARCHAR(500) DEFAULT '' AFTER documentation_url;

-- Create service_dependencies table
CREATE TABLE IF NOT EXISTS service_dependencies (
    id CHAR(36) NOT NULL PRIMARY KEY,
    service_id CHAR(36) NOT NULL,
    depends_on_id CHAR(36) NOT NULL,
    relation_type VARCHAR(50) DEFAULT 'depends_on',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    UNIQUE KEY idx_service_dep (service_id, depends_on_id),
    INDEX idx_service_id (service_id),
    INDEX idx_depends_on_id (depends_on_id),
    INDEX idx_deleted_at (deleted_at),
    CONSTRAINT fk_service_dep_service FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE,
    CONSTRAINT fk_service_dep_depends_on FOREIGN KEY (depends_on_id) REFERENCES services(id) ON DELETE CASCADE
);
-- Create service_dependencies table
CREATE TABLE IF NOT EXISTS service_dependencies (
    id CHAR(36) NOT NULL PRIMARY KEY,
    service_id CHAR(36) NOT NULL,
    depends_on_id CHAR(36) NOT NULL,
    relation_type VARCHAR(50) DEFAULT 'depends_on',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    UNIQUE KEY idx_service_dep (service_id, depends_on_id),
    INDEX idx_service_id (service_id),
    INDEX idx_depends_on_id (depends_on_id),
    INDEX idx_deleted_at (deleted_at),
    CONSTRAINT fk_service_dep_service FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE CASCADE,
    CONSTRAINT fk_service_dep_depends_on FOREIGN KEY (depends_on_id) REFERENCES services(id) ON DELETE CASCADE
);ALTER TABLE services ADD COLUMN repository_url VARCHAR(500) DEFAULT '' AFTER documentation_url;ALTER TABLE services ADD COLUMN documentation_url VARCHAR(500) DEFAULT '' AFTER team;ALTER TABLE services ADD COLUMN team VARCHAR(255) DEFAULT '' AFTER maintainer;ALTER TABLE services ADD COLUMN maintainer VARCHAR(255) DEFAULT '' AFTER health_check_details;ALTER TABLE services ADD COLUMN openapi_spec LONGTEXT AFTER endpoint;-- Add new fields to services table
ALTER TABLE services ADD COLUMN endpoint VARCHAR(500) DEFAULT '' AFTER labels;-- Add new fields to services table
ALTER TABLE services ADD COLUMN endpoint VARCHAR(500) DEFAULT '' AFTER labels;
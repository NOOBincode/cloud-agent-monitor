-- 009_service_catalog_extension.up.sql
-- Service catalog extension (PostgreSQL 17 version)

-- Add new fields to services table
ALTER TABLE obs_platform.services ADD COLUMN endpoint VARCHAR(500) DEFAULT '';
ALTER TABLE obs_platform.services ADD COLUMN openapi_spec TEXT;
ALTER TABLE obs_platform.services ADD COLUMN maintainer VARCHAR(255) DEFAULT '';
ALTER TABLE obs_platform.services ADD COLUMN team VARCHAR(255) DEFAULT '';
ALTER TABLE obs_platform.services ADD COLUMN documentation_url VARCHAR(500) DEFAULT '';
ALTER TABLE obs_platform.services ADD COLUMN repository_url VARCHAR(500) DEFAULT '';

-- Create service_dependencies table
CREATE TABLE IF NOT EXISTS obs_platform.service_dependencies (
    id UUID NOT NULL PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL,
    depends_on_id UUID NOT NULL,
    relation_type VARCHAR(50) DEFAULT 'depends_on',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    CONSTRAINT uk_service_dep UNIQUE (service_id, depends_on_id),
    CONSTRAINT fk_service_dep_service FOREIGN KEY (service_id) REFERENCES obs_platform.services(id) ON DELETE CASCADE,
    CONSTRAINT fk_service_dep_depends_on FOREIGN KEY (depends_on_id) REFERENCES obs_platform.services(id) ON DELETE CASCADE
);

CREATE INDEX idx_service_dep_service ON obs_platform.service_dependencies(service_id);
CREATE INDEX idx_service_dep_depends ON obs_platform.service_dependencies(depends_on_id);
CREATE INDEX idx_service_dep_deleted ON obs_platform.service_dependencies(deleted_at);

-- Trigger for auto-updating updated_at
CREATE TRIGGER update_service_dependencies_updated_at BEFORE UPDATE ON obs_platform.service_dependencies
    FOR EACH ROW EXECUTE FUNCTION obs_platform.update_updated_at_column();
-- 009_service_catalog_extension.down.sql
-- Rollback service catalog extension

DROP TRIGGER IF EXISTS update_service_dependencies_updated_at ON obs_platform.service_dependencies;
DROP TABLE IF EXISTS obs_platform.service_dependencies;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS repository_url;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS documentation_url;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS team;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS maintainer;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS openapi_spec;
ALTER TABLE obs_platform.services DROP COLUMN IF EXISTS endpoint;
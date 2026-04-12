-- Drop service_dependencies table
DROP TABLE IF EXISTS service_dependencies;

-- Remove new fields from services table
ALTER TABLE services DROP COLUMN repository_url;
ALTER TABLE services DROP COLUMN documentation_url;
ALTER TABLE services DROP COLUMN team;
ALTER TABLE services DROP COLUMN maintainer;
ALTER TABLE services DROP COLUMN openapi_spec;
ALTER TABLE services DROP COLUMN endpoint;

-- 005_ai_infra_extension.down.sql
-- Rollback AI infrastructure extension

DROP TRIGGER IF EXISTS update_cost_budgets_updated_at ON obs_platform.cost_budgets;
DROP TRIGGER IF EXISTS update_queue_jobs_updated_at ON obs_platform.queue_jobs;
DROP TRIGGER IF EXISTS update_model_versions_updated_at ON obs_platform.model_versions;
DROP TRIGGER IF EXISTS update_inference_services_updated_at ON obs_platform.inference_services;
DROP TRIGGER IF EXISTS update_gpu_nodes_updated_at ON obs_platform.gpu_nodes;

DROP TABLE IF EXISTS obs_platform.tool_execution_logs;
DROP TABLE IF EXISTS obs_platform.prompt_audit_logs;
DROP TABLE IF EXISTS obs_platform.budget_alerts;
DROP TABLE IF EXISTS obs_platform.cost_budgets;
DROP TABLE IF EXISTS obs_platform.resource_allocations;
DROP TABLE IF EXISTS obs_platform.queue_jobs;
DROP TABLE IF EXISTS obs_platform.model_versions;
DROP TABLE IF EXISTS obs_platform.inference_requests;
DROP TABLE IF EXISTS obs_platform.inference_services;
DROP TABLE IF EXISTS obs_platform.gpu_alerts;
DROP TABLE IF EXISTS obs_platform.gpu_metrics;
DROP TABLE IF EXISTS obs_platform.gpu_nodes;
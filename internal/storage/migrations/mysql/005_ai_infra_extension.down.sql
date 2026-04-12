-- 005_ai_infra_extension.down.sql
-- Rollback AI Infrastructure extension schema (MySQL version)

-- Module 5: Security & Audit Enhancement
USE obs_platform;
DROP TABLE IF EXISTS tool_execution_logs;
DROP TABLE IF EXISTS prompt_audit_logs;

-- Module 4: Cost Governance
DROP TABLE IF EXISTS budget_alerts;
DROP TABLE IF EXISTS cost_budgets;

-- Module 3: Queue & Scheduling
DROP TABLE IF EXISTS resource_allocations;
DROP TABLE IF EXISTS queue_jobs;

-- Module 2: Inference Service
DROP TABLE IF EXISTS model_versions;
DROP TABLE IF EXISTS inference_requests;
DROP TABLE IF EXISTS inference_services;

-- Module 1: GPU Observability
DROP TABLE IF EXISTS gpu_alerts;
DROP TABLE IF EXISTS gpu_metrics;
DROP TABLE IF EXISTS gpu_nodes;

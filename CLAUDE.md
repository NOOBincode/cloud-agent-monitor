# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
# Full workflow
make all                    # deps -> lint -> test -> build

# Build
make build                  # builds bin/platform-api, bin/worker, bin/agent
go build -o bin/platform-api ./cmd/platform-api

# Run
make run                    # go run ./cmd/platform-api
CONFIG_PATH=configs/config.yaml go run ./cmd/platform-api

# Tests
make test                   # all tests with race detection
make test-unit              # -short ./internal/... only
make test-integration       # -tags=integration (requires Docker)
make test-cover             # coverage report -> coverage.html
make test-bench             # benchmarks for topology/aiinfra

# Single test
go test -v -race -run TestFunctionName ./path/to/package/

# Lint & Format
make lint                   # golangci-lint (28 linters, see .golangci.yml)
make lint-fix               # golangci-lint --fix
make fmt                    # go fmt + goimports (local prefix: cloud-agent-monitor)
make vet                    # go vet
make security               # gosec scan

# Wire (dependency injection code generation)
make wire                   # regenerate wire_gen.go

# Docker
cd deploy/compose && docker compose up -d          # infra only (PG, Redis, Prometheus)
cd deploy/compose && docker compose --profile app up -d --build  # full stack
```

## Architecture

### Module Organization

The project is a cloud-native monitoring platform. Entry points in `cmd/`: `platform-api` (primary HTTP server), `worker` (async tasks), `agent` (edge agent bootstrap). Only `platform-api` has full implementation.

Business logic lives in `internal/` as DDD bounded contexts. Each module follows the 4-layer pattern:

```
internal/<module>/
  domain/        -- models, interfaces, value objects (no infra imports)
  application/   -- service orchestration (depends on domain interfaces)
  infrastructure/ -- implementations (persistence, external APIs, caches)
  interfaces/    -- HTTP handlers + route registration
```

Active modules with full DDD: `alerting`, `topology`, `slo`. The `agent` module has partial implementation (eino tool/pool subsystem complete; MCP Server, agent_host, subagent, context, memory, workflow, task, RAG are stubs). `auth` and `user` are flat packages without DDD decomposition.

### Cross-Module Dependencies

```
Config (pkg/config)
  -> PostgreSQL (internal/storage) -> Repository interfaces for all modules
  -> Redis -> Topology cache
  -> Prometheus Client -> Topology/SLO/Health backends
  -> Alertmanager Client -> Alerting service
  -> Agent Module -> wraps AlertService, SLOService, TopologyService as ToolProviders
```

`internal/storage` is a shared infrastructure layer providing GORM-based repository implementations for all bounded contexts. Domain modules define their own repository interfaces in `domain/`; `storage` implements them.

`pkg/infra` provides two shared services: `Cache` (freecache local cache) and `Queue` (asynq Redis-backed task queue), used by alerting, topology, and SLO.

### Wire Dependency Injection

All DI is wired in `cmd/platform-api/wire.go` (~35 provider functions). After modifying providers:
1. Update `wire.go`
2. Run `make wire` to regenerate `wire_gen.go`
3. New modules must register routes in `internal/platform/routes.go` and providers in `wire.go`

### Agent Module (Eino Framework)

The agent module uses CloudWeGo's `eino` for LLM tool-calling orchestration. Directory structure:

```
internal/agent/
  domain/           -- domain models (tool, tool_pool, domain_agent, workflow, skill, context, task, session, memory, trace)
  application/      -- service orchestration (domain_agent_service, skill_service, context_service, task_service, observability_service, session_service, rag_service)
  infrastructure/
    mcp/            -- MCP Server toolsets (server, alerting_server, topology_server, slo_server, gpu_server, inference_server, cost_server)
    agent_host/     -- Domain Agent Host (host, router, builder, invoke_tool + builtin/ agent configs)
    subagent/       -- Sub-agent execution (builder, executor; dual-layer memory: Working + Semantic)
    eino/           -- Eino tool integration (registry, pool, pools_builtin, node, llm, fat/thin tools)
    workflow/       -- Scenario workflows (builder, router, registry + builtin/ workflow definitions)
    auth/           -- RBAC permission system (permission, provider)
    middleware/      -- HTTP middleware (eino_auth Bearer token)
    context/        -- Context management (manager, postgres_store, pgvector_store, keyword_compressor, summary_compressor, cleaner)
    task/           -- Task processing (decomposer, executor, verifier, retry)
    memory/         -- Two-layer memory (working, semantic)
    rag/            -- RAG subsystem (pgvector_store, embedder, retriever, loader, chunker)
    otel/           -- OTel GenAI observability (tracer, eino_interceptor, span_processor, metrics)
    persistence/    -- PostgreSQL repositories (context, task, pool, trace, thought)
  interfaces/http/  -- Agent API handlers + route registration
```

Key components:

- **ToolRegistry**: central map of `ReadOnlyTool` instances, each wrapped with `AuthzToolWrapper` (auth + permission check) and `TracedToolDecorator` (OTel spans)
- **PoolRegistry**: groups tools into intent-based pools with budget limits. `SelectTools(ctx, intent)` routes via `IntentRouter` (keyword matching) -> pool -> authorized tool subset
- **ToolProvider**: interface implemented by AlertingToolProvider, SLOToolProvider, TopologyToolProvider. Each creates a "fat tool" (dispatches by `action` field) plus multiple "thin tools" (inject fixed action, delegate to fat tool via `FatToolDelegator`)
- **MCP Server** (infrastructure/mcp/): per-domain MCP Server exposing thin tools via stdio, replacing fat tool inline registration
- **Agent Host** (infrastructure/agent_host/): DomainAgentHost discovering MCP Servers, building eino Graph with system prompt + budget config; builtin/ defines domain agent configs (GPU, K8s, alerting, SLO, diagnosis, coordinator)
- **Subagent** (infrastructure/subagent/): sub-agent builder/executor with dual-layer memory (Working + Semantic)
- 4 active pools: alerting (priority 9, 6 tools), topology (8, 10 tools), slo (7, 5 tools), general (1, 7 tools)
- LLM: OpenAI-compatible `ToolCallingChatModel` (currently DeepSeek via `config.LLM`)

### HTTP API

Gin server assembled by Wire. Middleware: `gin.Logger -> gin.Recovery`, protected routes add `RequireAPIKey -> CasbinMiddleware`. Route groups: `/healthz`, `/api/v1/auth/` (public), `/api/v1/{services,alerts,slos,topology,agent}/` (protected).

### Configuration

`pkg/config/config.go` struct maps 1:1 to `configs/config.yaml`. Viper with `PLATFORM_` env prefix (dots/hyphens -> underscores). Override config path with `CONFIG_PATH` env var. Validation: JWT secret >= 32 chars, max_open >= max_idle.

### Database

PostgreSQL with GORM. All tables in `obs_platform` schema. 12 migrations in `internal/storage/migrations/postgresql/` (001-012, each has `.up.sql` + `.down.sql`). Migrations applied manually or via scripts (not auto-run on startup).

### Observability

OTel instrumentation in `internal/agent/infrastructure/otel/`: typed span creation (`tracer.go`), metrics counters/histograms (`metrics.go`), eino lifecycle hooks (`eino_interceptor.go`). Semantic conventions defined in `internal/aiinfra/domain/semconv.go` following OTel GenAI spec.

## Testing Standards

- Table-driven tests mandatory (see `docs/testing-standards.md`)
- Mock interfaces, not concrete types
- `-race` flag required on all test runs
- Coverage targets: domain >=90%, application >=80%, infrastructure >=75%
- Integration tests require Docker (testcontainers-go)
- golangci-lint excludes dupl/gosec/goconst/gocyclo for test files
// Package eino provides the Agent tool infrastructure built on the eino framework.
// It implements a fat/thin tool delegation pattern where lightweight "thin" tools
// forward requests to a single "fat" tool with a fixed action field, enabling
// fine-grained tool selection for LLM agents while keeping business logic centralized.
//
// Core components:
//   - IntentRouter: keyword-based intent matching to select the best tool pool
//   - PoolRegistry: central orchestrator managing tool pools, providers, and budgets
//   - ToolRegistry: tool instance registry with authorization wrappers
//   - FatToolDelegator: injects the "action" field into JSON args and delegates to the fat tool
//   - TracedToolDecorator: wraps tool calls with OpenTelemetry spans
package eino

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud-agent-monitor/internal/agent/domain"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// IntentRouter matches natural language intents to tool pools using a keyword index.
// It supports both exact keyword matching and substring matching, with priority-based
// selection when multiple pools match. Falls back to the "general" pool or the first
// available pool when no keywords match.
type IntentRouter struct {
	mu           sync.RWMutex
	pools        map[string]*domain.ToolPool
	keywordIndex map[string][]string
}

// NewIntentRouter creates a new IntentRouter with empty keyword index.
func NewIntentRouter() *IntentRouter {
	return &IntentRouter{
		pools:        make(map[string]*domain.ToolPool),
		keywordIndex: make(map[string][]string),
	}
}

// AddPool indexes a tool pool by its keywords for intent routing.
func (r *IntentRouter) AddPool(pool *domain.ToolPool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pools[pool.ID] = pool
	for _, kw := range pool.Keywords {
		kwLower := strings.ToLower(kw)
		r.keywordIndex[kwLower] = append(r.keywordIndex[kwLower], pool.ID)
	}
}

// RemovePool removes a pool and cleans up all its keyword mappings.
func (r *IntentRouter) RemovePool(poolID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	pool, ok := r.pools[poolID]
	if !ok {
		return
	}

	for _, kw := range pool.Keywords {
		kwLower := strings.ToLower(kw)
		poolIDs := r.keywordIndex[kwLower]
		filtered := make([]string, 0, len(poolIDs))
		for _, id := range poolIDs {
			if id != poolID {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) == 0 {
			delete(r.keywordIndex, kwLower)
		} else {
			r.keywordIndex[kwLower] = filtered
		}
	}
	delete(r.pools, poolID)
}

// Route resolves a natural language intent to the best matching ToolPool.
// It first tries exact keyword matching, then substring matching, and finally
// falls back to the "general" pool or the first available pool.
func (r *IntentRouter) Route(intent string) *domain.ToolPool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	intentLower := strings.ToLower(intent)

	type scored struct {
		pool     *domain.ToolPool
		priority int
	}

	var candidates []scored
	seen := make(map[string]bool)

	words := tokenize(intentLower)
	for _, word := range words {
		poolIDs, ok := r.keywordIndex[word]
		if !ok {
			continue
		}
		for _, poolID := range poolIDs {
			if seen[poolID] {
				continue
			}
			seen[poolID] = true
			pool := r.pools[poolID]
			if pool != nil {
				candidates = append(candidates, scored{pool: pool, priority: pool.Priority})
			}
		}
	}

	for _ = range words {
		for _, pool := range r.pools {
			if seen[pool.ID] {
				continue
			}
			for _, kw := range pool.Keywords {
				if strings.Contains(intentLower, strings.ToLower(kw)) {
					seen[pool.ID] = true
					candidates = append(candidates, scored{pool: pool, priority: pool.Priority})
					break
				}
			}
		}
	}

	if len(candidates) == 0 {
		if general, ok := r.pools["general"]; ok {
			return general
		}
		if len(r.pools) > 0 {
			for _, pool := range r.pools {
				return pool
			}
		}
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].priority > candidates[j].priority
	})

	return candidates[0].pool
}

// tokenize splits text into tokens using common separators including
// Chinese punctuation and particles.
func tokenize(text string) []string {
	sep := []string{" ", "\t", ",", "，", "、", ";", "；", "|", "：", ":", "的", "了", "是", "在"}
	result := []string{text}
	for _, s := range sep {
		var expanded []string
		for _, r := range result {
			parts := strings.Split(r, s)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					expanded = append(expanded, p)
				}
			}
		}
		result = expanded
	}
	return result
}

// TracedToolDecorator wraps a ReadOnlyTool with OpenTelemetry tracing.
// It records tool name, category, duration, and error status on every invocation.
type TracedToolDecorator struct {
	inner    ReadOnlyTool
	tracer   trace.Tracer
	toolName string
	category domain.ToolCategory
}

// NewTracedToolDecorator creates a new traced decorator for the given tool.
func NewTracedToolDecorator(inner ReadOnlyTool, category domain.ToolCategory) *TracedToolDecorator {
	info, _ := inner.Info(context.Background())
	name := ""
	if info != nil {
		name = info.Name
	}
	return &TracedToolDecorator{
		inner:    inner,
		tracer:   otel.Tracer("cloud-agent-monitor/agent/tool"),
		toolName: name,
		category: category,
	}
}

func (d *TracedToolDecorator) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return d.inner.Info(ctx)
}

func (d *TracedToolDecorator) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	ctx, span := d.tracer.Start(ctx, fmt.Sprintf("execute_tool %s", d.toolName),
		trace.WithAttributes(
			attribute.String("gen_ai.tool.name", d.toolName),
			attribute.String("gen_ai.tool.type", "function"),
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("agent.tool.category", string(d.category)),
		),
	)
	defer span.End()

	start := time.Now()
	result, err := d.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	duration := time.Since(start)

	span.SetAttributes(
		attribute.Int("gen_ai.tool.duration_ms", int(duration.Milliseconds())),
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return result, err
}

func (d *TracedToolDecorator) IsReadOnly() bool {
	return d.inner.IsReadOnly()
}

func (d *TracedToolDecorator) RequiredPermission() string {
	return d.inner.RequiredPermission()
}

// PoolRegistry is the central orchestrator for tool pools, providers, and budgets.
// It coordinates intent routing, tool selection with authorization, and dynamic
// pool registration/unregistration.
type PoolRegistry struct {
	mu        sync.RWMutex
	registry  *ToolRegistry
	pools     map[string]*domain.ToolPool
	router    *IntentRouter
	budget    *domain.ToolBudget
	providers map[domain.ToolCategory]domain.ToolProvider
}

// NewPoolRegistry creates a new PoolRegistry with the given tool registry and budget.
func NewPoolRegistry(registry *ToolRegistry, budget *domain.ToolBudget) *PoolRegistry {
	return &PoolRegistry{
		registry:  registry,
		pools:     make(map[string]*domain.ToolPool),
		router:    NewIntentRouter(),
		budget:    budget,
		providers: make(map[domain.ToolCategory]domain.ToolProvider),
	}
}

// RegisterProvider registers a ToolProvider and its default pools without
// creating tool instances in the registry.
func (pr *PoolRegistry) RegisterProvider(provider domain.ToolProvider) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	cat := provider.Category()
	pr.providers[cat] = provider

	tools, err := provider.Tools(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get tools from provider %s: %w", cat, err)
	}

	for i := range tools {
		spec := tools[i]
		impl, ok := pr.registry.tools[spec.Name]
		if !ok {
			continue
		}
		_ = impl
	}

	for _, pool := range provider.DefaultPools() {
		pr.pools[pool.ID] = pool
		pr.router.AddPool(pool)
	}

	return nil
}

// RegisterProviderWithTools registers a ToolProvider, registers all provided
// tool instances into the tool registry, and indexes the provider's default pools.
func (pr *PoolRegistry) RegisterProviderWithTools(provider domain.ToolProvider, tools []ReadOnlyTool) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	cat := provider.Category()
	pr.providers[cat] = provider

	for _, t := range tools {
		pr.registry.Register(t)
	}

	for _, pool := range provider.DefaultPools() {
		pr.pools[pool.ID] = pool
		pr.router.AddPool(pool)
	}

	return nil
}

// RegisterPool adds a custom tool pool. Returns an error if a pool with the same ID exists.
func (pr *PoolRegistry) RegisterPool(pool *domain.ToolPool) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if _, exists := pr.pools[pool.ID]; exists {
		return fmt.Errorf("pool %s already exists", pool.ID)
	}

	pr.pools[pool.ID] = pool
	pr.router.AddPool(pool)
	return nil
}

// UnregisterPool removes a non-builtin pool by ID. Builtin pools cannot be removed.
func (pr *PoolRegistry) UnregisterPool(poolID string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pool, ok := pr.pools[poolID]
	if !ok {
		return fmt.Errorf("pool %s not found", poolID)
	}

	if pool.IsBuiltin {
		return fmt.Errorf("cannot unregister builtin pool %s", poolID)
	}

	pr.router.RemovePool(poolID)
	delete(pr.pools, poolID)
	return nil
}

// SelectTools routes the intent to the best pool and returns authorized tools
// for the given user. It respects pool MaxTools limits and filters by permissions.
func (pr *PoolRegistry) SelectTools(ctx context.Context, intent string) ([]tool.InvokableTool, error) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	tracer := otel.Tracer("cloud-agent-monitor/agent/pool")
	ctx, span := tracer.Start(ctx, "agent.pool_selection",
		trace.WithAttributes(
			attribute.String("agent.pool.intent", intent),
		),
	)
	defer span.End()

	pool := pr.router.Route(intent)
	if pool == nil {
		span.SetStatus(codes.Error, "no pool matched")
		return nil, fmt.Errorf("no tool pool matched for intent: %s", intent)
	}

	span.SetAttributes(
		attribute.String("agent.pool.selected", pool.ID),
		attribute.Int("agent.pool.priority", pool.Priority),
	)

	userID := GetUserIDFromContext(ctx)
	if userID == "" {
		span.SetStatus(codes.Error, "no user context")
		return nil, fmt.Errorf("authentication required: no user context")
	}

	var selected []tool.InvokableTool
	maxTools := pool.MaxTools
	if maxTools <= 0 {
		maxTools = pr.budget.MaxToolsPerRequest
	}

	for _, toolName := range pool.ToolNames {
		if maxTools > 0 && len(selected) >= maxTools {
			break
		}

		wrapper, ok := pr.registry.wrappers[toolName]
		if !ok {
			continue
		}

		innerTool, toolOk := pr.registry.tools[toolName]
		if !toolOk {
			selected = append(selected, wrapper)
			continue
		}

		permission := innerTool.RequiredPermission()
		hasPermission, err := pr.registry.authz.HasPermission(ctx, userID, permission)
		if err != nil {
			continue
		}
		if !hasPermission {
			continue
		}

		selected = append(selected, wrapper)
	}

	span.SetAttributes(
		attribute.Int("agent.pool.tools_count", len(selected)),
	)

	return selected, nil
}

// ListPools returns all registered pools sorted by priority descending.
func (pr *PoolRegistry) ListPools(ctx context.Context) []*domain.ToolPool {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	result := make([]*domain.ToolPool, 0, len(pr.pools))
	for _, pool := range pr.pools {
		result = append(result, pool)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})

	return result
}

// GetPool returns a pool by ID.
func (pr *PoolRegistry) GetPool(poolID string) (*domain.ToolPool, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	pool, ok := pr.pools[poolID]
	return pool, ok
}

// GetToolsForPool returns all registered tool wrappers for the given pool ID.
func (pr *PoolRegistry) GetToolsForPool(ctx context.Context, poolID string) ([]tool.InvokableTool, error) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	pool, ok := pr.pools[poolID]
	if !ok {
		return nil, fmt.Errorf("pool %s not found", poolID)
	}

	var tools []tool.InvokableTool
	for _, toolName := range pool.ToolNames {
		wrapper, wrapperOk := pr.registry.wrappers[toolName]
		if wrapperOk {
			tools = append(tools, wrapper)
		}
	}

	return tools, nil
}

// GetProviders returns a copy of all registered providers keyed by category.
func (pr *PoolRegistry) GetProviders() map[domain.ToolCategory]domain.ToolProvider {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	result := make(map[domain.ToolCategory]domain.ToolProvider, len(pr.providers))
	for k, v := range pr.providers {
		result[k] = v
	}
	return result
}

// ListAvailableTools delegates to the underlying ToolRegistry to list all tools
// with their permission status for the given user.
func (pr *PoolRegistry) ListAvailableTools(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	toolToPools := make(map[string][]string)
	for _, pool := range pr.pools {
		for _, toolName := range pool.ToolNames {
			toolToPools[toolName] = append(toolToPools[toolName], pool.ID)
		}
	}

	return pr.registry.ListAvailableTools(ctx, userID)
}

// FatToolDelegator forwards thin tool calls to the fat tool with a fixed action.
// It merges the action field into the JSON arguments before delegating.
type FatToolDelegator struct {
	fat    ReadOnlyTool
	action string
}

// NewFatToolDelegator creates a delegator that injects the given action on each call.
func NewFatToolDelegator(fat ReadOnlyTool, action string) *FatToolDelegator {
	return &FatToolDelegator{fat: fat, action: action}
}

// Delegate merges the action field into argsJSON and calls the fat tool.
func (d *FatToolDelegator) Delegate(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	merged := mergeAction(argsJSON, d.action)
	return d.fat.InvokableRun(ctx, merged, opts...)
}

// mergeAction injects or overwrites the "action" key in a JSON string.
// If the input is empty, invalid JSON, or "{}", it returns a minimal {"action":"<action>"} object.
func mergeAction(argsJSON string, action string) string {
	if argsJSON == "" || argsJSON == "{}" {
		return fmt.Sprintf(`{"action":"%s"}`, action)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return fmt.Sprintf(`{"action":"%s"}`, action)
	}
	m["action"] = action
	result, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf(`{"action":"%s"}`, action)
	}
	return string(result)
}

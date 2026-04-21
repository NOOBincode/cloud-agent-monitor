package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	KeyServiceTopology = "service_topology"
	KeyNetworkTopology = "network_topology"
	KeyAdjacencyList   = "adjacency_list"
	KeyNodePrefix      = "node:"
	KeyEdgePrefix      = "edge:"
	KeyImpactPrefix    = "impact:"
)

type RedisCache struct {
	client *redis.Client
	prefix string
}

func NewRedisCache(client *redis.Client) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: "topology:",
	}
}

func (c *RedisCache) buildKey(parts ...string) string {
	key := c.prefix
	for _, part := range parts {
		key += part
	}
	return key
}

func (c *RedisCache) GetServiceTopology(ctx context.Context) (*domain.ServiceTopology, error) {
	key := c.buildKey(KeyServiceTopology)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get service topology from cache: %w", err)
	}

	var topology domain.ServiceTopology
	if err := json.Unmarshal(data, &topology); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service topology: %w", err)
	}

	return &topology, nil
}

func (c *RedisCache) SetServiceTopology(ctx context.Context, topology *domain.ServiceTopology, ttl time.Duration) error {
	if topology == nil {
		return nil
	}

	data, err := json.Marshal(topology)
	if err != nil {
		return fmt.Errorf("failed to marshal service topology: %w", err)
	}

	key := c.buildKey(KeyServiceTopology)
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) GetNetworkTopology(ctx context.Context) (*domain.NetworkTopology, error) {
	key := c.buildKey(KeyNetworkTopology)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get network topology from cache: %w", err)
	}

	var topology domain.NetworkTopology
	if err := json.Unmarshal(data, &topology); err != nil {
		return nil, fmt.Errorf("failed to unmarshal network topology: %w", err)
	}

	return &topology, nil
}

func (c *RedisCache) SetNetworkTopology(ctx context.Context, topology *domain.NetworkTopology, ttl time.Duration) error {
	if topology == nil {
		return nil
	}

	data, err := json.Marshal(topology)
	if err != nil {
		return fmt.Errorf("failed to marshal network topology: %w", err)
	}

	key := c.buildKey(KeyNetworkTopology)
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) GetNode(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	key := c.buildKey(KeyNodePrefix, id.String())
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get node from cache: %w", err)
	}

	var node domain.ServiceNode
	if err := json.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %w", err)
	}

	return &node, nil
}

func (c *RedisCache) SetNode(ctx context.Context, node *domain.ServiceNode, ttl time.Duration) error {
	if node == nil {
		return nil
	}

	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	key := c.buildKey(KeyNodePrefix, node.ID.String())
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) DeleteNode(ctx context.Context, id uuid.UUID) error {
	key := c.buildKey(KeyNodePrefix, id.String())
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) GetEdge(ctx context.Context, id uuid.UUID) (*domain.CallEdge, error) {
	key := c.buildKey(KeyEdgePrefix, id.String())
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get edge from cache: %w", err)
	}

	var edge domain.CallEdge
	if err := json.Unmarshal(data, &edge); err != nil {
		return nil, fmt.Errorf("failed to unmarshal edge: %w", err)
	}

	return &edge, nil
}

func (c *RedisCache) SetEdge(ctx context.Context, edge *domain.CallEdge, ttl time.Duration) error {
	if edge == nil {
		return nil
	}

	data, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("failed to marshal edge: %w", err)
	}

	key := c.buildKey(KeyEdgePrefix, edge.ID.String())
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) GetAdjacencyList(ctx context.Context) (map[uuid.UUID][]uuid.UUID, error) {
	key := c.buildKey(KeyAdjacencyList)
	data, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get adjacency list from cache: %w", err)
	}

	if len(data) == 0 {
		return nil, domain.ErrCacheMiss
	}

	adjList := make(map[uuid.UUID][]uuid.UUID)
	for sourceIDStr, targetsJSON := range data {
		sourceID, err := uuid.Parse(sourceIDStr)
		if err != nil {
			continue
		}

		var targets []string
		if err := json.Unmarshal([]byte(targetsJSON), &targets); err != nil {
			continue
		}

		targetUUIDs := make([]uuid.UUID, 0, len(targets))
		for _, t := range targets {
			if targetID, err := uuid.Parse(t); err == nil {
				targetUUIDs = append(targetUUIDs, targetID)
			}
		}
		adjList[sourceID] = targetUUIDs
	}

	return adjList, nil
}

func (c *RedisCache) SetAdjacencyList(ctx context.Context, adjList map[uuid.UUID][]uuid.UUID, ttl time.Duration) error {
	if len(adjList) == 0 {
		return nil
	}

	key := c.buildKey(KeyAdjacencyList)

	pipe := c.client.Pipeline()
	for sourceID, targets := range adjList {
		targetStrs := make([]string, len(targets))
		for i, t := range targets {
			targetStrs[i] = t.String()
		}
		targetsJSON, _ := json.Marshal(targetStrs)
		pipe.HSet(ctx, key, sourceID.String(), string(targetsJSON))
	}
	pipe.Expire(ctx, key, ttl)

	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) GetImpactCache(ctx context.Context, serviceID uuid.UUID) (*domain.ImpactResult, error) {
	key := c.buildKey(KeyImpactPrefix, serviceID.String())
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrCacheMiss
		}
		return nil, fmt.Errorf("failed to get impact cache: %w", err)
	}

	var result domain.ImpactResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal impact result: %w", err)
	}

	return &result, nil
}

func (c *RedisCache) SetImpactCache(ctx context.Context, serviceID uuid.UUID, result *domain.ImpactResult, ttl time.Duration) error {
	if result == nil {
		return nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal impact result: %w", err)
	}

	key := c.buildKey(KeyImpactPrefix, serviceID.String())
	return c.client.Set(ctx, key, data, ttl).Err()
}

func (c *RedisCache) DeleteImpactCache(ctx context.Context, serviceID uuid.UUID) error {
	key := c.buildKey(KeyImpactPrefix, serviceID.String())
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) InvalidateAll(ctx context.Context) error {
	pattern := c.prefix + "*"
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for invalidation: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	return c.client.Del(ctx, keys...).Err()
}

func (c *RedisCache) InvalidateService(ctx context.Context, serviceID uuid.UUID) error {
	keys := []string{
		c.buildKey(KeyNodePrefix, serviceID.String()),
		c.buildKey(KeyImpactPrefix, serviceID.String()),
	}

	pipe := c.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) BatchSetNodes(ctx context.Context, nodes []*domain.ServiceNode, ttl time.Duration) error {
	if len(nodes) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for _, node := range nodes {
		data, err := json.Marshal(node)
		if err != nil {
			continue
		}
		key := c.buildKey(KeyNodePrefix, node.ID.String())
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) BatchSetEdges(ctx context.Context, edges []*domain.CallEdge, ttl time.Duration) error {
	if len(edges) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for _, edge := range edges {
		data, err := json.Marshal(edge)
		if err != nil {
			continue
		}
		key := c.buildKey(KeyEdgePrefix, edge.ID.String())
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) BatchSetImpactCache(ctx context.Context, results map[uuid.UUID]*domain.ImpactResult, ttl time.Duration) error {
	if len(results) == 0 {
		return nil
	}

	pipe := c.client.Pipeline()
	for serviceID, result := range results {
		data, err := json.Marshal(result)
		if err != nil {
			continue
		}
		key := c.buildKey(KeyImpactPrefix, serviceID.String())
		pipe.Set(ctx, key, data, ttl)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ============ 带重试机制的缓存操作 ============

// ResilientRedisCache 带重试和熔断保护的 Redis 缓存
//
// TODO: 实现带重试机制的缓存操作
// 该结构体包装 RedisCache，添加：
// - 指数退避重试
// - 熔断器保护
// - 超时控制
type ResilientRedisCache struct {
	*RedisCache
	// TODO: 添加重试和熔断器配置
	// retryConfig    resilience.RetryConfig
	// circuitBreaker *resilience.CircuitBreaker
	// timeout        time.Duration
}

// NewResilientRedisCache 创建带重试机制的 Redis 缓存
//
// TODO: 实现创建函数
func NewResilientRedisCache(client *redis.Client, maxRetries int, timeout time.Duration) *ResilientRedisCache {
	// TODO: 实现创建函数
	// 骨架代码：
	// return &ResilientRedisCache{
	//     RedisCache:  NewRedisCache(client),
	//     retryConfig: resilience.RetryConfig{
	//         MaxAttempts:  maxRetries,
	//         InitialDelay: 100 * time.Millisecond,
	//         MaxDelay:     5 * time.Second,
	//         Multiplier:   2.0,
	//         Jitter:       true,
	//     },
	//     circuitBreaker: resilience.NewCircuitBreaker(5, 30*time.Second),
	//     timeout:        timeout,
	// }
	return &ResilientRedisCache{RedisCache: NewRedisCache(client)}
}

// GetServiceTopologyWithRetry 带重试的服务拓扑获取
//
// TODO: 实现带重试的获取逻辑
// 提示：
// 1. 使用 resilience.Retry 包装原始方法
// 2. 设置合理的超时时间
// 3. 只对网络错误重试，不缓存穿透错误
func (c *ResilientRedisCache) GetServiceTopologyWithRetry(ctx context.Context) (*domain.ServiceTopology, error) {
	// TODO: 实现带重试的获取逻辑
	// 骨架代码：
	// ctx, cancel := context.WithTimeout(ctx, c.timeout)
	// defer cancel()
	//
	// var result *domain.ServiceTopology
	// err := resilience.Retry(ctx, c.retryConfig, func() error {
	//     var err error
	//     result, err = c.RedisCache.GetServiceTopology(ctx)
	//     return err
	// }, func(err error) bool {
	//     // 只对网络错误重试
	//     return !errors.Is(err, domain.ErrCacheMiss)
	// })
	//
	// return result, err

	return c.RedisCache.GetServiceTopology(ctx)
}

// SetServiceTopologyWithRetry 带重试的服务拓扑设置
//
// TODO: 实现带重试的设置逻辑
func (c *ResilientRedisCache) SetServiceTopologyWithRetry(ctx context.Context, topology *domain.ServiceTopology, ttl time.Duration) error {
	// TODO: 实现带重试的设置逻辑
	// 参考 GetServiceTopologyWithRetry 实现

	return c.RedisCache.SetServiceTopology(ctx, topology, ttl)
}

// GetNodeWithRetry 带重试的节点获取
//
// TODO: 实现带重试的节点获取
func (c *ResilientRedisCache) GetNodeWithRetry(ctx context.Context, id uuid.UUID) (*domain.ServiceNode, error) {
	// TODO: 实现带重试的节点获取

	return c.RedisCache.GetNode(ctx, id)
}

// IsHealthy 检查缓存健康状态
//
// TODO: 实现健康检查
// 提示：使用 Redis PING 命令检查连接
func (c *ResilientRedisCache) IsHealthy(ctx context.Context) bool {
	// TODO: 实现健康检查
	// 骨架代码：
	// ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	// defer cancel()
	// return c.client.Ping(ctx).Err() == nil

	return true
}

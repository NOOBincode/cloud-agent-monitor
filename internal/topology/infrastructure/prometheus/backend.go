// Package prometheus 提供 Prometheus 指标数据源的拓扑发现能力。
//
// 该包实现了 DiscoveryBackend 接口，通过 Prometheus 查询获取服务指标数据，
// 用于丰富服务节点的健康状态、错误率、延迟等信息。
//
// 核心功能:
//   - 服务发现：通过 up 指标发现监控的服务
//   - 指标丰富：查询错误率、延迟、QPS 等指标
//   - 调用关系发现：通过网络指标推断服务调用关系
package prometheus

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud-agent-monitor/internal/promclient"
	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	"github.com/prometheus/common/model"
)

// Backend 是 Prometheus 数据源的拓扑发现后端。
//
// Backend 通过 Prometheus API 查询指标数据，用于：
//   - 发现被监控的服务
//   - 获取服务的健康状态和性能指标
//   - 推断服务间的调用关系
type Backend struct {
	client       *promclient.Client
	queryTimeout time.Duration
	jobFilter    []string
	mu           sync.RWMutex
}

// Config 定义 Prometheus Backend 的配置参数。
type Config struct {
	// QueryTimeout 查询超时时间
	QueryTimeout time.Duration
	// JobFilter 过滤的 job 列表，为空时查询所有 job
	JobFilter []string
}

// NewBackend 创建一个新的 Prometheus 拓扑发现后端。
//
// 参数:
//   - client: Prometheus 客户端
//   - cfg: 后端配置，为 nil 时使用默认配置
//
// 默认配置:
//   - QueryTimeout: 10 秒
func NewBackend(client *promclient.Client, cfg *Config) *Backend {
	if cfg == nil {
		cfg = &Config{
			QueryTimeout: 10 * time.Second,
		}
	}

	return &Backend{
		client:       client,
		queryTimeout: cfg.QueryTimeout,
		jobFilter:    cfg.JobFilter,
	}
}

// DiscoverNodes 通过 Prometheus 指标发现服务节点。
//
// 该方法通过以下步骤发现服务：
//  1. 查询 up 指标获取被监控的服务列表
//  2. 查询错误率、延迟、QPS 等指标丰富节点信息
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*ServiceNode: 发现的服务节点列表
//   - error: 查询失败时返回错误
func (b *Backend) DiscoverNodes(ctx context.Context) ([]*domain.ServiceNode, error) {
	ctx, cancel := context.WithTimeout(ctx, b.queryTimeout)
	defer cancel()

	nodes, err := b.discoverServicesFromUp(ctx)
	if err != nil {
		return nil, domain.NewTopologyError("discover_nodes", err, "failed to discover services from up metric")
	}

	if err := b.enrichNodesWithMetrics(ctx, nodes); err != nil {
		return nodes, nil
	}

	return nodes, nil
}

// discoverServicesFromUp 通过 up 指标发现被监控的服务。
//
// up 指标包含 job、namespace、service、instance 等标签，
// 用于构建服务节点的基本信息。服务状态根据 up 值判定：
//   - up=1: Healthy
//   - up=0: Unhealthy
func (b *Backend) discoverServicesFromUp(ctx context.Context) ([]*domain.ServiceNode, error) {
	query := `up`
	if len(b.jobFilter) > 0 {
		jobPattern := strings.Join(b.jobFilter, "|")
		query = fmt.Sprintf(`up{job=~"%s"}`, jobPattern)
	}

	results, err := b.client.QueryInstantVector(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query up metric: %w", err)
	}

	nodeMap := make(map[string]*domain.ServiceNode)
	for _, result := range results {
		job := string(result.Metric["job"])
		namespace := string(result.Metric["namespace"])
		service := string(result.Metric["service"])
		instance := string(result.Metric["instance"])

		if job == "" || namespace == "" {
			continue
		}

		if service == "" {
			service = job
		}

		key := fmt.Sprintf("%s/%s", namespace, service)
		if _, exists := nodeMap[key]; !exists {
			nodeMap[key] = &domain.ServiceNode{
				ID:          uuid.NewSHA1(uuid.Nil, []byte(key)),
				Name:        service,
				Namespace:   namespace,
				Environment: string(result.Metric["environment"]),
				Labels:      make(map[string]string),
				Status:      domain.ServiceStatusHealthy,
				UpdatedAt:   time.Now(),
			}

			for k, v := range result.Metric {
				if strings.HasPrefix(string(k), "__") {
					continue
				}
				nodeMap[key].Labels[string(k)] = string(v)
			}
		}

		if result.Value == 0 {
			nodeMap[key].Status = domain.ServiceStatusUnhealthy
		}

		nodeMap[key].PodCount++
		if result.Value == 1 {
			nodeMap[key].ReadyPods++
		}

		_ = instance
	}

	nodes := make([]*domain.ServiceNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// enrichNodesWithMetrics 为服务节点丰富指标数据。
//
// 该方法并发查询每个服务的 QPS、错误率、P99 延迟等指标，
// 更新到对应的 ServiceNode 中。查询失败不影响其他指标的获取。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - nodes: 待丰富的服务节点列表
//
// 返回: 所有查询都失败时返回错误
func (b *Backend) enrichNodesWithMetrics(ctx context.Context, nodes []*domain.ServiceNode) error {
	if len(nodes) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(nodes)*3)

	for i := range nodes {
		node := nodes[i]

		wg.Add(1)
		go func(n *domain.ServiceNode) {
			defer wg.Done()
			if err := b.fetchRequestRate(ctx, n); err != nil {
				errCh <- err
			}
		}(node)

		wg.Add(1)
		go func(n *domain.ServiceNode) {
			defer wg.Done()
			if err := b.fetchErrorRate(ctx, n); err != nil {
				errCh <- err
			}
		}(node)

		wg.Add(1)
		go func(n *domain.ServiceNode) {
			defer wg.Done()
			if err := b.fetchLatency(ctx, n); err != nil {
				errCh <- err
			}
		}(node)
	}

	wg.Wait()
	close(errCh)

	var lastErr error
	for err := range errCh {
		lastErr = err
	}

	return lastErr
}

// fetchRequestRate 查询服务的请求速率（QPS）。
//
// 查询 Prometheus 的 http_requests_total 指标，计算 5 分钟内的请求速率。
// 查询失败时返回 nil（不设置值），不影响其他指标的获取。
func (b *Backend) fetchRequestRate(ctx context.Context, node *domain.ServiceNode) error {
	query := fmt.Sprintf(
		`sum(rate(http_requests_total{namespace="%s",service="%s"}[5m])) or sum(rate(http_requests_total{namespace="%s",job="%s"}[5m]))`,
		node.Namespace, node.Name, node.Namespace, node.Name,
	)

	value, err := b.client.QueryScalar(ctx, query)
	if err != nil {
		return nil
	}
	node.RequestRate = value
	return nil
}

// fetchErrorRate 查询服务的错误率。
//
// 查询 Prometheus 的 http_requests_total 指标，计算 5xx 错误占比。
// 返回值为百分比（0-100）。查询失败时返回 nil。
func (b *Backend) fetchErrorRate(ctx context.Context, node *domain.ServiceNode) error {
	query := fmt.Sprintf(
		`sum(rate(http_requests_total{namespace="%s",service="%s",code=~"5.."}[5m])) / sum(rate(http_requests_total{namespace="%s",service="%s"}[5m])) * 100 or vector(0)`,
		node.Namespace, node.Name, node.Namespace, node.Name,
	)

	value, err := b.client.QueryScalar(ctx, query)
	if err != nil {
		return nil
	}
	node.ErrorRate = value
	return nil
}

// fetchLatency 查询服务的延迟指标。
//
// 查询 Prometheus 的 http_request_duration_seconds 指标，
// 计算 P50、P95、P99 延迟。延迟值转换为毫秒存储。
// 查询失败时跳过对应指标，不影响其他延迟指标。
func (b *Backend) fetchLatency(ctx context.Context, node *domain.ServiceNode) error {
	p99Query := fmt.Sprintf(
		`histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{namespace="%s",service="%s"}[5m])) by (le))`,
		node.Namespace, node.Name,
	)

	p99, err := b.client.QueryScalar(ctx, p99Query)
	if err == nil {
		node.LatencyP99 = p99 * 1000
	}

	p95Query := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{namespace="%s",service="%s"}[5m])) by (le))`,
		node.Namespace, node.Name,
	)

	p95, err := b.client.QueryScalar(ctx, p95Query)
	if err == nil {
		node.LatencyP95 = p95 * 1000
	}

	p50Query := fmt.Sprintf(
		`histogram_quantile(0.50, sum(rate(http_request_duration_seconds_bucket{namespace="%s",service="%s"}[5m])) by (le))`,
		node.Namespace, node.Name,
	)

	p50, err := b.client.QueryScalar(ctx, p50Query)
	if err == nil {
		node.LatencyP50 = p50 * 1000
	}

	return nil
}

// DiscoverEdges 通过 Prometheus 指标发现服务调用关系。
//
// 该方法通过以下指标源发现调用边：
//   - HTTP 客户端指标：http_client_requests_total
//   - gRPC 客户端指标：grpc_client_handled_total
//   - 数据库连接指标：db_client_calls_total
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*CallEdge: 发现的调用边列表
//   - error: 查询失败时返回错误
func (b *Backend) DiscoverEdges(ctx context.Context) ([]*domain.CallEdge, error) {
	ctx, cancel := context.WithTimeout(ctx, b.queryTimeout)
	defer cancel()

	edges, err := b.discoverHTTPClientEdges(ctx)
	if err != nil {
		return nil, domain.NewTopologyError("discover_edges", err, "failed to discover HTTP client edges")
	}

	grpcEdges, err := b.discoverGRPCEdges(ctx)
	if err == nil {
		edges = append(edges, grpcEdges...)
	}

	dbEdges, err := b.discoverDatabaseEdges(ctx)
	if err == nil {
		edges = append(edges, dbEdges...)
	}

	return edges, nil
}

// discoverHTTPClientEdges 通过 HTTP 客户端指标发现服务调用关系。
//
// 查询 http_client_requests_total 指标，提取 namespace、service、host 标签，
// 构建服务间的 HTTP 调用边。置信度为 0.9（高置信度）。
func (b *Backend) discoverHTTPClientEdges(ctx context.Context) ([]*domain.CallEdge, error) {
	query := `sum(rate(http_client_requests_total[5m])) by (namespace,service,host)`

	results, err := b.client.QueryInstantVector(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query HTTP client metrics: %w", err)
	}

	edgeMap := make(map[string]*domain.CallEdge)
	for _, result := range results {
		sourceNamespace := string(result.Metric["namespace"])
		sourceService := string(result.Metric["service"])
		host := string(result.Metric["host"])

		if sourceNamespace == "" || sourceService == "" || host == "" {
			continue
		}

		targetService := b.extractServiceFromHost(host)
		if targetService == "" {
			continue
		}

		targetNamespace := sourceNamespace

		edgeKey := fmt.Sprintf("%s/%s->%s/%s", sourceNamespace, sourceService, targetNamespace, targetService)
		if existing, exists := edgeMap[edgeKey]; exists {
			existing.RequestRate += result.Value
			continue
		}

		edgeMap[edgeKey] = &domain.CallEdge{
			ID:          uuid.NewSHA1(uuid.Nil, []byte(edgeKey)),
			SourceID:    uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", sourceNamespace, sourceService))),
			TargetID:    uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", targetNamespace, targetService))),
			EdgeType:    domain.EdgeTypeHTTP,
			IsDirect:    true,
			Confidence:  0.9,
			Protocol:    "HTTP",
			RequestRate: result.Value,
			UpdatedAt:   time.Now(),
		}
	}

	edges := make([]*domain.CallEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}

	return edges, nil
}

// discoverGRPCEdges 通过 gRPC 客户端指标发现服务调用关系。
//
// 查询 grpc_client_handled_total 指标，提取 namespace、service、grpc_service 标签，
// 构建服务间的 gRPC 调用边。
func (b *Backend) discoverGRPCEdges(ctx context.Context) ([]*domain.CallEdge, error) {
	query := `sum(rate(grpc_client_handled_total[5m])) by (namespace,service,grpc_service)`

	results, err := b.client.QueryInstantVector(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query gRPC client metrics: %w", err)
	}

	edgeMap := make(map[string]*domain.CallEdge)
	for _, result := range results {
		sourceNamespace := string(result.Metric["namespace"])
		sourceService := string(result.Metric["service"])
		grpcService := string(result.Metric["grpc_service"])

		if sourceNamespace == "" || sourceService == "" || grpcService == "" {
			continue
		}

		targetService := b.extractServiceFromGRPCService(grpcService)
		if targetService == "" {
			continue
		}

		edgeKey := fmt.Sprintf("%s/%s->%s/%s:grpc", sourceNamespace, sourceService, sourceNamespace, targetService)
		if existing, exists := edgeMap[edgeKey]; exists {
			existing.RequestRate += result.Value
			continue
		}

		edgeMap[edgeKey] = &domain.CallEdge{
			ID:          uuid.NewSHA1(uuid.Nil, []byte(edgeKey)),
			SourceID:    uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", sourceNamespace, sourceService))),
			TargetID:    uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", sourceNamespace, targetService))),
			EdgeType:    domain.EdgeTypeGRPC,
			IsDirect:    true,
			Confidence:  0.9,
			Protocol:    "gRPC",
			RequestRate: result.Value,
			UpdatedAt:   time.Now(),
		}
	}

	edges := make([]*domain.CallEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}

	return edges, nil
}

// discoverDatabaseEdges 通过数据库连接指标发现服务与数据库的关系。
//
// 查询 db_client_connections 指标，提取 namespace、service、database 标签，
// 构建服务到数据库的调用边。置信度为 0.95（最高置信度）。
func (b *Backend) discoverDatabaseEdges(ctx context.Context) ([]*domain.CallEdge, error) {
	query := `sum(db_client_connections) by (namespace,service,database)`

	results, err := b.client.QueryInstantVector(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query database connection metrics: %w", err)
	}

	edgeMap := make(map[string]*domain.CallEdge)
	for _, result := range results {
		sourceNamespace := string(result.Metric["namespace"])
		sourceService := string(result.Metric["service"])
		database := string(result.Metric["database"])

		if sourceNamespace == "" || sourceService == "" || database == "" {
			continue
		}

		edgeKey := fmt.Sprintf("%s/%s->%s:db", sourceNamespace, sourceService, database)
		if _, exists := edgeMap[edgeKey]; exists {
			continue
		}

		edgeMap[edgeKey] = &domain.CallEdge{
			ID:         uuid.NewSHA1(uuid.Nil, []byte(edgeKey)),
			SourceID:   uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", sourceNamespace, sourceService))),
			TargetID:   uuid.NewSHA1(uuid.Nil, []byte(database)),
			EdgeType:   domain.EdgeTypeDatabase,
			IsDirect:   true,
			Confidence: 0.95,
			Protocol:   "Database",
			UpdatedAt:  time.Now(),
		}
	}

	edges := make([]*domain.CallEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}

	return edges, nil
}

// DiscoverNetworkNodes 通过 Prometheus 指标发现网络节点。
//
// 查询 container_network_receive_bytes_total 指标，提取 namespace、pod 标签，
// 构建网络层的 Pod 节点。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*NetworkNode: 发现的网络节点列表
//   - error: 查询失败时返回错误
func (b *Backend) DiscoverNetworkNodes(ctx context.Context) ([]*domain.NetworkNode, error) {
	ctx, cancel := context.WithTimeout(ctx, b.queryTimeout)
	defer cancel()

	query := `sum(container_network_receive_bytes_total) by (namespace,pod)`

	results, err := b.client.QueryInstantVector(ctx, query)
	if err != nil {
		return nil, domain.NewTopologyError("discover_network_nodes", err, "failed to query network metrics")
	}

	nodeMap := make(map[string]*domain.NetworkNode)
	for _, result := range results {
		namespace := string(result.Metric["namespace"])
		pod := string(result.Metric["pod"])

		if namespace == "" || pod == "" {
			continue
		}

		key := fmt.Sprintf("%s/%s", namespace, pod)
		if _, exists := nodeMap[key]; !exists {
			nodeMap[key] = &domain.NetworkNode{
				ID:        uuid.NewSHA1(uuid.Nil, []byte(key)),
				Name:      pod,
				Type:      "pod",
				Layer:     domain.NetworkLayerPod,
				Namespace: namespace,
				PodName:   pod,
				UpdatedAt: time.Now(),
			}
		}
	}

	nodes := make([]*domain.NetworkNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// DiscoverNetworkEdges 发现网络层的连接关系。
//
// 当前实现返回空列表，网络边的发现需要通过网络流量分析实现。
func (b *Backend) DiscoverNetworkEdges(ctx context.Context) ([]*domain.NetworkEdge, error) {
	return nil, nil
}

// HealthCheck 检查 Prometheus 后端的健康状态。
//
// 委托给 Prometheus 客户端执行健康检查。
func (b *Backend) HealthCheck(ctx context.Context) error {
	return b.client.HealthCheck(ctx)
}

// extractServiceFromHost 从 HTTP host 中提取服务名称。
//
// 移除协议前缀、端口号和域名后缀，返回服务名称。
// 例如：http://user-service.default.svc.cluster.local:8080 -> user-service
func (b *Backend) extractServiceFromHost(host string) string {
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.Split(host, ":")[0]
	host = strings.Split(host, ".")[0]

	return host
}

// extractServiceFromGRPCService 从 gRPC 服务名中提取服务名称。
//
// gRPC 服务名格式通常为 package.Service，提取最后一部分作为服务名。
// 例如：com.example.UserService -> UserService
func (b *Backend) extractServiceFromGRPCService(grpcService string) string {
	parts := strings.Split(grpcService, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return grpcService
}

func (b *Backend) QueryWithResult(ctx context.Context, query string) (model.Value, error) {
	return b.client.Query(ctx, query)
}

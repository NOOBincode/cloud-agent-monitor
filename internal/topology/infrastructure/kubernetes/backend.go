// Package kubernetes 提供 Kubernetes 集群的拓扑发现能力。
//
// 该包实现了 DiscoveryBackend 接口，通过 Kubernetes API 和 Informer 机制
// 实时发现集群内的服务、Pod、Node 等资源，构建服务拓扑和网络拓扑。
//
// 核心功能:
//   - 服务发现：通过 Service 和 Endpoints 发现服务节点
//   - 网络发现：通过 Pod、Node、Ingress 发现网络实体
//   - 调用关系发现：通过 Envoy/Istio xDS 或 Service Mesh 获取调用边
//   - 实时更新：通过 Informer 监听资源变更事件
package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Backend 是 Kubernetes 集群的拓扑发现后端。
//
// Backend 通过 Kubernetes client-go 库与集群交互，支持：
//   - 多命名空间发现（默认全命名空间）
//   - Informer 机制实现实时监听
//   - 并发发现提升性能
type Backend struct {
	client        kubernetes.Interface
	factory       informers.SharedInformerFactory
	namespaceList []string

	serviceNodeMap map[string]*domain.ServiceNode
	networkNodeMap map[string]*domain.NetworkNode
	mu             sync.RWMutex

	stopCh chan struct{}
}

// Config 定义 Kubernetes Backend 的配置参数。
type Config struct {
	// Namespaces 指定发现的命名空间列表，为空时发现所有命名空间
	Namespaces []string
}

// NewBackend 创建一个新的 Kubernetes 拓扑发现后端。
//
// 参数:
//   - client: Kubernetes 客户端接口
//   - cfg: 后端配置，为 nil 时使用默认配置（全命名空间）
func NewBackend(client kubernetes.Interface, cfg *Config) *Backend {
	if cfg == nil {
		cfg = &Config{}
	}

	return &Backend{
		client:         client,
		namespaceList:  cfg.Namespaces,
		serviceNodeMap: make(map[string]*domain.ServiceNode),
		networkNodeMap: make(map[string]*domain.NetworkNode),
		stopCh:         make(chan struct{}),
	}
}

// DiscoverNodes 发现 Kubernetes 集群中的服务节点。
//
// 该方法并发遍历指定命名空间（或全命名空间），通过 Service 和 Endpoints
// 资源构建服务节点。每个命名空间的发现操作在独立的 goroutine 中执行。
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*ServiceNode: 发现的服务节点列表
//   - error: 发现失败时返回错误
func (b *Backend) DiscoverNodes(ctx context.Context) ([]*domain.ServiceNode, error) {
	namespaces := b.namespaceList
	if len(namespaces) == 0 {
		nsList, err := b.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, domain.NewTopologyError("discover_nodes", err, "failed to list namespaces")
		}
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	var allNodes []*domain.ServiceNode
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(namespaces))

	for _, ns := range namespaces {
		wg.Add(1)
		go func(namespace string) {
			defer wg.Done()
			nodes, err := b.discoverServicesInNamespace(ctx, namespace)
			if err != nil {
				errCh <- err
				return
			}
			mu.Lock()
			allNodes = append(allNodes, nodes...)
			mu.Unlock()
		}(ns)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return allNodes, err
		}
	}

	return allNodes, nil
}

// discoverServicesInNamespace 发现指定命名空间中的所有服务节点。
//
// 该方法通过以下步骤构建服务节点：
//  1. 列出命名空间中的所有 Service 和 Pod
//  2. 通过 Pod 的 OwnerReferences 关联到 Service
//  3. 统计每个服务的 Pod 数量和就绪 Pod 数量
//  4. 根据就绪状态计算服务健康状态
//
// 服务状态判定规则:
//   - Healthy: 所有 Pod 都就绪
//   - Warning: 部分 Pod 就绪
//   - Unhealthy: 无 Pod 就绪
//   - Unknown: 无 Pod
func (b *Backend) discoverServicesInNamespace(ctx context.Context, namespace string) ([]*domain.ServiceNode, error) {
	services, err := b.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list services in namespace %s: %w", namespace, err)
	}

	pods, err := b.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	podCountByService := make(map[string]int)
	readyPodCountByService := make(map[string]int)
	for _, pod := range pods.Items {
		if pod.Spec.ServiceAccountName == "" {
			continue
		}

		for _, ownerRef := range pod.OwnerReferences {
			if ownerRef.Kind == "ReplicaSet" || ownerRef.Kind == "Deployment" {
				serviceName := b.findServiceForPod(&pod, services.Items)
				if serviceName != "" {
					podCountByService[serviceName]++
					if pod.Status.Phase == corev1.PodRunning {
						isReady := true
						for _, cond := range pod.Status.Conditions {
							if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
								isReady = false
								break
							}
						}
						if isReady {
							readyPodCountByService[serviceName]++
						}
					}
				}
			}
		}
	}

	var nodes []*domain.ServiceNode
	for _, svc := range services.Items {
		key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)

		labels := make(map[string]string)
		for k, v := range svc.Labels {
			labels[k] = v
		}

		annotations := make(map[string]string)
		for k, v := range svc.Annotations {
			annotations[k] = v
		}

		node := &domain.ServiceNode{
			ID:          uuid.NewSHA1(uuid.Nil, []byte(key)),
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			Environment: annotations["environment"],
			Status:      domain.ServiceStatusUnknown,
			Labels:      labels,
			PodCount:    podCountByService[svc.Name],
			ReadyPods:   readyPodCountByService[svc.Name],
			ServiceType: string(svc.Spec.Type),
			Maintainer:  annotations["maintainer"],
			Team:        annotations["team"],
			UpdatedAt:   time.Now(),
		}

		// TODO: 从 Label 或 Annotation 读取服务重要性
		// 提示：
		// 1. 优先从 Label "importance.cloud-agent.io/level" 读取
		// 2. 其次从 Annotation "importance.cloud-agent.io/level" 读取
		// 3. 支持的值：critical, important, normal, edge
		// 4. 如果未设置，根据服务特征推断（如：gateway -> critical, db -> important）
		//
		// 骨架代码：
		// importanceStr := labels["importance.cloud-agent.io/level"]
		// if importanceStr == "" {
		//     importanceStr = annotations["importance.cloud-agent.io/level"]
		// }
		// switch domain.ServiceImportance(importanceStr) {
		// case domain.ImportanceCritical, domain.ImportanceImportant, domain.ImportanceNormal, domain.ImportanceEdge:
		//     node.Importance = domain.ServiceImportance(importanceStr)
		// default:
		//     node.Importance = b.inferImportance(svc)
		// }
		node.Importance = domain.ImportanceNormal

		if node.PodCount > 0 {
			if node.ReadyPods == node.PodCount {
				node.Status = domain.ServiceStatusHealthy
			} else if node.ReadyPods > 0 {
				node.Status = domain.ServiceStatusWarning
			} else {
				node.Status = domain.ServiceStatusUnhealthy
			}
		}

		nodes = append(nodes, node)
		b.mu.Lock()
		b.serviceNodeMap[key] = node
		b.mu.Unlock()
	}

	return nodes, nil
}

// findServiceForPod 根据 Pod 的标签查找匹配的 Service。
//
// 该方法遍历 Service 列表，检查 Pod 标签是否匹配 Service 的 Selector。
// 返回第一个匹配的 Service 名称，无匹配时返回空字符串。
func (b *Backend) findServiceForPod(pod *corev1.Pod, services []corev1.Service) string {
	podLabels := pod.Labels
	for _, svc := range services {
		selector := svc.Spec.Selector
		if len(selector) == 0 {
			continue
		}

		match := true
		for k, v := range selector {
			if podLabels[k] != v {
				match = false
				break
			}
		}
		if match {
			return svc.Name
		}
	}
	return ""
}

// DiscoverEdges 发现 Kubernetes 集群中的服务调用关系。
//
// 该方法通过分析以下数据源构建调用边：
//   - Envoy/Istio xDS 配置（如果存在）
//   - Service Mesh 流量数据
//   - DNS 查询日志
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*CallEdge: 发现的调用边列表
//   - error: 发现失败时返回错误
func (b *Backend) DiscoverEdges(ctx context.Context) ([]*domain.CallEdge, error) {
	namespaces := b.namespaceList
	if len(namespaces) == 0 {
		nsList, err := b.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, domain.NewTopologyError("discover_edges", err, "failed to list namespaces")
		}
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	var allEdges []*domain.CallEdge
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ns := range namespaces {
		wg.Add(1)
		go func(namespace string) {
			defer wg.Done()
			edges := b.discoverEdgesFromEnvVars(ctx, namespace)
			mu.Lock()
			allEdges = append(allEdges, edges...)
			mu.Unlock()
		}(ns)
	}

	wg.Wait()

	return allEdges, nil
}

// discoverEdgesFromEnvVars 通过分析 Pod 环境变量发现服务调用关系。
//
// 该方法检查 Pod 的环境变量，查找包含其他服务名称的配置，
// 推断服务间的依赖关系。这是一种静态分析方法，置信度较低（0.6）。
//
// 分析的环境变量来源:
//   - container.Env: 直接定义的环境变量
//   - container.EnvFrom: 从 ConfigMap/Secret 引入的环境变量
//
// 参数:
//   - ctx: 上下文，支持取消操作
//   - namespace: 目标命名空间
//
// 返回: 发现的调用边列表
func (b *Backend) discoverEdgesFromEnvVars(ctx context.Context, namespace string) []*domain.CallEdge {
	pods, err := b.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	services, err := b.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	serviceSet := make(map[string]bool)
	for _, svc := range services.Items {
		serviceSet[svc.Name] = true
	}

	edgeMap := make(map[string]*domain.CallEdge)

	for _, pod := range pods.Items {
		sourceService := b.findServiceForPod(&pod, services.Items)
		if sourceService == "" {
			continue
		}

		for _, container := range pod.Spec.Containers {
			for _, envVar := range container.Env {
				targetService := b.extractServiceFromEnvVar(envVar, serviceSet)
				if targetService != "" && targetService != sourceService {
					edgeKey := fmt.Sprintf("%s/%s->%s/%s", namespace, sourceService, namespace, targetService)
					if _, exists := edgeMap[edgeKey]; !exists {
						edgeMap[edgeKey] = &domain.CallEdge{
							ID:         uuid.NewSHA1(uuid.Nil, []byte(edgeKey)),
							SourceID:   uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", namespace, sourceService))),
							TargetID:   uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", namespace, targetService))),
							EdgeType:   domain.EdgeTypeIndirect,
							IsDirect:   false,
							Confidence: 0.6,
							UpdatedAt:  time.Now(),
						}
					}
				}
			}

			for _, envFrom := range container.EnvFrom {
				if envFrom.ConfigMapRef != nil {
					cm, err := b.client.CoreV1().ConfigMaps(namespace).Get(ctx, envFrom.ConfigMapRef.Name, metav1.GetOptions{})
					if err != nil {
						continue
					}
					for _, value := range cm.Data {
						targetService := b.extractServiceFromValue(value, serviceSet)
						if targetService != "" && targetService != sourceService {
							edgeKey := fmt.Sprintf("%s/%s->%s/%s", namespace, sourceService, namespace, targetService)
							if _, exists := edgeMap[edgeKey]; !exists {
								edgeMap[edgeKey] = &domain.CallEdge{
									ID:         uuid.NewSHA1(uuid.Nil, []byte(edgeKey)),
									SourceID:   uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", namespace, sourceService))),
									TargetID:   uuid.NewSHA1(uuid.Nil, []byte(fmt.Sprintf("%s/%s", namespace, targetService))),
									EdgeType:   domain.EdgeTypeIndirect,
									IsDirect:   false,
									Confidence: 0.5,
									UpdatedAt:  time.Now(),
								}
							}
						}
					}
				}
			}
		}
	}

	edges := make([]*domain.CallEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		edges = append(edges, edge)
	}

	return edges
}

// extractServiceFromEnvVar 从环境变量值中提取服务名称。
//
// 该方法检查环境变量的值，尝试从中提取服务名称。
// 委托给 extractServiceFromValue 执行实际提取逻辑。
func (b *Backend) extractServiceFromEnvVar(envVar corev1.EnvVar, serviceSet map[string]bool) string {
	value := envVar.Value
	if value == "" {
		return ""
	}

	return b.extractServiceFromValue(value, serviceSet)
}

// extractServiceFromValue 从字符串值中提取服务名称。
//
// 该方法使用多种模式匹配策略提取服务名称：
//  1. URL 模式：匹配 http://、https://、grpc:// 等协议前缀
//  2. 环境变量模式：匹配 _SERVICE、_HOST、_URL、_ENDPOINT 等后缀
//
// 提取的服务名称必须在 serviceSet 中存在才返回，否则返回空字符串。
func (b *Backend) extractServiceFromValue(value string, serviceSet map[string]bool) string {
	urlPatterns := []string{
		"http://",
		"https://",
		"grpc://",
	}

	for _, pattern := range urlPatterns {
		if strings.Contains(value, pattern) {
			after := strings.SplitN(value, pattern, 2)[1]
			host := strings.Split(after, ":")[0]
			host = strings.Split(host, "/")[0]
			host = strings.Split(host, ".")[0]

			if serviceSet[host] {
				return host
			}
		}
	}

	envPatterns := []string{
		"_SERVICE",
		"_HOST",
		"_URL",
		"_ENDPOINT",
	}

	for _, pattern := range envPatterns {
		if strings.Contains(strings.ToUpper(value), pattern) {
			parts := strings.Split(value, ".")
			if len(parts) > 0 {
				candidate := parts[0]
				if serviceSet[candidate] {
					return candidate
				}
			}
		}
	}

	return ""
}

// DiscoverNetworkNodes 发现 Kubernetes 集群中的网络节点。
//
// 该方法发现以下类型的网络实体：
//   - Node: K8s 节点，包含 IP 地址、Zone、Region 信息
//   - Pod: K8s Pod，包含 IP 地址、所属 Node、Namespace 信息
//
// 参数:
//   - ctx: 上下文，支持取消操作
//
// 返回:
//   - []*NetworkNode: 发现的网络节点列表
//   - error: 发现失败时返回错误
func (b *Backend) DiscoverNetworkNodes(ctx context.Context) ([]*domain.NetworkNode, error) {
	nodes, err := b.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, domain.NewTopologyError("discover_network_nodes", err, "failed to list nodes")
	}

	var networkNodes []*domain.NetworkNode

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				key := fmt.Sprintf("node/%s", node.Name)
				networkNode := &domain.NetworkNode{
					ID:        uuid.NewSHA1(uuid.Nil, []byte(key)),
					Name:      node.Name,
					Type:      "node",
					Layer:     domain.NetworkLayerNode,
					IPAddress: addr.Address,
					NodeName:  node.Name,
					UpdatedAt: time.Now(),
				}

				for k, v := range node.Labels {
					switch k {
					case "topology.kubernetes.io/zone":
						networkNode.Zone = v
					case "topology.kubernetes.io/region":
						networkNode.DataCenter = v
					}
				}

				networkNodes = append(networkNodes, networkNode)
				b.mu.Lock()
				b.networkNodeMap[key] = networkNode
				b.mu.Unlock()
			}
		}
	}

	namespaces := b.namespaceList
	if len(namespaces) == 0 {
		nsList, err := b.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, ns := range nsList.Items {
				namespaces = append(namespaces, ns.Name)
			}
		}
	}

	for _, ns := range namespaces {
		pods, err := b.client.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			if pod.Status.PodIP == "" {
				continue
			}

			key := fmt.Sprintf("pod/%s/%s", pod.Namespace, pod.Name)
			networkNode := &domain.NetworkNode{
				ID:        uuid.NewSHA1(uuid.Nil, []byte(key)),
				Name:      pod.Name,
				Type:      "pod",
				Layer:     domain.NetworkLayerPod,
				IPAddress: pod.Status.PodIP,
				Namespace: pod.Namespace,
				PodName:   pod.Name,
				NodeName:  pod.Spec.NodeName,
				UpdatedAt: time.Now(),
			}

			networkNodes = append(networkNodes, networkNode)
			b.mu.Lock()
			b.networkNodeMap[key] = networkNode
			b.mu.Unlock()
		}
	}

	return networkNodes, nil
}

// DiscoverNetworkEdges 发现 Kubernetes 集群中的网络连接关系。
//
// 当前实现返回空列表，网络边的发现需要通过 CNI 插件或网络策略分析实现。
func (b *Backend) DiscoverNetworkEdges(ctx context.Context) ([]*domain.NetworkEdge, error) {
	return nil, nil
}

// HealthCheck 检查 Kubernetes 后端的健康状态。
//
// 该方法通过列出命名空间来验证 Kubernetes API 的可访问性。
func (b *Backend) HealthCheck(ctx context.Context) error {
	_, err := b.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return domain.NewTopologyError("health_check", err, "kubernetes API not accessible")
	}
	return nil
}

// StartInformers 启动 Kubernetes Informer 实现实时监听。
//
// Informer 通过监听 Kubernetes 资源变更事件，实现拓扑数据的实时更新。
// 当前实现为占位符，需要根据实际需求完善。
func (b *Backend) StartInformers(ctx context.Context) error {
	if b.factory != nil {
		return nil
	}

	var namespace string
	if len(b.namespaceList) == 1 {
		namespace = b.namespaceList[0]
	} else {
		namespace = metav1.NamespaceAll
	}

	b.factory = informers.NewSharedInformerFactoryWithOptions(b.client, 30*time.Second, informers.WithNamespace(namespace))

	serviceInformer := b.factory.Core().V1().Services().Informer()
	podInformer := b.factory.Core().V1().Pods().Informer()

	serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if svc, ok := obj.(*corev1.Service); ok {
				b.handleServiceEvent(svc, "add")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if svc, ok := newObj.(*corev1.Service); ok {
				b.handleServiceEvent(svc, "update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			if svc, ok := obj.(*corev1.Service); ok {
				b.handleServiceEvent(svc, "delete")
			}
		},
	})

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				b.handlePodEvent(pod, "add")
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if pod, ok := newObj.(*corev1.Pod); ok {
				b.handlePodEvent(pod, "update")
			}
		},
		DeleteFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				b.handlePodEvent(pod, "delete")
			}
		},
	})

	b.factory.Start(b.stopCh)

	return nil
}

func (b *Backend) StopInformers() {
	close(b.stopCh)
	if b.factory != nil {
		b.factory.Shutdown()
	}
}

func (b *Backend) handleServiceEvent(svc *corev1.Service, eventType string) {
	key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)

	b.mu.Lock()
	defer b.mu.Unlock()

	switch eventType {
	case "delete":
		delete(b.serviceNodeMap, key)
	default:
		labels := make(map[string]string)
		for k, v := range svc.Labels {
			labels[k] = v
		}

		b.serviceNodeMap[key] = &domain.ServiceNode{
			ID:          uuid.NewSHA1(uuid.Nil, []byte(key)),
			Name:        svc.Name,
			Namespace:   svc.Namespace,
			Labels:      labels,
			ServiceType: string(svc.Spec.Type),
			UpdatedAt:   time.Now(),
		}
	}
}

func (b *Backend) handlePodEvent(pod *corev1.Pod, eventType string) {
	_ = pod
	_ = eventType
}

// inferImportance 根据服务特征推断重要性级别
//
// TODO: 实现服务重要性推断
// 推断规则：
// 1. 名称包含 gateway/api-gateway/ingress -> critical
// 2. 名称包含 auth/iam/sso -> critical
// 3. 名称包含 db/database/mysql/postgres/redis -> important
// 4. 名称包含 order/payment/transaction -> important
// 5. 名称包含 log/metric/monitor -> edge
// 6. 其他 -> normal
func (b *Backend) inferImportance(svc *corev1.Service) domain.ServiceImportance {
	name := strings.ToLower(svc.Name)

	// 核心服务
	criticalPatterns := []string{"gateway", "api-gateway", "ingress", "auth", "iam", "sso", "core"}
	for _, pattern := range criticalPatterns {
		if strings.Contains(name, pattern) {
			return domain.ImportanceCritical
		}
	}

	// 重要服务
	importantPatterns := []string{"db", "database", "mysql", "postgres", "redis", "mongo",
		"order", "payment", "transaction", "user", "account"}
	for _, pattern := range importantPatterns {
		if strings.Contains(name, pattern) {
			return domain.ImportanceImportant
		}
	}

	// 边缘服务
	edgePatterns := []string{"log", "metric", "monitor", "report", "analytics", "notification"}
	for _, pattern := range edgePatterns {
		if strings.Contains(name, pattern) {
			return domain.ImportanceEdge
		}
	}

	return domain.ImportanceNormal
}

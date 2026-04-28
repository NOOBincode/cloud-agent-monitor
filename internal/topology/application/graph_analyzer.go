// Package application 实现拓扑模块的应用层业务逻辑。
//
// 该包包含拓扑服务的核心业务逻辑，负责协调领域对象完成
// 拓扑发现、影响分析、路径查找等核心功能。应用层通过依赖注入
// 使用基础设施层提供的服务，保持领域模型的纯粹性。
//
// 核心组件:
//   - TopologyService: 拓扑服务主入口，协调发现、缓存和持久化
//   - GraphAnalyzer: 图算法分析器，提供影响分析和路径查找
//   - ImpactCacheService: 影响分析结果预计算服务
//   - InMemoryGraph: 内存图结构，支持高效的拓扑遍历
package application

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"time"

	"cloud-agent-monitor/internal/topology/domain"

	"github.com/google/uuid"
)

const (
	HighErrorRateThreshold = 0.05
	HighLatencyThresholdMs = 1000.0
	DefaultAnomalyMaxDepth = 5
	DefaultPathMaxHops     = 10
)

type GraphAnalyzer struct {
	graph *InMemoryGraph
}

func NewGraphAnalyzer(graph *InMemoryGraph) *GraphAnalyzer {
	return &GraphAnalyzer{
		graph: graph,
	}
}

func (a *GraphAnalyzer) AnalyzeImpact(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*domain.ImpactResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(serviceID)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}

	result := &domain.ImpactResult{
		RootService:     node,
		UpstreamDepth:   0,
		DownstreamDepth: 0,
		Upstream:        make([]*domain.ImpactNode, 0),
		Downstream:      make([]*domain.ImpactNode, 0),
		CriticalPath:    make([]domain.PathHop, 0),
		AnalyzedAt:      time.Now(),
	}

	upstreamVisited := make(map[uuid.UUID]int)
	a.bfsUpstream(serviceID, maxDepth, upstreamVisited)

	for id, depth := range upstreamVisited {
		if id == serviceID {
			continue
		}
		n := a.graph.GetNode(id)
		if n != nil {
			result.Upstream = append(result.Upstream, &domain.ImpactNode{
				Node:       n,
				Depth:      depth,
				Impact:     a.calculateImpactScore(n, depth),
				IsCritical: depth <= 2,
			})
			if depth > result.UpstreamDepth {
				result.UpstreamDepth = depth
			}
		}
	}

	downstreamVisited := make(map[uuid.UUID]int)
	a.bfsDownstream(serviceID, maxDepth, downstreamVisited)

	for id, depth := range downstreamVisited {
		if id == serviceID {
			continue
		}
		n := a.graph.GetNode(id)
		if n != nil {
			result.Downstream = append(result.Downstream, &domain.ImpactNode{
				Node:       n,
				Depth:      depth,
				Impact:     a.calculateImpactScore(n, depth),
				IsCritical: depth <= 2,
			})
			if depth > result.DownstreamDepth {
				result.DownstreamDepth = depth
			}
		}
	}

	result.TotalAffected = len(result.Upstream) + len(result.Downstream)

	return result, nil
}

func (a *GraphAnalyzer) bfsUpstream(start uuid.UUID, maxDepth int, visited map[uuid.UUID]int) {
	a.bfs(start, maxDepth, visited, a.graph.GetUpstreamIDs)
}

func (a *GraphAnalyzer) bfsDownstream(start uuid.UUID, maxDepth int, visited map[uuid.UUID]int) {
	a.bfs(start, maxDepth, visited, a.graph.GetDownstreamIDs)
}

func (a *GraphAnalyzer) bfs(start uuid.UUID, maxDepth int, visited map[uuid.UUID]int, getNeighbors func(uuid.UUID) []uuid.UUID) {
	queue := []struct {
		id    uuid.UUID
		depth int
	}{{id: start, depth: 0}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth > maxDepth {
			continue
		}

		if _, seen := visited[current.id]; seen {
			continue
		}
		visited[current.id] = current.depth

		neighborIDs := getNeighbors(current.id)
		for _, neighborID := range neighborIDs {
			if _, seen := visited[neighborID]; !seen {
				queue = append(queue, struct {
					id    uuid.UUID
					depth int
				}{id: neighborID, depth: current.depth + 1})
			}
		}
	}
}

func (a *GraphAnalyzer) calculateImpactScore(node *domain.ServiceNode, depth int) float64 {
	baseScore := 1.0 / float64(depth)

	statusMultiplier := 1.0
	switch node.Status {
	case domain.ServiceStatusUnhealthy:
		statusMultiplier = 2.0
	case domain.ServiceStatusWarning:
		statusMultiplier = 1.5
	case domain.ServiceStatusHealthy:
		statusMultiplier = 1.0
	default:
		statusMultiplier = 1.0
	}

	trafficMultiplier := 1.0
	if node.RequestRate > 1000 {
		trafficMultiplier = 1.5
	} else if node.RequestRate > 100 {
		trafficMultiplier = 1.2
	}

	return baseScore * statusMultiplier * trafficMultiplier
}

func (a *GraphAnalyzer) FindPath(ctx context.Context, sourceID, targetID uuid.UUID, maxHops int) (*domain.PathResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil {
		return nil, domain.ErrNodeNotFound
	}
	if a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	paths := a.findAllPaths(sourceID, targetID, maxHops)
	if len(paths) == 0 {
		return nil, domain.ErrPathNotFound
	}

	result := &domain.PathResult{
		SourceID: sourceID,
		TargetID: targetID,
		Paths:    paths,
		FoundAt:  time.Now(),
	}

	if len(paths) > 0 {
		result.ShortestPath = paths[0]
	}

	return result, nil
}

func (a *GraphAnalyzer) findAllPaths(sourceID, targetID uuid.UUID, maxHops int) [][]domain.PathHop {
	var paths [][]domain.PathHop
	visited := make(map[uuid.UUID]bool)
	currentPath := make([]domain.PathHop, 0)

	a.dfsPaths(sourceID, targetID, maxHops, visited, &currentPath, &paths)

	return paths
}

func (a *GraphAnalyzer) dfsPaths(
	current, target uuid.UUID,
	maxHops int,
	visited map[uuid.UUID]bool,
	currentPath *[]domain.PathHop,
	paths *[][]domain.PathHop,
) {
	if len(*currentPath) > maxHops {
		return
	}

	if visited[current] {
		return
	}

	node := a.graph.GetNode(current)
	if node == nil {
		return
	}

	visited[current] = true
	*currentPath = append(*currentPath, domain.PathHop{
		NodeID:    current,
		NodeName:  node.Name,
		Namespace: node.Namespace,
	})

	if current == target {
		pathCopy := make([]domain.PathHop, len(*currentPath))
		copy(pathCopy, *currentPath)
		*paths = append(*paths, pathCopy)
	} else {
		downstreamIDs := a.graph.GetDownstreamIDs(current)
		for _, nextID := range downstreamIDs {
			a.dfsPaths(nextID, target, maxHops, visited, currentPath, paths)
		}
	}

	*currentPath = (*currentPath)[:len(*currentPath)-1]
	delete(visited, current)
}

func (a *GraphAnalyzer) FindShortestPath(ctx context.Context, sourceID, targetID uuid.UUID) ([]domain.PathHop, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil {
		return nil, domain.ErrNodeNotFound
	}
	if a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	parent := make(map[uuid.UUID]uuid.UUID)
	visited := make(map[uuid.UUID]bool)
	queue := []uuid.UUID{sourceID}
	visited[sourceID] = true

	found := false
	for len(queue) > 0 && !found {
		current := queue[0]
		queue = queue[1:]

		if current == targetID {
			found = true
			break
		}

		downstreamIDs := a.graph.GetDownstreamIDs(current)
		for _, nextID := range downstreamIDs {
			if !visited[nextID] {
				visited[nextID] = true
				parent[nextID] = current
				queue = append(queue, nextID)
			}
		}
	}

	if !found {
		return nil, domain.ErrPathNotFound
	}

	var path []domain.PathHop
	current := targetID
	for current != sourceID {
		node := a.graph.GetNode(current)
		if node != nil {
			path = append([]domain.PathHop{{
				NodeID:    current,
				NodeName:  node.Name,
				Namespace: node.Namespace,
			}}, path...)
		}
		current = parent[current]
	}

	sourceNode := a.graph.GetNode(sourceID)
	if sourceNode != nil {
		path = append([]domain.PathHop{{
			NodeID:    sourceID,
			NodeName:  sourceNode.Name,
			Namespace: sourceNode.Namespace,
		}}, path...)
	}

	return path, nil
}

func (a *GraphAnalyzer) FindAnomalies(ctx context.Context) ([]*domain.TopologyAnomaly, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	var anomalies []*domain.TopologyAnomaly

	nodes := a.graph.GetAllNodes()
	for _, node := range nodes {
		if node.Status == domain.ServiceStatusUnhealthy {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "unhealthy_service",
				Severity:    "high",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service is in unhealthy state",
				DetectedAt:  time.Now(),
			})
		}

		if node.ErrorRate > HighErrorRateThreshold {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "high_error_rate",
				Severity:    "medium",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service has high error rate",
				Metrics: map[string]float64{
					"error_rate": node.ErrorRate,
				},
				DetectedAt: time.Now(),
			})
		}

		if node.LatencyP99 > HighLatencyThresholdMs {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "high_latency",
				Severity:    "medium",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service has high P99 latency",
				Metrics: map[string]float64{
					"latency_p99": node.LatencyP99,
				},
				DetectedAt: time.Now(),
			})
		}

		if node.PodCount > 0 && node.ReadyPods < node.PodCount {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "pod_degradation",
				Severity:    "medium",
				NodeID:      node.ID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Not all pods are ready",
				Metrics: map[string]float64{
					"pod_count":  float64(node.PodCount),
					"ready_pods": float64(node.ReadyPods),
				},
				DetectedAt: time.Now(),
			})
		}
	}

	cycles := a.detectCycles()
	for _, cycle := range cycles {
		if len(cycle) > 0 {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "circular_dependency",
				Severity:    "high",
				NodeID:      cycle[0],
				NodeName:    "",
				Namespace:   "",
				Description: "Circular dependency detected in service topology",
				DetectedAt:  time.Now(),
			})
		}
	}

	orphanNodes := a.findOrphanNodes()
	for _, nodeID := range orphanNodes {
		node := a.graph.GetNode(nodeID)
		if node != nil {
			anomalies = append(anomalies, &domain.TopologyAnomaly{
				ID:          uuid.New(),
				Type:        "orphan_service",
				Severity:    "low",
				NodeID:      nodeID,
				NodeName:    node.Name,
				Namespace:   node.Namespace,
				Description: "Service has no incoming or outgoing connections",
				DetectedAt:  time.Now(),
			})
		}
	}

	return anomalies, nil
}

func (a *GraphAnalyzer) detectCycles() [][]uuid.UUID {
	var cycles [][]uuid.UUID
	visited := make(map[uuid.UUID]bool)
	recStack := make(map[uuid.UUID]bool)
	var path []uuid.UUID

	for _, node := range a.graph.GetAllNodes() {
		if !visited[node.ID] {
			a.dfsCycle(node.ID, visited, recStack, &path, &cycles)
		}
	}

	return cycles
}

func (a *GraphAnalyzer) dfsCycle(
	current uuid.UUID,
	visited, recStack map[uuid.UUID]bool,
	path *[]uuid.UUID,
	cycles *[][]uuid.UUID,
) {
	visited[current] = true
	recStack[current] = true
	*path = append(*path, current)

	downstreamIDs := a.graph.GetDownstreamIDs(current)
	for _, nextID := range downstreamIDs {
		if !visited[nextID] {
			a.dfsCycle(nextID, visited, recStack, path, cycles)
		} else if recStack[nextID] {
			cycleStart := -1
			for i, id := range *path {
				if id == nextID {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := make([]uuid.UUID, len(*path)-cycleStart)
				copy(cycle, (*path)[cycleStart:])
				*cycles = append(*cycles, cycle)
			}
		}
	}

	*path = (*path)[:len(*path)-1]
	recStack[current] = false
}

func (a *GraphAnalyzer) findOrphanNodes() []uuid.UUID {
	var orphans []uuid.UUID

	for _, node := range a.graph.GetAllNodes() {
		upstream := a.graph.GetUpstreamIDs(node.ID)
		downstream := a.graph.GetDownstreamIDs(node.ID)

		if len(upstream) == 0 && len(downstream) == 0 {
			orphans = append(orphans, node.ID)
		}
	}

	return orphans
}

func (a *GraphAnalyzer) CalculateCentrality(ctx context.Context) (map[uuid.UUID]float64, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	centrality := make(map[uuid.UUID]float64)
	nodes := a.graph.GetAllNodes()

	for _, node := range nodes {
		centrality[node.ID] = a.calculateBetweennessCentrality(node.ID)
	}

	return centrality, nil
}

func (a *GraphAnalyzer) calculateBetweennessCentrality(nodeID uuid.UUID) float64 {
	totalPaths := 0
	pathsThroughNode := 0

	nodes := a.graph.GetAllNodes()
	for i, source := range nodes {
		for j, target := range nodes {
			if i >= j {
				continue
			}

			paths := a.findAllPaths(source.ID, target.ID, 5)
			for _, path := range paths {
				totalPaths++
				for _, hop := range path {
					if hop.NodeID == nodeID {
						pathsThroughNode++
						break
					}
				}
			}
		}
	}

	if totalPaths == 0 {
		return 0
	}

	return float64(pathsThroughNode) / float64(totalPaths)
}

func (a *GraphAnalyzer) FindClusters(ctx context.Context) ([][]uuid.UUID, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	visited := make(map[uuid.UUID]bool)
	var clusters [][]uuid.UUID

	for _, node := range a.graph.GetAllNodes() {
		if !visited[node.ID] {
			cluster := a.findConnectedComponent(node.ID, visited)
			if len(cluster) > 1 {
				clusters = append(clusters, cluster)
			}
		}
	}

	return clusters, nil
}

func (a *GraphAnalyzer) findConnectedComponent(start uuid.UUID, visited map[uuid.UUID]bool) []uuid.UUID {
	var component []uuid.UUID
	queue := []uuid.UUID{start}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}

		visited[current] = true
		component = append(component, current)

		for _, neighbor := range a.graph.GetDownstreamIDs(current) {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}

		for _, neighbor := range a.graph.GetUpstreamIDs(current) {
			if !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}
	}

	return component
}

type Path struct {
	Nodes []uuid.UUID
	Cost  float64
}

type PriorityQueue []*Path

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Cost < pq[j].Cost
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x any) {
	*pq = append(*pq, x.(*Path))
}

func (pq *PriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func (a *GraphAnalyzer) FindShortestPathWeighted(ctx context.Context, sourceID, targetID uuid.UUID, weightFunc func(edge *domain.CallEdge) float64) ([]uuid.UUID, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil || a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	if weightFunc == nil {
		weightFunc = func(edge *domain.CallEdge) float64 {
			return 1.0
		}
	}

	dist := make(map[uuid.UUID]float64)
	prev := make(map[uuid.UUID]uuid.UUID)

	for _, node := range a.graph.GetAllNodes() {
		dist[node.ID] = math.Inf(1)
	}
	dist[sourceID] = 0

	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	heap.Push(&pq, &Path{Nodes: []uuid.UUID{sourceID}, Cost: 0})

	visited := make(map[uuid.UUID]bool)

	for pq.Len() > 0 {
		current := heap.Pop(&pq).(*Path)
		currentNode := current.Nodes[len(current.Nodes)-1]

		if visited[currentNode] {
			continue
		}
		visited[currentNode] = true

		if currentNode == targetID {
			return current.Nodes, nil
		}

		edges := a.graph.GetAllEdges()
		for _, edge := range edges {
			if edge.SourceID == currentNode {
				weight := weightFunc(edge)
				newDist := dist[currentNode] + weight

				if newDist < dist[edge.TargetID] {
					dist[edge.TargetID] = newDist
					prev[edge.TargetID] = currentNode

					newPath := make([]uuid.UUID, len(current.Nodes)+1)
					copy(newPath, current.Nodes)
					newPath[len(newPath)-1] = edge.TargetID

					heap.Push(&pq, &Path{Nodes: newPath, Cost: newDist})
				}
			}
		}
	}

	return nil, domain.ErrPathNotFound
}

// AnalyzeImpactWeighted performs weighted impact analysis considering service importance.
func (a *GraphAnalyzer) AnalyzeImpactWeighted(ctx context.Context, serviceID uuid.UUID, maxDepth int) (*domain.WeightedImpactResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(serviceID)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}

	upstreamVisited := make(map[uuid.UUID]int)
	downstreamVisited := make(map[uuid.UUID]int)

	a.bfsUpstream(serviceID, maxDepth, upstreamVisited)
	a.bfsDownstream(serviceID, maxDepth, downstreamVisited)

	result := &domain.WeightedImpactResult{
		ImpactResult: &domain.ImpactResult{
			RootService:     node,
			RootServiceID:   node.ID,
			RootServiceName: node.Name,
			Upstream:        make([]*domain.ImpactNode, 0),
			Downstream:      make([]*domain.ImpactNode, 0),
			CriticalPath:    make([]domain.PathHop, 0),
			AnalyzedAt:      time.Now(),
		},
		CriticalServices:   make([]*domain.ImpactNode, 0),
		ImpactByImportance: make(map[domain.ServiceImportance]int),
	}

	var weightedScore float64

	for id, depth := range upstreamVisited {
		if id == serviceID {
			continue
		}
		n := a.graph.GetNode(id)
		if n == nil {
			continue
		}

		nodeWeight := n.GetEffectiveWeight()
		impact := nodeWeight / float64(depth)
		weightedScore += impact

		impactNode := &domain.ImpactNode{
			Node:       n,
			Depth:      depth,
			Impact:     impact,
			IsCritical: n.Importance == domain.ImportanceCritical || n.Importance == domain.ImportanceImportant,
		}

		result.Upstream = append(result.Upstream, impactNode)
		result.ImpactByImportance[n.Importance]++

		if impactNode.IsCritical {
			result.CriticalServices = append(result.CriticalServices, impactNode)
		}
	}

	for id, depth := range downstreamVisited {
		if id == serviceID {
			continue
		}
		n := a.graph.GetNode(id)
		if n == nil {
			continue
		}

		nodeWeight := n.GetEffectiveWeight()
		impact := nodeWeight / float64(depth)
		weightedScore += impact

		impactNode := &domain.ImpactNode{
			Node:       n,
			Depth:      depth,
			Impact:     impact,
			IsCritical: n.Importance == domain.ImportanceCritical || n.Importance == domain.ImportanceImportant,
		}

		result.Downstream = append(result.Downstream, impactNode)
		result.ImpactByImportance[n.Importance]++

		if impactNode.IsCritical {
			result.CriticalServices = append(result.CriticalServices, impactNode)
		}
	}

	result.WeightedScore = math.Min(weightedScore*10, 100)
	result.TotalAffected = len(result.Upstream) + len(result.Downstream)
	if len(result.Upstream) > 0 {
		result.UpstreamDepth = result.Upstream[len(result.Upstream)-1].Depth
	}
	if len(result.Downstream) > 0 {
		result.DownstreamDepth = result.Downstream[len(result.Downstream)-1].Depth
	}

	return result, nil
}

// FindKShortestPaths finds k shortest paths using Yen's algorithm.
func (a *GraphAnalyzer) FindKShortestPaths(ctx context.Context, sourceID, targetID uuid.UUID, k int, weightFunc func(edge *domain.CallEdge) float64) ([]*domain.WeightedPathResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil || a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	if weightFunc == nil {
		weightFunc = func(edge *domain.CallEdge) float64 { return 1.0 }
	}

	firstPathNodes, err := a.FindShortestPathWeighted(ctx, sourceID, targetID, weightFunc)
	if err != nil {
		return nil, err
	}

	firstResult := a.buildWeightedPathResultFromIDs(sourceID, targetID, firstPathNodes, weightFunc)
	results := []*domain.WeightedPathResult{firstResult}

	if k == 1 {
		return results, nil
	}

	candidates := &PriorityQueue{}
	heap.Init(candidates)

	for i := 1; i < k && len(results) > 0; i++ {
		prevPathHops := results[i-1].ShortestPath
		if len(prevPathHops) == 0 {
			continue
		}

		for spurIndex := 0; spurIndex < len(prevPathHops)-1; spurIndex++ {
			rootPath := prevPathHops[:spurIndex+1]

			var removedEdgeKeys []string
			for _, prevResult := range results {
				prevP := prevResult.ShortestPath
				if len(prevP) > spurIndex {
					match := true
					for j := 0; j <= spurIndex; j++ {
						if prevP[j].NodeID != rootPath[j].NodeID {
							match = false
							break
						}
					}
					if match && len(prevP) > spurIndex {
						removedEdgeKeys = append(removedEdgeKeys, fmt.Sprintf("%s->%s", prevP[spurIndex].NodeID, prevP[spurIndex+1].NodeID))
					}
				}
			}

			var removedNodeIDs []uuid.UUID
			for j := 0; j < spurIndex; j++ {
				removedNodeIDs = append(removedNodeIDs, rootPath[j].NodeID)
			}

			spurPath, spurErr := a.findSpurPath(rootPath[spurIndex].NodeID, targetID, removedEdgeKeys, removedNodeIDs, weightFunc)
			if spurErr != nil {
				continue
			}

			fullPath := make([]domain.PathHop, len(rootPath)+len(spurPath)-1)
			copy(fullPath[:len(rootPath)], rootPath)
			copy(fullPath[len(rootPath):], spurPath[1:])

			totalWeight := a.calculatePathWeight(fullPath, weightFunc)

			isDuplicate := false
			for _, existing := range results {
				if a.pathsEqual(existing.ShortestPath, fullPath) {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				heap.Push(candidates, &Path{Nodes: pathHopIDs(fullPath), Cost: totalWeight})
			}
		}

		if candidates.Len() == 0 {
			break
		}

		best := heap.Pop(candidates).(*Path)
		result := a.buildWeightedPathResultFromIDs(sourceID, targetID, best.Nodes, weightFunc)
		results = append(results, result)
	}

	return results, nil
}

func (a *GraphAnalyzer) CalculateCentralityDetailed(ctx context.Context, nodeID uuid.UUID) (*CentralityResult, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(nodeID)
	if node == nil {
		return nil, domain.ErrNodeNotFound
	}

	result := &CentralityResult{
		NodeID: nodeID,
	}

	upstream := a.graph.GetUpstreamIDs(nodeID)
	downstream := a.graph.GetDownstreamIDs(nodeID)
	result.DegreeCentrality = float64(len(upstream) + len(downstream))

	result.BetweennessCentrality = a.calculateBetweennessCentrality(nodeID)

	// Closeness centrality: average reciprocal distance to all reachable nodes
	totalNodes := a.graph.GetAllNodes()
	reachableCount := 0
	totalDistance := 0.0
	for _, target := range totalNodes {
		if target.ID == nodeID {
			continue
		}
		visited := make(map[uuid.UUID]int)
		a.bfs(nodeID, 10, visited, a.graph.GetDownstreamIDs)
		if depth, ok := visited[target.ID]; ok {
			reachableCount++
			totalDistance += float64(depth)
		}
	}
	if reachableCount > 0 && totalDistance > 0 {
		result.ClosenessCentrality = float64(reachableCount) / totalDistance
	}

	result.WeightedCentrality = result.DegreeCentrality * node.GetEffectiveWeight()

	return result, nil
}

type CentralityResult struct {
	NodeID                uuid.UUID `json:"node_id"`
	DegreeCentrality      float64   `json:"degree_centrality"`
	ClosenessCentrality   float64   `json:"closeness_centrality"`
	BetweennessCentrality float64   `json:"betweenness_centrality"`
	WeightedCentrality    float64   `json:"weighted_centrality"`
}

func (a *GraphAnalyzer) FindCriticalPath(ctx context.Context, serviceID uuid.UUID, maxDepth int) ([]domain.PathHop, float64, error) {
	if a.graph == nil {
		return nil, 0, domain.ErrGraphNotReady
	}

	node := a.graph.GetNode(serviceID)
	if node == nil {
		return nil, 0, domain.ErrNodeNotFound
	}

	impactResult, err := a.AnalyzeImpact(ctx, serviceID, maxDepth)
	if err != nil {
		return nil, 0, err
	}

	allImpacted := impactNodeListByScore(impactResult)

	bestPath := make([]domain.PathHop, 0)
	bestScore := 0.0

	for _, impactNode := range allImpacted {
		paths := a.findAllPaths(serviceID, impactNode.Node.ID, maxDepth)
		for _, path := range paths {
			score := 0.0
			for _, hop := range path {
				n := a.graph.GetNode(hop.NodeID)
				if n != nil {
					score += n.GetEffectiveWeight()
				}
			}
			if score > bestScore {
				bestScore = score
				bestPath = path
			}
		}
		if bestScore > 0 {
			break
		}
	}

	if len(bestPath) == 0 {
		return nil, 0, domain.ErrPathNotFound
	}

	return bestPath, bestScore, nil
}

func impactNodeListByScore(result *domain.ImpactResult) []*domain.ImpactNode {
	all := append(result.Upstream, result.Downstream...)
	sortByImpact(all)
	return all
}

func sortByImpact(nodes []*domain.ImpactNode) {
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j].Impact > nodes[j-1].Impact; j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
}

func (a *GraphAnalyzer) DetectBottlenecks(ctx context.Context) ([]*BottleneckNode, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	var bottlenecks []*BottleneckNode

	nodes := a.graph.GetAllNodes()
	for _, node := range nodes {
		centrality, _ := a.CalculateCentralityDetailed(ctx, node.ID)

		score := centrality.WeightedCentrality * (1 + node.ErrorRate) * (1 + node.LatencyP99/1000)

		if score > 1.0 {
			bottlenecks = append(bottlenecks, &BottleneckNode{
				Node:    node,
				Score:   score,
				Reasons: identifyBottleneckReasons(node, centrality),
			})
		}
	}

	return bottlenecks, nil
}

type BottleneckNode struct {
	Node    *domain.ServiceNode `json:"node"`
	Score   float64             `json:"score"`
	Reasons []string            `json:"reasons"`
}

func identifyBottleneckReasons(node *domain.ServiceNode, centrality *CentralityResult) []string {
	var reasons []string

	if centrality.BetweennessCentrality > 10 {
		reasons = append(reasons, "high_betweenness")
	}
	if node.RequestRate > 1000 {
		reasons = append(reasons, "high_traffic")
	}
	if node.LatencyP99 > 500 {
		reasons = append(reasons, "high_latency")
	}
	if node.ErrorRate > 0.01 {
		reasons = append(reasons, "high_error_rate")
	}

	return reasons
}

// FindShortestPathAStar finds the shortest path using A* algorithm with a heuristic function.
func (a *GraphAnalyzer) FindShortestPathAStar(ctx context.Context, sourceID, targetID uuid.UUID, weightFunc func(edge *domain.CallEdge) float64, heuristic func(uuid.UUID) float64) ([]uuid.UUID, error) {
	if a.graph == nil {
		return nil, domain.ErrGraphNotReady
	}

	if a.graph.GetNode(sourceID) == nil || a.graph.GetNode(targetID) == nil {
		return nil, domain.ErrNodeNotFound
	}

	if weightFunc == nil {
		weightFunc = func(edge *domain.CallEdge) float64 { return 1.0 }
	}

	gScore := make(map[uuid.UUID]float64)
	for _, node := range a.graph.GetAllNodes() {
		gScore[node.ID] = math.Inf(1)
	}
	gScore[sourceID] = 0

	openSet := &PriorityQueue{}
	heap.Init(openSet)
	heap.Push(openSet, &Path{Nodes: []uuid.UUID{sourceID}, Cost: heuristic(sourceID)})

	closedSet := make(map[uuid.UUID]bool)

	for openSet.Len() > 0 {
		current := heap.Pop(openSet).(*Path)
		currentNode := current.Nodes[len(current.Nodes)-1]

		if currentNode == targetID {
			return current.Nodes, nil
		}

		if closedSet[currentNode] {
			continue
		}
		closedSet[currentNode] = true

		edges := a.graph.GetAllEdges()
		for _, edge := range edges {
			if edge.SourceID != currentNode {
				continue
			}

			tentativeG := gScore[currentNode] + weightFunc(edge)
			if tentativeG < gScore[edge.TargetID] {
				gScore[edge.TargetID] = tentativeG
				fScore := tentativeG + heuristic(edge.TargetID)

				newPath := make([]uuid.UUID, len(current.Nodes)+1)
				copy(newPath, current.Nodes)
				newPath[len(newPath)-1] = edge.TargetID

				heap.Push(openSet, &Path{Nodes: newPath, Cost: fScore})
			}
		}
	}

	return nil, domain.ErrPathNotFound
}

func (a *GraphAnalyzer) DefaultHeuristic(targetID uuid.UUID, weightFunc func(edge *domain.CallEdge) float64) func(uuid.UUID) float64 {
	if weightFunc == nil {
		weightFunc = func(edge *domain.CallEdge) float64 { return 1.0 }
	}
	minWeight := 1.0
	edges := a.graph.GetAllEdges()
	for _, edge := range edges {
		w := weightFunc(edge)
		if w > 0 && w < minWeight {
			minWeight = w
		}
	}
	return func(nodeID uuid.UUID) float64 {
		return minWeight
	}
}

type TimeDependentWeight struct {
	BaseWeight  float64
	TimeFactors map[time.Weekday]map[int]float64
}

func (tdw *TimeDependentWeight) GetWeight(t time.Time) float64 {
	weekday := t.Weekday()
	hour := t.Hour()

	if dayFactors, ok := tdw.TimeFactors[weekday]; ok {
		if factor, ok := dayFactors[hour]; ok {
			return tdw.BaseWeight * factor
		}
	}

	return tdw.BaseWeight
}

// Helper methods for Yen's algorithm and weighted path building.

func (a *GraphAnalyzer) buildWeightedPathResultFromIDs(sourceID, targetID uuid.UUID, nodeIDs []uuid.UUID, weightFunc func(edge *domain.CallEdge) float64) *domain.WeightedPathResult {
	pathHops := a.buildPathHopsFromIDs(nodeIDs)
	totalWeight := 0.0
	maxErrorRate := 0.0
	totalLatency := 0.0

	for i := 0; i < len(nodeIDs)-1; i++ {
		edge := a.graph.GetEdge(nodeIDs[i], nodeIDs[i+1])
		if edge != nil {
			totalWeight += weightFunc(edge)
			if edge.ErrorRate > maxErrorRate {
				maxErrorRate = edge.ErrorRate
			}
			totalLatency += edge.LatencyP99
		} else {
			totalWeight += 1.0
		}
	}

	avgLatency := 0.0
	if len(pathHops) > 1 {
		avgLatency = totalLatency / float64(len(pathHops)-1)
	}

	sourceName := ""
	targetName := ""
	sn := a.graph.GetNode(sourceID)
	if sn != nil {
		sourceName = sn.Name
	}
	tn := a.graph.GetNode(targetID)
	if tn != nil {
		targetName = tn.Name
	}

	return &domain.WeightedPathResult{
		PathResult: &domain.PathResult{
			SourceID:     sourceID,
			TargetID:     targetID,
			SourceName:   sourceName,
			TargetName:   targetName,
			Paths:        [][]domain.PathHop{pathHops},
			ShortestPath: pathHops,
			ShortestHops: len(pathHops),
			FoundAt:      time.Now(),
		},
		TotalWeight:    totalWeight,
		AverageLatency: avgLatency,
		MaxErrorRate:   maxErrorRate,
	}
}

func (a *GraphAnalyzer) buildPathHopsFromIDs(nodeIDs []uuid.UUID) []domain.PathHop {
	hops := make([]domain.PathHop, len(nodeIDs))
	for i, id := range nodeIDs {
		node := a.graph.GetNode(id)
		if node != nil {
			hops[i] = domain.PathHop{
				NodeID:    id,
				NodeName:  node.Name,
				Namespace: node.Namespace,
			}
		} else {
			hops[i] = domain.PathHop{NodeID: id}
		}
	}
	return hops
}

func (a *GraphAnalyzer) calculatePathWeight(hops []domain.PathHop, weightFunc func(edge *domain.CallEdge) float64) float64 {
	total := 0.0
	for i := 0; i < len(hops)-1; i++ {
		edge := a.graph.GetEdge(hops[i].NodeID, hops[i+1].NodeID)
		if edge != nil {
			total += weightFunc(edge)
		} else {
			total += 1.0
		}
	}
	return total
}

func (a *GraphAnalyzer) pathsEqual(aPath, bPath []domain.PathHop) bool {
	if len(aPath) != len(bPath) {
		return false
	}
	for i := range aPath {
		if aPath[i].NodeID != bPath[i].NodeID {
			return false
		}
	}
	return true
}

func pathHopIDs(hops []domain.PathHop) []uuid.UUID {
	ids := make([]uuid.UUID, len(hops))
	for i, hop := range hops {
		ids[i] = hop.NodeID
	}
	return ids
}

// findSpurPath finds a shortest path from spurNode to target, excluding specified edges and nodes.
func (a *GraphAnalyzer) findSpurPath(spurNodeID, targetID uuid.UUID, removedEdgeKeys []string, removedNodeIDs []uuid.UUID, weightFunc func(edge *domain.CallEdge) float64) ([]domain.PathHop, error) {
	removedEdgeSet := make(map[string]bool, len(removedEdgeKeys))
	for _, k := range removedEdgeKeys {
		removedEdgeSet[k] = true
	}

	removedNodeSet := make(map[uuid.UUID]bool, len(removedNodeIDs))
	for _, id := range removedNodeIDs {
		removedNodeSet[id] = true
	}

	parent := make(map[uuid.UUID]uuid.UUID)
	visited := make(map[uuid.UUID]bool)
	queue := []uuid.UUID{spurNodeID}
	visited[spurNodeID] = true

	found := false
	for len(queue) > 0 && !found {
		current := queue[0]
		queue = queue[1:]

		if current == targetID {
			found = true
			break
		}

		edges := a.graph.GetAllEdges()
		for _, edge := range edges {
			if edge.SourceID != current {
				continue
			}

			edgeKey := fmt.Sprintf("%s->%s", edge.SourceID, edge.TargetID)
			if removedEdgeSet[edgeKey] {
				continue
			}

			if removedNodeSet[edge.TargetID] {
				continue
			}

			if !visited[edge.TargetID] {
				visited[edge.TargetID] = true
				parent[edge.TargetID] = current
				queue = append(queue, edge.TargetID)
			}
		}
	}

	if !found {
		return nil, domain.ErrPathNotFound
	}

	var path []domain.PathHop
	current := targetID
	for current != spurNodeID {
		node := a.graph.GetNode(current)
		if node != nil {
			path = append([]domain.PathHop{{
				NodeID:    current,
				NodeName:  node.Name,
				Namespace: node.Namespace,
			}}, path...)
		} else {
			path = append([]domain.PathHop{{NodeID: current}}, path...)
		}
		current = parent[current]
	}

	spurNode := a.graph.GetNode(spurNodeID)
	if spurNode != nil {
		path = append([]domain.PathHop{{
			NodeID:    spurNodeID,
			NodeName:  spurNode.Name,
			Namespace: spurNode.Namespace,
		}}, path...)
	} else {
		path = append([]domain.PathHop{{NodeID: spurNodeID}}, path...)
	}

	return path, nil
}
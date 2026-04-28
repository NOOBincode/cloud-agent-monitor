package domain

// TopologyOTelAttribute defines typed string constants for topology OTel span attributes.
type TopologyOTelAttribute string

const (
	AttrTopologyOperation    TopologyOTelAttribute = "topology.operation.name"
	AttrTopologyBackend      TopologyOTelAttribute = "topology.discovery.backend"
	AttrTopologyNodeCount    TopologyOTelAttribute = "topology.graph.node_count"
	AttrTopologyEdgeCount    TopologyOTelAttribute = "topology.graph.edge_count"
	AttrTopologyNamespace    TopologyOTelAttribute = "topology.namespace"
	AttrTopologyServiceName  TopologyOTelAttribute = "topology.service.name"
	AttrTopologyQueryType    TopologyOTelAttribute = "topology.query.type"
	AttrTopologyAnomalyType  TopologyOTelAttribute = "topology.anomaly.type"
	AttrTopologyTotalAffected TopologyOTelAttribute = "topology.impact.total_affected"
	AttrTopologyResult       TopologyOTelAttribute = "topology.result"
	AttrResilienceOperation  TopologyOTelAttribute = "topology.resilience.operation"
	AttrResilienceResult     TopologyOTelAttribute = "topology.resilience.result"
	AttrResilienceAttempt    TopologyOTelAttribute = "topology.resilience.attempt"
	AttrCBState              TopologyOTelAttribute = "topology.circuit_breaker.state"
	AttrBulkheadConcurrency  TopologyOTelAttribute = "topology.bulkhead.concurrency"
)

// TopologyOTelMetric defines typed string constants for topology OTel metric names.
type TopologyOTelMetric string

const (
	MetricTopologyDiscoveryDuration   TopologyOTelMetric = "topology.client.discovery.duration"
	MetricTopologyDiscoveryNodes      TopologyOTelMetric = "topology.client.discovery.nodes"
	MetricTopologyDiscoveryEdges      TopologyOTelMetric = "topology.client.discovery.edges"
	MetricTopologyQueryDuration       TopologyOTelMetric = "topology.client.query.duration"
	MetricTopologyImpactAnalysis      TopologyOTelMetric = "topology.client.impact.analysis.duration"
	MetricTopologyGraphRebuild        TopologyOTelMetric = "topology.client.graph.rebuild.duration"
	MetricTopologyCacheOperation      TopologyOTelMetric = "topology.client.cache.operation.duration"
	MetricResilienceRetryAttempts     TopologyOTelMetric = "topology.resilience.retry.attempts"
	MetricResilienceCBStateChange     TopologyOTelMetric = "topology.resilience.circuit_breaker.state_changes"
	MetricResilienceBulkheadActive    TopologyOTelMetric = "topology.resilience.bulkhead.active_calls"
)
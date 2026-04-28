package otel

import (
	"context"
	"fmt"

	"cloud-agent-monitor/internal/topology/domain"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "cloud-agent-monitor/topology"

var (
	discoveryDuration      metric.Float64Histogram
	discoveryNodesCounter  metric.Int64Counter
	discoveryEdgesCounter  metric.Int64Counter
	queryDuration          metric.Float64Histogram
	impactAnalysisDuration metric.Float64Histogram
	graphRebuildDuration   metric.Float64Histogram
	cacheOpDuration        metric.Float64Histogram
	retryAttemptsCounter   metric.Int64Counter
	cbStateChangeCounter   metric.Int64Counter
	bulkheadActiveCalls    metric.Int64UpDownCounter
)

type MetricsCollector struct {
	meter metric.Meter
}

func NewMetricsCollector() (*MetricsCollector, error) {
	return newMetricsCollectorWithMeter(otel.Meter(meterName))
}

func newMetricsCollectorWithMeter(meter metric.Meter) (*MetricsCollector, error) {
	mc := &MetricsCollector{meter: meter}

	var err error

	discoveryDuration, err = meter.Float64Histogram(string(domain.MetricTopologyDiscoveryDuration),
		metric.WithDescription("Duration of topology discovery operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	discoveryNodesCounter, err = meter.Int64Counter(string(domain.MetricTopologyDiscoveryNodes),
		metric.WithDescription("Number of nodes discovered by topology discovery"),
	)
	if err != nil {
		return nil, err
	}

	discoveryEdgesCounter, err = meter.Int64Counter(string(domain.MetricTopologyDiscoveryEdges),
		metric.WithDescription("Number of edges discovered by topology discovery"),
	)
	if err != nil {
		return nil, err
	}

	queryDuration, err = meter.Float64Histogram(string(domain.MetricTopologyQueryDuration),
		metric.WithDescription("Duration of topology query operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	impactAnalysisDuration, err = meter.Float64Histogram(string(domain.MetricTopologyImpactAnalysis),
		metric.WithDescription("Duration of impact analysis operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	graphRebuildDuration, err = meter.Float64Histogram(string(domain.MetricTopologyGraphRebuild),
		metric.WithDescription("Duration of graph rebuild operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	cacheOpDuration, err = meter.Float64Histogram(string(domain.MetricTopologyCacheOperation),
		metric.WithDescription("Duration of topology cache operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	retryAttemptsCounter, err = meter.Int64Counter(string(domain.MetricResilienceRetryAttempts),
		metric.WithDescription("Number of resilience retry attempts"),
	)
	if err != nil {
		return nil, err
	}

	cbStateChangeCounter, err = meter.Int64Counter(string(domain.MetricResilienceCBStateChange),
		metric.WithDescription("Number of circuit breaker state transitions"),
	)
	if err != nil {
		return nil, err
	}

	bulkheadActiveCalls, err = meter.Int64UpDownCounter(string(domain.MetricResilienceBulkheadActive),
		metric.WithDescription("Number of currently active calls in the bulkhead"),
	)
	if err != nil {
		return nil, err
	}

	return mc, nil
}

func RecordDiscoveryDuration(ctx context.Context, backend, operation string, durationMs float64) {
	if discoveryDuration == nil {
		return
	}
	discoveryDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String(string(domain.AttrTopologyBackend), backend),
			attribute.String(string(domain.AttrTopologyOperation), operation),
		),
	)
}

func RecordDiscoveryNodes(ctx context.Context, backend string, count int64) {
	if discoveryNodesCounter == nil {
		return
	}
	discoveryNodesCounter.Add(ctx, count,
		metric.WithAttributes(
			attribute.String(string(domain.AttrTopologyBackend), backend),
		),
	)
}

func RecordDiscoveryEdges(ctx context.Context, backend string, count int64) {
	if discoveryEdgesCounter == nil {
		return
	}
	discoveryEdgesCounter.Add(ctx, count,
		metric.WithAttributes(
			attribute.String(string(domain.AttrTopologyBackend), backend),
		),
	)
}

func RecordQueryDuration(ctx context.Context, queryType, namespace string, durationMs float64) {
	if queryDuration == nil {
		return
	}
	queryDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String(string(domain.AttrTopologyQueryType), queryType),
			attribute.String(string(domain.AttrTopologyNamespace), namespace),
		),
	)
}

func RecordImpactAnalysis(ctx context.Context, serviceName string, totalAffected int, durationMs float64) {
	if impactAnalysisDuration == nil {
		return
	}
	impactAnalysisDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String(string(domain.AttrTopologyServiceName), serviceName),
			attribute.Int(string(domain.AttrTopologyTotalAffected), totalAffected),
		),
	)
}

func RecordGraphRebuild(ctx context.Context, nodeCount, edgeCount int, durationMs float64) {
	if graphRebuildDuration == nil {
		return
	}
	graphRebuildDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.Int(string(domain.AttrTopologyNodeCount), nodeCount),
			attribute.Int(string(domain.AttrTopologyEdgeCount), edgeCount),
		),
	)
}

func RecordCacheOperation(ctx context.Context, operation, result string, durationMs float64) {
	if cacheOpDuration == nil {
		return
	}
	cacheOpDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String(string(domain.AttrTopologyOperation), fmt.Sprintf("cache_%s", operation)),
			attribute.String(string(domain.AttrTopologyResult), result),
		),
	)
}

func RecordRetryAttempt(ctx context.Context, operation, result string, attempt int) {
	if retryAttemptsCounter == nil {
		return
	}
	retryAttemptsCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(string(domain.AttrResilienceOperation), operation),
			attribute.String(string(domain.AttrResilienceResult), result),
			attribute.Int(string(domain.AttrResilienceAttempt), attempt),
		),
	)
}

func RecordCBStateChange(ctx context.Context, fromState, toState string) {
	if cbStateChangeCounter == nil {
		return
	}
	cbStateChangeCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("from_state", fromState),
			attribute.String("to_state", toState),
		),
	)
}

func RecordBulkheadActiveChange(ctx context.Context, delta int64, concurrency int) {
	if bulkheadActiveCalls == nil {
		return
	}
	bulkheadActiveCalls.Add(ctx, delta,
		metric.WithAttributes(
			attribute.Int(string(domain.AttrBulkheadConcurrency), concurrency),
		),
	)
}
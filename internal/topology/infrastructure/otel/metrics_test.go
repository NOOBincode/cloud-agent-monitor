package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewMetricsCollector(t *testing.T) {
	t.Run("creates collector without error", func(t *testing.T) {
		mc, err := NewMetricsCollector()
		require.NoError(t, err)
		require.NotNil(t, mc)
	})
}

func TestNewMetricsCollectorWithSDKProvider(t *testing.T) {
	t.Run("creates collector and records metrics via SDK", func(t *testing.T) {
		reader := sdkmetric.NewManualReader()
		mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		meter := mp.Meter(meterName)

		mc, err := newMetricsCollectorWithMeter(meter)
		require.NoError(t, err)
		require.NotNil(t, mc)

		ctx := context.Background()
		RecordDiscoveryDuration(ctx, "k8s", "refresh", 150.0)
		RecordDiscoveryNodes(ctx, "k8s", 20)
		RecordDiscoveryEdges(ctx, "k8s", 10)
		RecordQueryDuration(ctx, "impact", "default", 25.0)
		RecordImpactAnalysis(ctx, "svc", 10, 200.0)
		RecordGraphRebuild(ctx, 50, 30, 150.0)
		RecordCacheOperation(ctx, "get", "hit", 5.0)
		RecordRetryAttempt(ctx, "cache", "timeout", 3)
		RecordCBStateChange(ctx, "closed", "open")
		RecordBulkheadActiveChange(ctx, 1, 10)

		var rm metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(ctx, &rm))
		assert.NotEmpty(t, rm.ScopeMetrics)
		mp.Shutdown(ctx)
	})
}

func TestRecordFunctionsNilGuard(t *testing.T) {
	ctx := context.Background()

	t.Run("RecordDiscoveryDuration nil guard", func(t *testing.T) {
		discoveryDuration = nil
		RecordDiscoveryDuration(ctx, "test", "op", 100.0)
	})
	t.Run("RecordDiscoveryNodes nil guard", func(t *testing.T) {
		discoveryNodesCounter = nil
		RecordDiscoveryNodes(ctx, "test", 10)
	})
	t.Run("RecordDiscoveryEdges nil guard", func(t *testing.T) {
		discoveryEdgesCounter = nil
		RecordDiscoveryEdges(ctx, "test", 5)
	})
	t.Run("RecordQueryDuration nil guard", func(t *testing.T) {
		queryDuration = nil
		RecordQueryDuration(ctx, "service_topology", "default", 50.0)
	})
	t.Run("RecordImpactAnalysis nil guard", func(t *testing.T) {
		impactAnalysisDuration = nil
		RecordImpactAnalysis(ctx, "svc", 10, 200.0)
	})
	t.Run("RecordGraphRebuild nil guard", func(t *testing.T) {
		graphRebuildDuration = nil
		RecordGraphRebuild(ctx, 20, 10, 300.0)
	})
	t.Run("RecordCacheOperation nil guard", func(t *testing.T) {
		cacheOpDuration = nil
		RecordCacheOperation(ctx, "get", "hit", 10.0)
	})
	t.Run("RecordRetryAttempt nil guard", func(t *testing.T) {
		retryAttemptsCounter = nil
		RecordRetryAttempt(ctx, "cache", "error", 2)
	})
	t.Run("RecordCBStateChange nil guard", func(t *testing.T) {
		cbStateChangeCounter = nil
		RecordCBStateChange(ctx, "closed", "open")
	})
	t.Run("RecordBulkheadActiveChange nil guard", func(t *testing.T) {
		bulkheadActiveCalls = nil
		RecordBulkheadActiveChange(ctx, 1, 5)
	})
}

func TestRecordFunctionsAfterInit(t *testing.T) {
	t.Run("all Record functions work after MetricsCollector init", func(t *testing.T) {
		mc, err := NewMetricsCollector()
		require.NoError(t, err)
		require.NotNil(t, mc)

		ctx := context.Background()
		RecordDiscoveryDuration(ctx, "service", "refresh", 100.0)
		RecordDiscoveryNodes(ctx, "service", 50)
		RecordDiscoveryEdges(ctx, "service", 30)
		RecordQueryDuration(ctx, "service_topology", "default", 25.0)
		RecordImpactAnalysis(ctx, "svc", 10, 200.0)
		RecordGraphRebuild(ctx, 50, 30, 150.0)
		RecordCacheOperation(ctx, "get", "hit", 5.0)
		RecordRetryAttempt(ctx, "cache", "timeout", 3)
		RecordCBStateChange(ctx, "closed", "open")
		RecordBulkheadActiveChange(ctx, 1, 10)

		assert.NotNil(t, discoveryDuration)
		assert.NotNil(t, discoveryNodesCounter)
		assert.NotNil(t, discoveryEdgesCounter)
	})
}
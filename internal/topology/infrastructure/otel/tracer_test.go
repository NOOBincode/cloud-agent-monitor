package otel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func setupTestTracerProvider() (*sdktrace.TracerProvider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(tp)
	return tp, recorder
}

func TestTracer(t *testing.T) {
	t.Run("returns non-nil tracer", func(t *testing.T) {
		tr := Tracer()
		assert.NotNil(t, tr)
	})
}

func TestStartDiscoverySpan(t *testing.T) {
	t.Run("creates span with correct name", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		_, span := StartDiscoverySpan(context.Background(), "kubernetes", "refresh_service")
		span.End()

		require.NoError(t, tp.ForceFlush(context.Background()))
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Contains(t, spans[0].Name(), "topology.discovery")
	})
}

func TestStartQuerySpan(t *testing.T) {
	t.Run("creates span with query type", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartQuerySpan(context.Background(), "service_topology")
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Contains(t, spans[0].Name(), "topology.query")
	})
}

func TestStartImpactAnalysisSpan(t *testing.T) {
	t.Run("creates span with service name", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartImpactAnalysisSpan(context.Background(), "my-service")
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Contains(t, spans[0].Name(), "topology.impact")
	})
}

func TestStartAnomalyDetectionSpan(t *testing.T) {
	t.Run("creates span", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartAnomalyDetectionSpan(context.Background())
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Equal(t, "topology.anomaly_detection", spans[0].Name())
	})
}

func TestStartCacheOperationSpan(t *testing.T) {
	t.Run("creates span with operation name", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartCacheOperationSpan(context.Background(), "get")
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Contains(t, spans[0].Name(), "topology.cache")
	})
}

func TestStartResilienceSpan(t *testing.T) {
	t.Run("creates span with operation name", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartResilienceSpan(context.Background(), "retry")
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Contains(t, spans[0].Name(), "topology.resilience")
	})
}

func TestStartGraphRebuildSpan(t *testing.T) {
	t.Run("creates span", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartGraphRebuildSpan(context.Background())
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Equal(t, "topology.graph_rebuild", spans[0].Name())
	})
}

func TestRecordErrorAndSetSpanSuccess(t *testing.T) {
	t.Run("RecordError sets error status", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartDiscoverySpan(context.Background(), "test", "op")
		RecordError(span, context.DeadlineExceeded)
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Equal(t, "Error", spans[0].Status().Code.String())
	})

	t.Run("SetSpanSuccess sets OK status", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartDiscoverySpan(context.Background(), "test", "op")
		SetSpanSuccess(span)
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Equal(t, "Ok", spans[0].Status().Code.String())
	})
}

func TestSetDiscoveryAttrs(t *testing.T) {
	t.Run("sets backend and counts on top of initial attrs", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		_, span := StartDiscoverySpan(context.Background(), "test", "op")
		SetDiscoveryAttrs(span, "prometheus", 10, 5)
		span.End()

		require.NoError(t, tp.ForceFlush(context.Background()))
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		// StartDiscoverySpan sets 2 attrs (backend, operation), SetDiscoveryAttrs adds 2 new (nodeCount, edgeCount), backend overlaps
		assert.Len(t, spans[0].Attributes(), 4)
	})
}

func TestSetQueryAttrs(t *testing.T) {
	t.Run("sets query type and namespace", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartQuerySpan(context.Background(), "service")
		SetQueryAttrs(span, "service_topology", "default")
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Len(t, spans[0].Attributes(), 3)
	})
}

func TestSetImpactAttrs(t *testing.T) {
	t.Run("sets service name and total affected", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartImpactAnalysisSpan(context.Background(), "svc")
		SetImpactAttrs(span, "svc", 15)
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Len(t, spans[0].Attributes(), 3)
	})
}

func TestSetResilienceAttrs(t *testing.T) {
	t.Run("sets operation, result and attempt on top of initial attrs", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		_, span := StartResilienceSpan(context.Background(), "retry")
		SetResilienceAttrs(span, "cache_get", "error", 3)
		span.End()

		require.NoError(t, tp.ForceFlush(context.Background()))
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		// StartResilienceSpan sets 1 attr (operation), SetResilienceAttrs adds 2 new (result, attempt), operation overlaps
		assert.Len(t, spans[0].Attributes(), 3)
	})
}

func TestSetCBAttrs(t *testing.T) {
	t.Run("sets circuit breaker state", func(t *testing.T) {
		tp, recorder := setupTestTracerProvider()
		defer otel.SetTracerProvider(nil)

		ctx, span := StartResilienceSpan(context.Background(), "cb")
		SetCBAttrs(span, "open")
		span.End()

		tp.ForceFlush(ctx)
		spans := recorder.Ended()
		require.Len(t, spans, 1)
		assert.Len(t, spans[0].Attributes(), 2)
	})
}
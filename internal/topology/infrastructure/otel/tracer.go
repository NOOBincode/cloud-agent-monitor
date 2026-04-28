package otel

import (
	"context"
	"fmt"

	"cloud-agent-monitor/internal/topology/domain"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "cloud-agent-monitor/topology"

func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

func StartDiscoverySpan(ctx context.Context, backend, operation string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("topology.discovery %s", backend),
		trace.WithAttributes(
			attribute.String(string(domain.AttrTopologyBackend), backend),
			attribute.String(string(domain.AttrTopologyOperation), operation),
		),
	)
	return ctx, span
}

func StartQuerySpan(ctx context.Context, queryType string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("topology.query %s", queryType),
		trace.WithAttributes(
			attribute.String(string(domain.AttrTopologyQueryType), queryType),
			attribute.String(string(domain.AttrTopologyOperation), "query"),
		),
	)
	return ctx, span
}

func StartImpactAnalysisSpan(ctx context.Context, serviceName string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("topology.impact %s", serviceName),
		trace.WithAttributes(
			attribute.String(string(domain.AttrTopologyServiceName), serviceName),
			attribute.String(string(domain.AttrTopologyOperation), "impact_analysis"),
		),
	)
	return ctx, span
}

func StartAnomalyDetectionSpan(ctx context.Context) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, "topology.anomaly_detection",
		trace.WithAttributes(
			attribute.String(string(domain.AttrTopologyOperation), "anomaly_detection"),
		),
	)
	return ctx, span
}

func StartCacheOperationSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("topology.cache %s", operation),
		trace.WithAttributes(
			attribute.String(string(domain.AttrTopologyOperation), fmt.Sprintf("cache_%s", operation)),
		),
	)
	return ctx, span
}

func StartResilienceSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("topology.resilience %s", operation),
		trace.WithAttributes(
			attribute.String(string(domain.AttrResilienceOperation), operation),
		),
	)
	return ctx, span
}

func StartGraphRebuildSpan(ctx context.Context) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, "topology.graph_rebuild",
		trace.WithAttributes(
			attribute.String(string(domain.AttrTopologyOperation), "graph_rebuild"),
		),
	)
	return ctx, span
}

func SetDiscoveryAttrs(span trace.Span, backend string, nodeCount, edgeCount int) {
	span.SetAttributes(
		attribute.String(string(domain.AttrTopologyBackend), backend),
		attribute.Int(string(domain.AttrTopologyNodeCount), nodeCount),
		attribute.Int(string(domain.AttrTopologyEdgeCount), edgeCount),
	)
}

func SetQueryAttrs(span trace.Span, queryType, namespace string) {
	span.SetAttributes(
		attribute.String(string(domain.AttrTopologyQueryType), queryType),
		attribute.String(string(domain.AttrTopologyNamespace), namespace),
	)
}

func SetImpactAttrs(span trace.Span, serviceName string, totalAffected int) {
	span.SetAttributes(
		attribute.String(string(domain.AttrTopologyServiceName), serviceName),
		attribute.Int(string(domain.AttrTopologyTotalAffected), totalAffected),
	)
}

func SetResilienceAttrs(span trace.Span, operation, result string, attempt int) {
	span.SetAttributes(
		attribute.String(string(domain.AttrResilienceOperation), operation),
		attribute.String(string(domain.AttrResilienceResult), result),
		attribute.Int(string(domain.AttrResilienceAttempt), attempt),
	)
}

func SetCBAttrs(span trace.Span, state string) {
	span.SetAttributes(
		attribute.String(string(domain.AttrCBState), state),
	)
}

func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func SetSpanSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}
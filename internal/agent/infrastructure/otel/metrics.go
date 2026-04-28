package otel

import (
	"context"

	aidomain "cloud-agent-monitor/internal/aiinfra/domain"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "cloud-agent-monitor/agent"

var (
	tokenUsageCounter       metric.Int64Counter
	operationDuration       metric.Float64Histogram
	toolCallDuration        metric.Float64Histogram
	toolCallCounter         metric.Int64Counter
	sessionCounter          metric.Int64Counter
)

type MetricsCollector struct {
	meter metric.Meter
}

func NewMetricsCollector() (*MetricsCollector, error) {
	meter := otel.Meter(meterName)
	mc := &MetricsCollector{meter: meter}

	var err error

	tokenUsageCounter, err = meter.Int64Counter(string(aidomain.MetricGenAIClientTokenUsage),
		metric.WithDescription("Number of tokens used by GenAI operations"),
	)
	if err != nil {
		return nil, err
	}

	operationDuration, err = meter.Float64Histogram(string(aidomain.MetricGenAIClientOperationDuration),
		metric.WithDescription("Duration of GenAI client operations in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	toolCallDuration, err = meter.Float64Histogram("gen_ai.client.tool.duration",
		metric.WithDescription("Duration of tool calls in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	toolCallCounter, err = meter.Int64Counter("gen_ai.client.tool.calls",
		metric.WithDescription("Number of tool calls made"),
	)
	if err != nil {
		return nil, err
	}

	sessionCounter, err = meter.Int64Counter("gen_ai.client.sessions",
		metric.WithDescription("Number of GenAI sessions"),
	)
	if err != nil {
		return nil, err
	}

	return mc, nil
}

func RecordTokenUsage(ctx context.Context, system, model string, inputTokens, outputTokens int64) {
	if tokenUsageCounter == nil {
		return
	}
	opts := []metric.AddOption{
		metric.WithAttributes(
			attribute.String(string(aidomain.AttrGenAISystem), system),
			attribute.String(string(aidomain.AttrGenAIRequestModel), model),
			attribute.String("token_type", "input"),
		),
	}
	tokenUsageCounter.Add(ctx, inputTokens, opts...)

	opts[0] = metric.WithAttributes(
		attribute.String(string(aidomain.AttrGenAISystem), system),
		attribute.String(string(aidomain.AttrGenAIRequestModel), model),
		attribute.String("token_type", "output"),
	)
	tokenUsageCounter.Add(ctx, outputTokens, opts...)
}

func RecordOperationDuration(ctx context.Context, system, model, operation string, durationMs float64) {
	if operationDuration == nil {
		return
	}
	operationDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String(string(aidomain.AttrGenAISystem), system),
			attribute.String(string(aidomain.AttrGenAIRequestModel), model),
			attribute.String(string(aidomain.AttrGenAIOperationName), operation),
		),
	)
}

func RecordToolCallDuration(ctx context.Context, toolName, toolType string, durationMs float64) {
	if toolCallDuration == nil {
		return
	}
	toolCallDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String(string(aidomain.AttrGenAIToolName), toolName),
			attribute.String(string(aidomain.AttrGenAIToolType), toolType),
		),
	)
}

func RecordToolCall(ctx context.Context, toolName, toolType, status string) {
	if toolCallCounter == nil {
		return
	}
	toolCallCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(string(aidomain.AttrGenAIToolName), toolName),
			attribute.String(string(aidomain.AttrGenAIToolType), toolType),
			attribute.String("status", status),
		),
	)
}

func RecordSession(ctx context.Context, system, operation, status string) {
	if sessionCounter == nil {
		return
	}
	sessionCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(string(aidomain.AttrGenAISystem), system),
			attribute.String(string(aidomain.AttrGenAIOperationName), operation),
			attribute.String("status", status),
		),
	)
}

package otel

import (
	"context"
	"fmt"

	aidomain "cloud-agent-monitor/internal/aiinfra/domain"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "cloud-agent-monitor/agent"
)

func Tracer() trace.Tracer {
	return otel.Tracer(tracerName)
}

func StartChatModelSpan(ctx context.Context, system, model, operation string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("gen_ai.%s %s", operation, model),
		trace.WithAttributes(
			attribute.String(string(aidomain.AttrGenAISystem), system),
			attribute.String(string(aidomain.AttrGenAIOperationName), operation),
			attribute.String(string(aidomain.AttrGenAIRequestModel), model),
		),
	)
	return ctx, span
}

func StartToolCallSpan(ctx context.Context, toolName, toolType string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("gen_ai.tool %s", toolName),
		trace.WithAttributes(
			attribute.String(string(aidomain.AttrGenAIToolName), toolName),
			attribute.String(string(aidomain.AttrGenAIToolType), toolType),
			attribute.String(string(aidomain.AttrGenAIOperationName), "execute_tool"),
		),
	)
	return ctx, span
}

func StartAgentSpan(ctx context.Context, agentName string) (context.Context, trace.Span) {
	ctx, span := Tracer().Start(ctx, fmt.Sprintf("agent.run %s", agentName),
		trace.WithAttributes(
			attribute.String("agent.name", agentName),
		),
	)
	return ctx, span
}

func SetChatModelRequestAttrs(span trace.Span, maxTokens int, temperature, topP float64) {
	span.SetAttributes(
		attribute.Int(string(aidomain.AttrGenAIRequestMaxTokens), maxTokens),
		attribute.Float64(string(aidomain.AttrGenAIRequestTemperature), temperature),
		attribute.Float64(string(aidomain.AttrGenAIRequestTopP), topP),
	)
}

func SetChatModelResponseAttrs(span trace.Span, model string, inputTokens, outputTokens int, finishReasons string) {
	span.SetAttributes(
		attribute.String(string(aidomain.AttrGenAIResponseModel), model),
		attribute.String(string(aidomain.AttrGenAIResponseFinishReasons), finishReasons),
		attribute.Int(string(aidomain.AttrGenAIUsageInputTokens), inputTokens),
		attribute.Int(string(aidomain.AttrGenAIUsageOutputTokens), outputTokens),
		attribute.Int(string(aidomain.AttrGenAIUsageTotalTokens), inputTokens+outputTokens),
	)
}

func SetToolCallAttrs(span trace.Span, callID string, arguments []byte, result []byte) {
	attrs := []attribute.KeyValue{
		attribute.String(string(aidomain.AttrGenAIToolCallID), callID),
	}
	if arguments != nil {
		attrs = append(attrs, attribute.String(string(aidomain.AttrGenAIToolCallArguments), string(arguments)))
	}
	if result != nil {
		attrs = append(attrs, attribute.String(string(aidomain.AttrGenAIToolResult), string(result)))
	}
	span.SetAttributes(attrs...)
}

func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

func SetSpanSuccess(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}

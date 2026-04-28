package otel

import (
	"context"
	"encoding/json"
	"time"

	aidomain "cloud-agent-monitor/internal/aiinfra/domain"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type spanContextKey struct{}

type SpanInfo struct {
	TraceID  string
	SpanID   string
	ToolName string
}

func ContextWithSpanInfo(ctx context.Context, info *SpanInfo) context.Context {
	return context.WithValue(ctx, spanContextKey{}, info)
}

func SpanInfoFromContext(ctx context.Context) *SpanInfo {
	if info, ok := ctx.Value(spanContextKey{}).(*SpanInfo); ok {
		return info
	}
	return nil
}

type EinoCallRecord struct {
	TraceID       string    `json:"trace_id"`
	SpanID        string    `json:"span_id"`
	ParentSpanID  string    `json:"parent_span_id,omitempty"`
	ComponentType string    `json:"component_type"`
	ComponentName string    `json:"component_name"`
	Input         string    `json:"input,omitempty"`
	Output        string    `json:"output,omitempty"`
	Error         string    `json:"error,omitempty"`
	DurationMs    int       `json:"duration_ms"`
	Timestamp     time.Time `json:"timestamp"`
}

type EinoInterceptor struct {
	recorder EinoCallRecorder
}

type EinoCallRecorder interface {
	RecordCall(ctx context.Context, record EinoCallRecord) error
}

func NewEinoInterceptor(recorder EinoCallRecorder) *EinoInterceptor {
	return &EinoInterceptor{recorder: recorder}
}

func (i *EinoInterceptor) OnToolCallStart(ctx context.Context, toolName string, arguments []byte) context.Context {
	ctx, span := StartToolCallSpan(ctx, toolName, aidomain.GenAIToolTypeFunction)
	SetToolCallAttrs(span, "", arguments, nil)
	return context.WithValue(ctx, einoStartKey{}, time.Now())
}

func (i *EinoInterceptor) OnToolCallEnd(ctx context.Context, toolName string, result []byte) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(string(aidomain.AttrGenAIToolResult), string(result)),
		)
		SetSpanSuccess(span)
	}
	span.End()

	i.recordCall(ctx, "tool", toolName, string(result), nil)
}

func (i *EinoInterceptor) OnToolCallError(ctx context.Context, toolName string, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		RecordError(span, err)
	}
	span.End()

	i.recordCall(ctx, "tool", toolName, "", err)
}

func (i *EinoInterceptor) OnChatModelStart(ctx context.Context, system, model string, messages []byte) context.Context {
	ctx, _ = StartChatModelSpan(ctx, system, model, aidomain.GenAIOperationChat)
	return context.WithValue(ctx, einoStartKey{}, time.Now())
}

func (i *EinoInterceptor) OnChatModelEnd(ctx context.Context, model string, inputTokens, outputTokens int, response []byte) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		SetChatModelResponseAttrs(span, model, inputTokens, outputTokens, "stop")
		SetSpanSuccess(span)
	}
	span.End()

	RecordTokenUsage(ctx, "", model, int64(inputTokens), int64(outputTokens))
}

func (i *EinoInterceptor) OnChatModelError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		RecordError(span, err)
	}
	span.End()
}

func (i *EinoInterceptor) recordCall(ctx context.Context, componentType, componentName, output string, err error) {
	if i.recorder == nil {
		return
	}

	traceID, spanID := SpanContextFromCtx(ctx)

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	record := EinoCallRecord{
		TraceID:       traceID,
		SpanID:        spanID,
		ComponentType: componentType,
		ComponentName: componentName,
		Output:        output,
		Error:         errMsg,
		Timestamp:     time.Now(),
	}

	_ = i.recorder.RecordCall(context.Background(), record)
}

func MarshalArguments(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return data
}

type einoStartKey struct{}

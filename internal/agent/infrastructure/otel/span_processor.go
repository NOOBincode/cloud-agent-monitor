package otel

import (
	"context"
	"log/slog"
	"time"

	aidomain "cloud-agent-monitor/internal/aiinfra/domain"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

type ToolCallRecorder interface {
	RecordToolCall(ctx context.Context, record ToolCallRecord) error
}

type ToolCallRecord struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	ToolName     string
	ToolType     string
	ToolCallID   string
	Arguments    []byte
	Result       []byte
	Status       string
	ErrorType    string
	ErrorMessage string
	DurationMs   int
	CapturedAt   time.Time
}

type GenAISpanProcessor struct {
	recorder ToolCallRecorder
	logger   *slog.Logger
}

func NewGenAISpanProcessor(recorder ToolCallRecorder) *GenAISpanProcessor {
	return &GenAISpanProcessor{
		recorder: recorder,
		logger:   slog.Default(),
	}
}

func (p *GenAISpanProcessor) OnStart(ctx context.Context, s sdktrace.ReadWriteSpan) {
}

func (p *GenAISpanProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	attrs := s.Attributes()

	toolName := getAttrValue(attrs, string(aidomain.AttrGenAIToolName))
	if toolName == "" {
		return
	}

	status := "success"
	if s.Status().Code == codes.Error {
		status = "error"
	}

	sc := s.SpanContext()
	var parentSpanID string
	if ps := s.Parent(); ps.IsValid() {
		parentSpanID = ps.SpanID().String()
	}

	durationMs := int(s.EndTime().Sub(s.StartTime()) / time.Millisecond)

	arguments := []byte(getAttrValue(attrs, string(aidomain.AttrGenAIToolCallArguments)))
	result := []byte(getAttrValue(attrs, string(aidomain.AttrGenAIToolResult)))

	record := ToolCallRecord{
		TraceID:      sc.TraceID().String(),
		SpanID:       sc.SpanID().String(),
		ParentSpanID: parentSpanID,
		ToolName:     toolName,
		ToolType:     getAttrValue(attrs, string(aidomain.AttrGenAIToolType)),
		ToolCallID:   getAttrValue(attrs, string(aidomain.AttrGenAIToolCallID)),
		Arguments:    arguments,
		Result:       result,
		Status:       status,
		ErrorMessage: s.Status().Description,
		DurationMs:   durationMs,
		CapturedAt:   time.Now(),
	}

	if err := p.recorder.RecordToolCall(context.Background(), record); err != nil {
		p.logger.Error("failed to record tool call from span",
			"trace_id", record.TraceID,
			"span_id", record.SpanID,
			"tool_name", record.ToolName,
			"error", err,
		)
	}
}

func (p *GenAISpanProcessor) ForceFlush(ctx context.Context) error {
	return nil
}

func (p *GenAISpanProcessor) Shutdown(ctx context.Context) error {
	return nil
}

func getAttrValue(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsString()
		}
	}
	return ""
}

func SpanContextFromCtx(ctx context.Context) (traceID, spanID string) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return "", ""
}

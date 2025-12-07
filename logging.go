package main

import (
	"context"
	"log/slog"
	"os"

	otellog "go.opentelemetry.io/otel/log"
	otellogsdk "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/trace"
)

var (
	loggerProvider *otellogsdk.LoggerProvider
	otelLogger     otellog.Logger
)

// TraceContextHandler is a slog handler that adds trace context to logs
type TraceContextHandler struct {
	slog.Handler
}

// NewTraceContextHandler creates a new handler that adds trace context
func NewTraceContextHandler() *TraceContextHandler {
	baseHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &TraceContextHandler{Handler: baseHandler}
}

// Handle adds trace context to log records
func (h *TraceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract trace context from context
	span := trace.SpanFromContext(ctx)
	var traceID, spanID string
	var traceFlags trace.TraceFlags

	if span.IsRecording() {
		spanCtx := span.SpanContext()
		if spanCtx.IsValid() {
			traceID = spanCtx.TraceID().String()
			spanID = spanCtx.SpanID().String()
			traceFlags = spanCtx.TraceFlags()

			// Add trace context to slog record
			r.AddAttrs(
				slog.String("trace_id", traceID),
				slog.String("span_id", spanID),
				slog.String("trace_flags", traceFlags.String()),
			)
		}
	}

	// Send to stdout
	err := h.Handler.Handle(ctx, r)

	// Also send to OTLP exporter
	if loggerProvider != nil && otelLogger != nil {
		// Convert slog level to OTel severity
		severity := otellog.SeverityInfo
		switch r.Level {
		case slog.LevelDebug:
			severity = otellog.SeverityDebug
		case slog.LevelInfo:
			severity = otellog.SeverityInfo
		case slog.LevelWarn:
			severity = otellog.SeverityWarn
		case slog.LevelError:
			severity = otellog.SeverityError
		}

		// Create log record
		logRecord := otellog.Record{}
		logRecord.SetTimestamp(r.Time)
		logRecord.SetSeverity(severity)
		logRecord.SetSeverityText(r.Level.String())
		logRecord.SetBody(otellog.StringValue(r.Message))

		// ⭐ CRITICAL: Add trace context as attributes
		// SigNoz uses these attributes for log-trace correlation
		attrs := []otellog.KeyValue{
			otellog.String("log.message", r.Message),
			otellog.String("log.level", r.Level.String()),
		}

		// Add trace context attributes (SigNoz uses these for correlation)
		if traceID != "" {
			attrs = append(attrs, otellog.String("trace_id", traceID))
		}
		if spanID != "" {
			attrs = append(attrs, otellog.String("span_id", spanID))
		}
		if traceFlags.IsSampled() {
			attrs = append(attrs, otellog.String("trace_flags", traceFlags.String()))
		}

		// Add all slog attributes
		r.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, otellog.String(a.Key, a.Value.String()))
			return true
		})

		// Add all attributes to the record
		logRecord.AddAttributes(attrs...)

		// Emit log with context (OTel will extract trace context from ctx automatically)
		// The context already contains the span, so trace correlation happens automatically
		otelLogger.Emit(ctx, logRecord)
	}

	return err
}

// Enabled checks if logging is enabled for the given level
func (h *TraceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

// WithAttrs returns a new handler with additional attributes
func (h *TraceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceContextHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with a group
func (h *TraceContextHandler) WithGroup(name string) slog.Handler {
	return &TraceContextHandler{Handler: h.Handler.WithGroup(name)}
}

// SetupLogging initializes structured logging with trace context and OTLP export
func SetupLogging(lp *otellogsdk.LoggerProvider) {
	loggerProvider = lp
	if lp != nil {
		// Create OTel logger
		otelLogger = lp.Logger("span-links-demo",
			otellog.WithSchemaURL("https://opentelemetry.io/schemas/1.21.0"),
		)
	}

	// Setup slog with trace context handler
	logger := slog.New(NewTraceContextHandler())
	slog.SetDefault(logger)
}

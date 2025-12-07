package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	otellogsdk "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

// TelemetryProviders holds all OTel providers
type TelemetryProviders struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
	LoggerProvider *otellogsdk.LoggerProvider
}

// InitTracer initializes OpenTelemetry for traces, metrics, and logs
func InitTracer(ctx context.Context) (*TelemetryProviders, error) {
	// Get configuration from environment variables
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:4317" // Default local endpoint
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "span-links-demo"
	}

	// Get headers for authentication (SigNoz Cloud)
	headersStr := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS")
	var headers map[string]string
	if headersStr != "" {
		headers = parseHeaders(headersStr)
	}

	// Create resource describing the service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", "demo"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Parse endpoint URL to extract host:port
	endpointHost, useInsecure := parseEndpoint(endpoint)

	// Create OTLP trace exporter
	traceExporterOptions := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpointHost),
		otlptracehttp.WithURLPath("/v1/traces"),
	}
	if useInsecure {
		traceExporterOptions = append(traceExporterOptions, otlptracehttp.WithInsecure())
	}
	if len(headers) > 0 {
		traceExporterOptions = append(traceExporterOptions, otlptracehttp.WithHeaders(headers))
	}

	traceExporter, err := otlptracehttp.New(ctx, traceExporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create OTLP metrics exporter
	metricExporterOptions := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpointHost),
		otlpmetrichttp.WithURLPath("/v1/metrics"),
	}
	if useInsecure {
		metricExporterOptions = append(metricExporterOptions, otlpmetrichttp.WithInsecure())
	}
	if len(headers) > 0 {
		metricExporterOptions = append(metricExporterOptions, otlpmetrichttp.WithHeaders(headers))
	}

	metricExporter, err := otlpmetrichttp.New(ctx, metricExporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create tracer provider with batch span processor
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample all for demo
	)

	// Create meter provider with metric exporter
	reader := metric.NewPeriodicReader(metricExporter,
		metric.WithInterval(5*time.Second),
	)
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)

	// Create OTLP log exporter
	logExporterOptions := []otlploghttp.Option{
		otlploghttp.WithEndpoint(endpointHost),
		otlploghttp.WithURLPath("/v1/logs"),
	}
	if useInsecure {
		logExporterOptions = append(logExporterOptions, otlploghttp.WithInsecure())
	}
	if len(headers) > 0 {
		logExporterOptions = append(logExporterOptions, otlploghttp.WithHeaders(headers))
	}

	logExporter, err := otlploghttp.New(ctx, logExporterOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create log exporter: %w", err)
	}

	// Create logger provider
	lp := otellogsdk.NewLoggerProvider(
		otellogsdk.WithResource(res),
		otellogsdk.WithProcessor(otellogsdk.NewBatchProcessor(logExporter)),
	)

	// Set global providers
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)

	// Set propagator for distributed tracing
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	log.Printf("Initialized OpenTelemetry with endpoint: %s", endpointHost)

	return &TelemetryProviders{
		TracerProvider: tp,
		MeterProvider:  mp,
		LoggerProvider: lp,
	}, nil
}

// parseEndpoint extracts host:port from URL and returns insecure flag
func parseEndpoint(endpoint string) (string, bool) {
	var useInsecure bool

	if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
		useInsecure = false
	} else if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
		useInsecure = true
	} else {
		useInsecure = true
	}

	return endpoint, useInsecure
}

// parseHeaders parses header string in format "key1=value1,key2=value2" or "key=value"
func parseHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)

	// Split by comma if multiple headers
	pairs := strings.Split(headersStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// Split key=value
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headers[key] = value
		}
	}

	return headers
}

// shutdownProviders gracefully shuts down all OpenTelemetry providers
func shutdownProviders(providers *TelemetryProviders) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := providers.TracerProvider.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown tracer provider: %v", err)
	}

	if err := providers.MeterProvider.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown meter provider: %v", err)
	}

	if providers.LoggerProvider != nil {
		if err := providers.LoggerProvider.Shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown logger provider: %v", err)
		}
	}
}

// SetupLogging initializes structured logging with trace context and OTLP export
func SetupLogging(lp *otellogsdk.LoggerProvider) {
	loggerProvider = lp
	if lp != nil {
		otelLogger = lp.Logger("span-links-demo")
	}

	// Setup slog with trace context handler
	logger := slog.New(NewTraceContextHandler())
	slog.SetDefault(logger)
}

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

		// Add trace context as attributes
		attrs := []otellog.KeyValue{
			otellog.String("log.message", r.Message),
			otellog.String("log.level", r.Level.String()),
		}

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

		logRecord.AddAttributes(attrs...)

		// Emit log with context
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


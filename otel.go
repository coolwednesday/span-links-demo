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
	"go.opentelemetry.io/otel/propagation"
	otellog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// TelemetryProviders holds all OTel providers
type TelemetryProviders struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *metric.MeterProvider
	LoggerProvider *otellog.LoggerProvider
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
	// Configure periodic reader to export metrics every 5 seconds
	reader := metric.NewPeriodicReader(metricExporter,
		metric.WithInterval(5*time.Second), // Export every 5 seconds
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
	lp := otellog.NewLoggerProvider(
		otellog.WithResource(res),
		otellog.WithProcessor(otellog.NewBatchProcessor(logExporter)),
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

	// Note: Using log.Printf here because slog is initialized after this function
	log.Printf("OpenTelemetry initialized successfully")
	log.Printf("  Endpoint: %s", endpointHost)
	log.Printf("  Traces: /v1/traces")
	log.Printf("  Metrics: /v1/metrics (export interval: 5s)")
	log.Printf("  Logs: /v1/logs (with trace context)")

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

// Helper function to create a span context from stored trace info
func SpanContextFromMessage(order Order) trace.SpanContext {
	// In production, properly parse the traceparent header
	// For this demo, we construct it from the stored values
	if len(order.TraceParent) < 53 {
		return trace.SpanContext{}
	}

	// Parse traceparent format: 00-<trace-id>-<span-id>-<flags>
	// Example: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
	traceIDStr := order.TraceParent[3:35]  // 32 hex chars
	spanIDStr := order.TraceParent[36:52]  // 16 hex chars

	tid, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		// Note: Using log.Printf as this may be called before slog initialization
		log.Printf("Failed to parse trace ID from message: %v", err)
		return trace.SpanContext{}
	}

	sid, err := trace.SpanIDFromHex(spanIDStr)
	if err != nil {
		// Note: Using log.Printf as this may be called before slog initialization
		log.Printf("Failed to parse span ID from message: %v", err)
		return trace.SpanContext{}
	}

	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     true, // Indicates this context comes from a remote source
	})
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

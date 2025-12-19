package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Demonstrates the downside of forcing async work into parent-child via a remote
// parent context: the parent finishes, and the child starts later (via a
// handoff channel), inflating apparent latency within one trace.
func main() {
	ctx := context.Background()

	tp, err := initTracing(ctx)
	if err != nil {
		log.Fatalf("failed to init tracing: %v", err)
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(c); err != nil {
			log.Printf("shutdown tracer provider: %v", err)
		}
	}()

	tracer := otel.Tracer("remote-parent-gap")

	// Artificial delay to make the "gap" visible in UIs (simulates queue/scheduler delay).
	// Set REMOTE_PARENT_GAP_DELAY_MS to control it.
	delay := 2000 * time.Millisecond
	if v := os.Getenv("REMOTE_PARENT_GAP_DELAY_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms >= 0 {
			delay = time.Duration(ms) * time.Millisecond
		}
	}

	// Channel simulates a remote handoff of the parent context
	carrierCh := make(chan propagation.MapCarrier, 1)

	// Parent ends quickly, then hands off its context
	parentCtx, parentSpan := tracer.Start(ctx, "ParentRequest",
		trace.WithAttributes(
			attribute.String("note", "ends immediately"),
			attribute.Int64("demo.gap_delay_ms", delay.Milliseconds()),
		),
	)
	parentSpan.End()

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(parentCtx, carrier)
	carrierCh <- carrier
	close(carrierCh)

	// Async worker starts when it receives the carrier (after an artificial delay)
	go func() {
		carrier, ok := <-carrierCh
		if !ok {
			return
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		remoteCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

		_, childSpan := tracer.Start(remoteCtx, "AsyncWorkerChild",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("note", "remote-parent-handshake"),
				attribute.Int64("demo.gap_delay_ms", delay.Milliseconds()),
			),
		)
		// Do real work here if desired (no sleep needed)
		childSpan.End()
	}()

	// Give the worker enough time to run + export before process exit.
	time.Sleep(delay + 1500*time.Millisecond)
	log.Printf("Done. In SigNoz, you’ll see one trace: parent ends immediately; child starts later via remote context after %s, inflating apparent end-to-end duration.", delay)
}

// Trace-only setup
func initTracing(ctx context.Context) (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:4317"
	}
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "remote-parent-gap"
	}
	headers := parseHeaders(os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", "demo"),
		),
	)
	if err != nil {
		return nil, err
	}

	host, insecure := parseEndpoint(endpoint)
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath("/v1/traces"),
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(headers))
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	log.Printf("Tracing initialized for service=%s endpoint=%s", serviceName, host)
	return tp, nil
}

func parseEndpoint(endpoint string) (string, bool) {
	if strings.HasPrefix(endpoint, "https://") {
		return strings.TrimPrefix(endpoint, "https://"), false
	}
	if strings.HasPrefix(endpoint, "http://") {
		return strings.TrimPrefix(endpoint, "http://"), true
	}
	return endpoint, true
}

func parseHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)
	if headersStr == "" {
		return headers
	}
	pairs := strings.Split(headersStr, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return headers
}

package examples

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RetryExample demonstrates retry pattern with Span Links
// Each retry attempt links back to the original attempt
func RetryExample(ctx context.Context) {
	tracer := otel.Tracer("retry-example")
	requestID := "req-123"

	// Original attempt
	ctx, originalSpan := tracer.Start(ctx, "ProcessRequest",
		trace.WithAttributes(
			attribute.String("request.id", requestID),
			attribute.Int("attempt", 1),
		),
	)

	originalSpanCtx := originalSpan.SpanContext()

	// Simulate processing that might fail
	success := simulateProcessing(ctx, originalSpan, 1)
	originalSpan.End()

	if success {
		slog.InfoContext(ctx, "Request processed successfully on first attempt",
			slog.String("request.id", requestID),
		)
		return
	}

	// Retry logic with Span Links
	maxRetries := 3
	for attempt := 2; attempt <= maxRetries; attempt++ {
		slog.InfoContext(ctx, "Retrying request",
			slog.String("request.id", requestID),
			slog.Int("attempt", attempt),
			slog.Int("max_retries", maxRetries),
		)

		// Create a link to the original span
		link := trace.Link{
			SpanContext: originalSpanCtx,
			Attributes: []attribute.KeyValue{
				attribute.String("link.type", "retry"),
				attribute.Int("retry.attempt", attempt),
				attribute.String("original.request.id", requestID),
			},
		}

		// Create retry span with link
		retryCtx, retrySpan := tracer.Start(context.Background(), "ProcessRequest",
			trace.WithLinks(link),
			trace.WithAttributes(
				attribute.String("request.id", requestID),
				attribute.Int("attempt", attempt),
				attribute.Bool("is_retry", true),
			),
		)

		// Simulate processing
		success := simulateProcessing(retryCtx, retrySpan, attempt)
		retrySpan.End()

		if success {
			slog.InfoContext(retryCtx, "Request processed successfully",
				slog.String("request.id", requestID),
				slog.Int("attempt", attempt),
			)
			return
		}

		// Wait before next retry (exponential backoff)
		backoff := time.Duration(attempt) * 100 * time.Millisecond
		time.Sleep(backoff)
	}

	slog.ErrorContext(ctx, "Request failed after all retry attempts",
		slog.String("request.id", requestID),
		slog.Int("max_retries", maxRetries),
	)
}

// simulateProcessing simulates a processing operation that might fail
func simulateProcessing(ctx context.Context, span trace.Span, attempt int) bool {
	// Simulate processing time
	time.Sleep(50 * time.Millisecond)

	// First attempt always fails to demonstrate retry
	if attempt == 1 {
		err := fmt.Errorf("processing failed on attempt %d", attempt)
		span.RecordError(err)
		span.SetStatus(codes.Error, "Processing failed")
		return false
	}

	// Subsequent attempts have 70% success rate
	if rand.Float32() < 0.7 {
		span.AddEvent("Processing succeeded",
			trace.WithAttributes(
				attribute.String("status", "success"),
			),
		)
		span.SetStatus(codes.Ok, "Processing succeeded")
		return true
	}

	err := fmt.Errorf("processing failed on attempt %d", attempt)
	span.RecordError(err)
	span.SetStatus(codes.Error, "Processing failed")
	return false
}

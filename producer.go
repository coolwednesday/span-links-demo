package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ProducerService publishes orders to the queue
type ProducerService struct {
	queue  *SimpleQueue
	tracer trace.Tracer
}

// NewProducerService creates a new producer service
func NewProducerService(queue *SimpleQueue) *ProducerService {
	return &ProducerService{
		queue:  queue,
		tracer: otel.Tracer("producer-service"),
	}
}

// PublishOrderBatch publishes multiple orders to the queue and returns the span context
// for workers to link back to
func (p *ProducerService) PublishOrderBatch(ctx context.Context, count int) (trace.SpanContext, error) {
	if count <= 0 {
		return trace.SpanContext{}, errors.New("batch size must be greater than zero")
	}

	ctx, span := p.tracer.Start(ctx, "PublishOrderBatch",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.Int("order.batch.size", count),
		),
	)
	defer span.End()

	var publishedCount int
	var lastErr error

	for i := 0; i < count; i++ {
		order := Order{
			ID:         fmt.Sprintf("ORDER-%s", uuid.New().String()[:8]),
			CustomerID: fmt.Sprintf("CUST-%d", 1000+i),
			Amount:     float64(100 + i*10),
			CreatedAt:  time.Now(),
		}

		ctx, pubSpan := p.tracer.Start(ctx, "PublishOrder",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("order.id", order.ID),
				attribute.String("customer.id", order.CustomerID),
				attribute.Float64("order.amount", order.Amount),
			),
		)

		if err := p.queue.Publish(ctx, order); err != nil {
			pubSpan.RecordError(err)
			pubSpan.End()
			lastErr = fmt.Errorf("failed to publish order %s: %w", order.ID, err)
			continue
		}

		publishedCount++
		pubSpan.End()
	}

	if publishedCount == 0 {
		span.RecordError(lastErr)
		return trace.SpanContext{}, fmt.Errorf("failed to publish any orders: %w", lastErr)
	}

	span.AddEvent("Batch published",
		trace.WithAttributes(
			attribute.Int("published.count", publishedCount),
			attribute.Int("total.count", count),
		),
	)

	slog.InfoContext(ctx, "Order batch published successfully",
		slog.Int(LogKeyBatchSize, publishedCount),
	)

	return span.SpanContext(), nil
}

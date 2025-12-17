package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
// for workers to link back to.
// The documentation refers to actions performed in publishInternal to simplify removing the complexity of dual/backward linking.
func (p *ProducerService) PublishOrderBatch(ctx context.Context, count int) (trace.SpanContext, error) {
	span, _, _, err := p.publishInternal(ctx, count, false)
	if err != nil {
		return trace.SpanContext{}, err
	}
	return span.SpanContext(), nil
}

// PublishOrderBatchWithOpenSpan publishes orders and returns the open batch span
// (caller must End it) along with per-order spans and the count published. Used for forward-link demo.
func (p *ProducerService) PublishOrderBatchWithOpenSpan(ctx context.Context, count int) (trace.Span, map[string]trace.Span, int, error) {
	return p.publishInternal(ctx, count, true)
}

func (p *ProducerService) publishInternal(ctx context.Context, count int, keepOpen bool) (trace.Span, map[string]trace.Span, int, error) {
	if count <= 0 {
		return nil, nil, 0, errors.New("batch size must be greater than zero")
	}

	ctx, span := p.tracer.Start(ctx, "PublishOrderBatch",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.Int("order.batch.size", count),
		),
	)

	var publishedCount int
	orderSpans := make(map[string]trace.Span, count)
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
		orderSpans[order.ID] = pubSpan
		if !keepOpen {
			pubSpan.End()
		}
	}

	if publishedCount == 0 {
		span.RecordError(lastErr)
		if !keepOpen {
			span.End()
		}
		return span, orderSpans, 0, fmt.Errorf("failed to publish any orders: %w", lastErr)
	}

	span.AddEvent("Batch published",
		trace.WithAttributes(
			attribute.Int("published.count", publishedCount),
			attribute.Int("total.count", count),
		),
	)

	log.Printf("Order batch published successfully (published=%d)", publishedCount)

	if !keepOpen {
		span.End()
		for _, s := range orderSpans {
			s.End()
		}
	}

	// When keepOpen, caller is responsible to End batch span and any order spans it keeps open.
	return span, orderSpans, publishedCount, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// WorkerService processes orders from the queue with observability instrumentation
type WorkerService struct {
	queue        *SimpleQueue
	tracer       trace.Tracer
	activeOrders int64
	spanCtxSink  chan OrderSpanContext
}

// OrderSpanContext is used to emit consumer span contexts back to the producer.
type OrderSpanContext struct {
	OrderID string
	Ctx     trace.SpanContext
}

// NewWorkerService creates a new worker service with metrics instrumentation
func NewWorkerService(queue *SimpleQueue) *WorkerService {
	return &WorkerService{
		queue:  queue,
		tracer: otel.Tracer("worker-service"),
	}
}

// SetSpanContextSink sets an optional channel to emit finished processing span contexts
// (used for forward-link demo). If nil, no emission is performed.
func (w *WorkerService) SetSpanContextSink(ch chan OrderSpanContext) {
	w.spanCtxSink = ch
}

// ProcessOrders continuously consumes and processes orders from the queue
func (w *WorkerService) ProcessOrders(ctx context.Context, workerID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			order, err := w.queue.Consume(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}

			if err := w.processOrderWithLink(ctx, order, workerID); err != nil {
				log.Printf("Failed to process order %s (worker=%s): %v", order.ID, workerID, err)
			}
		}
	}
}

// processOrderWithLink processes an order and creates a span link to the producer span
func (w *WorkerService) processOrderWithLink(ctx context.Context, order Order, workerID string) error {
	if order.ID == "" {
		return errors.New("order ID is required")
	}

	startTime := time.Now()
	originalSpanCtx := SpanContextFromMessage(order)

	// Create span link to producer span
	link := trace.Link{
		SpanContext: originalSpanCtx,
		Attributes: []attribute.KeyValue{
			attribute.String("link.type", "queue_consumption"),
			attribute.String("source.service", "producer-service"),
		},
	}

	// Start processing span with link
	ctx, span := w.tracer.Start(ctx, "ProcessOrder",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithLinks(link),
		trace.WithAttributes(
			attribute.String("order.id", order.ID),
			attribute.String("customer.id", order.CustomerID),
			attribute.Float64("order.amount", order.Amount),
			attribute.String("worker.id", workerID),
		),
	)
	defer span.End()

	atomic.AddInt64(&w.activeOrders, 1)
	defer atomic.AddInt64(&w.activeOrders, -1)

	log.Printf("Order processing started (order=%s worker=%s amount=%.2f)", order.ID, workerID, order.Amount)

	// Process order steps
	if err := w.validateOrder(ctx, order); err != nil {
		span.RecordError(err)
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := w.processPayment(ctx, order); err != nil {
		span.RecordError(err)
		return fmt.Errorf("payment processing failed: %w", err)
	}

	if err := w.shipOrder(ctx, order); err != nil {
		span.RecordError(err)
		return fmt.Errorf("shipping failed: %w", err)
	}

	duration := time.Since(startTime).Seconds()
	log.Printf("Order processing completed successfully (order=%s worker=%s duration=%.2fs)", order.ID, workerID, duration)

	// Emit span context for optional forward-linking demo
	if w.spanCtxSink != nil {
		select {
		case w.spanCtxSink <- OrderSpanContext{OrderID: order.ID, Ctx: span.SpanContext()}:
		default:
			// drop if channel full
		}
	}

	return nil
}

// validateOrder validates the order
func (w *WorkerService) validateOrder(ctx context.Context, order Order) error {
	ctx, span := w.tracer.Start(ctx, "ValidateOrder")
	defer span.End()

	time.Sleep(ValidationTimeout)

	// Validation logic would go here
	// For demo, we always succeed
	return nil
}

// processPayment processes payment for the order
func (w *WorkerService) processPayment(ctx context.Context, order Order) error {
	ctx, span := w.tracer.Start(ctx, "ProcessPayment",
		trace.WithAttributes(
			attribute.Float64("payment.amount", order.Amount),
		),
	)
	defer span.End()

	time.Sleep(PaymentTimeout)

	log.Printf("Payment processed successfully (order=%s amount=%.2f)", order.ID, order.Amount)

	return nil
}

// shipOrder ships the order to the customer
func (w *WorkerService) shipOrder(ctx context.Context, order Order) error {
	ctx, span := w.tracer.Start(ctx, "ShipOrder",
		trace.WithAttributes(
			attribute.String("customer.id", order.CustomerID),
		),
	)
	defer span.End()

	time.Sleep(ShippingTimeout)

	log.Printf("Order shipped to customer (order=%s customer=%s)", order.ID, order.CustomerID)

	return nil
}

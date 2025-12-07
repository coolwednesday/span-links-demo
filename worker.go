package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// WorkerService processes orders from the queue with observability instrumentation
type WorkerService struct {
	queue                  *SimpleQueue
	tracer                 trace.Tracer
	meter                  metric.Meter
	ordersProcessedCounter metric.Int64Counter
	processingDurationHist metric.Float64Histogram
	queueDepthGauge        metric.Int64ObservableGauge
	activeOrders           int64
}

// NewWorkerService creates a new worker service with metrics instrumentation
func NewWorkerService(queue *SimpleQueue) *WorkerService {
	meter := otel.Meter("worker-service")

	// Create metrics instruments
	ordersProcessed, err := meter.Int64Counter("orders.processed",
		metric.WithDescription("Total orders processed"),
	)
	if err != nil {
		log.Printf("Failed to create orders.processed counter: %v", err)
	}

	processingDuration, err := meter.Float64Histogram("processing.duration",
		metric.WithDescription("Order processing duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Printf("Failed to create processing.duration histogram: %v", err)
	}

	queueDepth, err := meter.Int64ObservableGauge("queue.depth",
		metric.WithDescription("Current queue depth"),
		metric.WithUnit("1"),
	)
	if err != nil {
		log.Printf("Failed to create queue.depth gauge: %v", err)
	}

	ws := &WorkerService{
		queue:                  queue,
		tracer:                 otel.Tracer("worker-service"),
		meter:                  meter,
		ordersProcessedCounter: ordersProcessed,
		processingDurationHist: processingDuration,
		queueDepthGauge:        queueDepth,
	}

	// Register callback for queue depth observable gauge
	if queueDepth != nil {
		_, err = meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
			queueLen := int64(ws.queue.Length())
			o.ObserveInt64(queueDepth, queueLen,
				metric.WithAttributes(
					attribute.String("metric.type", "gauge"),
				),
			)
			return nil
		}, queueDepth)
		if err != nil {
			log.Printf("Failed to register queue depth callback: %v", err)
		}
	}

	return ws
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
				slog.ErrorContext(ctx, "Failed to process order",
					slog.String(LogKeyOrderID, order.ID),
					slog.String(LogKeyWorkerID, workerID),
					slog.String(LogKeyError, err.Error()),
				)
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

	slog.InfoContext(ctx, "Order processing started",
		slog.String(LogKeyWorkerID, workerID),
		slog.Float64(LogKeyAmount, order.Amount),
	)

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

	// Record metrics
	duration := time.Since(startTime).Seconds()
	if w.ordersProcessedCounter != nil {
		w.ordersProcessedCounter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("order.status", "success"),
				attribute.String("worker.id", workerID),
			),
		)
	}

	if w.processingDurationHist != nil {
		w.processingDurationHist.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("worker.id", workerID),
			),
		)
	}

	slog.InfoContext(ctx, "Order processing completed successfully",
		slog.String(LogKeyWorkerID, workerID),
		slog.Float64(LogKeyDuration, duration),
	)

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

	slog.InfoContext(ctx, "Payment processed successfully",
		slog.Float64(LogKeyAmount, order.Amount),
	)

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

	slog.InfoContext(ctx, "Order shipped to customer",
		slog.String(LogKeyCustomerID, order.CustomerID),
	)

	return nil
}

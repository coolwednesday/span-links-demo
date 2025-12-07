package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// Order represents a message in our queue
type Order struct {
	ID             string    `json:"id"`
	CustomerID     string    `json:"customer_id"`
	Amount         float64   `json:"amount"`
	CreatedAt      time.Time `json:"created_at"`
	TraceParent    string    `json:"trace_parent"`     // W3C traceparent header
	TraceState     string    `json:"trace_state"`      // W3C tracestate
	OriginalSpanID string    `json:"original_span_id"` // Link to original span
}

// SimpleQueue mimics a message queue (in production, use RabbitMQ, Kafka, etc.)
type SimpleQueue struct {
	messages chan Order
	mu       sync.Mutex
}

func NewSimpleQueue() *SimpleQueue {
	return &SimpleQueue{
		messages: make(chan Order, DefaultQueueCapacity),
	}
}

// Publish adds a message to the queue
func (q *SimpleQueue) Publish(ctx context.Context, order Order) error {
	// Get current span context to pass to workers later
	span := trace.SpanFromContext(ctx)
	spanCtx := span.SpanContext()

	// Store span context info in the message so workers can link back
	order.OriginalSpanID = spanCtx.SpanID().String()
	order.TraceParent = fmt.Sprintf("00-%s-%s-01",
		spanCtx.TraceID().String(),
		spanCtx.SpanID().String(),
	)

	q.mu.Lock()
	defer q.mu.Unlock()

	select {
	case q.messages <- order:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Consume retrieves a message from the queue
func (q *SimpleQueue) Consume(ctx context.Context) (Order, error) {
	select {
	case msg := <-q.messages:
		return msg, nil
	case <-ctx.Done():
		return Order{}, ctx.Err()
	}
}

// Length returns the number of messages in the queue
func (q *SimpleQueue) Length() int {
	return len(q.messages)
}

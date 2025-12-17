package examples

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// FanOutExample demonstrates one-to-many pattern with Span Links
// One producer creates a batch, multiple workers process items in parallel
func FanOutExample(ctx context.Context) {
	tracer := otel.Tracer("fanout-example")

	// Create a root span for the batch operation
	ctx, rootSpan := tracer.Start(ctx, "CreateBatch",
		trace.WithAttributes(
			attribute.String("batch.id", uuid.New().String()),
			attribute.Int("batch.size", 5),
		),
	)
	defer rootSpan.End()

	rootSpanCtx := rootSpan.SpanContext()
	batchID := uuid.New().String()
	items := []string{"item-1", "item-2", "item-3", "item-4", "item-5"}

	log.Printf("Creating batch (batch.id=%s items.count=%d)", batchID, len(items))

	// Fan-out: Process each item in parallel with Span Links
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		go func(idx int, itemID string) {
			defer wg.Done()

			// Create a link to the root batch span
			link := trace.Link{
				SpanContext: rootSpanCtx,
				Attributes: []attribute.KeyValue{
					attribute.String("link.type", "fan_out"),
					attribute.String("batch.id", batchID),
					attribute.Int("item.index", idx),
				},
			}

			// Create a new span with link (new trace, but linked to batch)
			_, itemSpan := tracer.Start(context.Background(), "ProcessItem",
				trace.WithLinks(link),
				trace.WithAttributes(
					attribute.String("item.id", itemID),
					attribute.String("batch.id", batchID),
					attribute.Int("item.index", idx),
				),
			)
			defer itemSpan.End()

			// Simulate processing
			log.Printf("Processing item (item.id=%s batch.id=%s)", itemID, batchID)
			time.Sleep(200 * time.Millisecond)

			itemSpan.AddEvent("Item processed",
				trace.WithAttributes(
					attribute.String("item.status", "completed"),
				),
			)
		}(i, item)
	}

	// Wait for all items to complete
	wg.Wait()

	rootSpan.AddEvent("Batch processing completed",
		trace.WithAttributes(
			attribute.Int("processed.count", len(items)),
		),
	)

	log.Printf("Batch processing completed (batch.id=%s processed.count=%d)", batchID, len(items))
}

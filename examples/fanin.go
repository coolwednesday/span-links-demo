package examples

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// FanInExample demonstrates many-to-one pattern with Span Links
// Multiple producers create items, one aggregator collects them
func FanInExample(ctx context.Context) {
	tracer := otel.Tracer("fanin-example")

	// Simulate multiple producers creating items
	numProducers := 3
	results := make(chan string, numProducers)
	producerSpansChan := make(chan trace.SpanContext, numProducers)

	var wg sync.WaitGroup
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()

			// Each producer creates its own span
			_, producerSpan := tracer.Start(context.Background(), "ProduceItem",
				trace.WithAttributes(
					attribute.Int("producer.id", producerID),
					attribute.String("item.value", fmt.Sprintf("value-%d", producerID)),
				),
			)
			defer producerSpan.End()

			// Send span context to channel (thread-safe)
			producerSpansChan <- producerSpan.SpanContext()

			// Simulate production
			log.Printf("Producer creating item (producer.id=%d)", producerID)
			time.Sleep(150 * time.Millisecond)

			results <- fmt.Sprintf("item-from-producer-%d", producerID)
		}(i)
	}

	// Wait for all producers to finish
	wg.Wait()
	close(results)
	close(producerSpansChan)

	// Collect all producer span contexts
	var producerSpans []trace.SpanContext
	for spanCtx := range producerSpansChan {
		producerSpans = append(producerSpans, spanCtx)
	}

	// Create links from aggregator to all producer spans
	links := make([]trace.Link, 0, len(producerSpans))
	for i, producerSpanCtx := range producerSpans {
		links = append(links, trace.Link{
			SpanContext: producerSpanCtx,
			Attributes: []attribute.KeyValue{
				attribute.String("link.type", "fan_in"),
				attribute.Int("producer.index", i),
			},
		})
	}

	// Create aggregator span with links to all producers
	ctx, aggregatorSpan := tracer.Start(ctx, "AggregateResults",
		trace.WithLinks(links...),
		trace.WithAttributes(
			attribute.String("aggregation.id", uuid.New().String()),
			attribute.Int("items.count", len(producerSpans)),
		),
	)
	defer aggregatorSpan.End()

	// Aggregate results
	aggregated := []string{}
	for result := range results {
		aggregated = append(aggregated, result)
		log.Printf("Aggregated item (item=%s)", result)
	}

	aggregatorSpan.AddEvent("Aggregation completed",
		trace.WithAttributes(
			attribute.Int("aggregated.count", len(aggregated)),
		),
	)

	log.Printf("Aggregation completed (items.count=%d)", len(aggregated))
}

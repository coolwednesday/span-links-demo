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

// set to true to also add forward links from workers to aggregator (same trace)
const enableForwardLinksToAggregator = false

// SameTraceSpanLinks demonstrates span links within the SAME trace.
// Workers run in parallel under the same root trace, and an aggregator later links back
// to all worker spans (N:1) using span links (same TraceID).
func SameTraceSpanLinks(ctx context.Context) {
	tracer := otel.Tracer("same-trace-span-links")

	// Root request span (all work shares this trace)
	ctx, root := tracer.Start(ctx, "SearchRequest",
		trace.WithAttributes(
			attribute.String("request.id", uuid.New().String()),
			attribute.Int("shard.count", 4),
		),
	)
	defer root.End()

	shardIDs := []string{"shard-a", "shard-b", "shard-c", "shard-d"}
	workerSpanContexts := make([]trace.SpanContext, len(shardIDs))

	// If you want worker spans to link *forward* to the aggregator, the aggregator must exist
	// while workers run (so they can reference its SpanContext). This makes the aggregator
	// span duration overlap with the workers by design.
	var aggSpanCtx trace.SpanContext
	var aggSpan trace.Span
	var aggCtx context.Context
	if enableForwardLinksToAggregator {
		aggCtx, aggSpan = tracer.Start(ctx, "AggregateResults",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("aggregation.mode", "same_trace_span_links"),
				attribute.Bool("demo.agg_started_before_workers", true),
			),
		)
		aggSpanCtx = aggSpan.SpanContext()
	}

	var wg sync.WaitGroup
	for i, shard := range shardIDs {
		wg.Add(1)
		go func(idx int, shardID string) {
			defer wg.Done()

			// Child span in the SAME trace (inherits from root ctx)
			workerCtx, workerSpan := tracer.Start(ctx, "QueryShard",
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					attribute.String("shard.id", shardID),
					attribute.Int("shard.index", idx),
				),
			)

			// Simulate work
			time.Sleep(120 * time.Millisecond)
			workerSpan.AddEvent("Shard query completed")

			// Optional forward link to aggregator (same trace)
			if enableForwardLinksToAggregator {
				workerSpan.AddLink(trace.Link{
					SpanContext: aggSpanCtx,
					Attributes: []attribute.KeyValue{
						attribute.String("link.type", "forward_to_aggregator"),
						attribute.String("link.direction", "forward"),
						attribute.String("link.trace_relationship", "same_trace"),
					},
				})
			}

			workerSpanContexts[idx] = workerSpan.SpanContext()
			workerSpan.End()

			log.Printf("Shard %s completed (trace=%s span=%s)",
				shardID, workerSpanContexts[idx].TraceID(), workerSpanContexts[idx].SpanID())

			_ = workerCtx
		}(i, shard)
	}

	wg.Wait()

	// Aggregator runs after workers finish. It is still in the SAME trace (root ctx),
	// but it links back to all worker spans to express N:1 relationship.
	links := make([]trace.Link, 0, len(workerSpanContexts))
	for i, sc := range workerSpanContexts {
		if sc.IsValid() {
			links = append(links, trace.Link{
				SpanContext: sc,
				Attributes: []attribute.KeyValue{
					attribute.String("link.type", "shard_result"),
					attribute.String("shard.id", shardIDs[i]),
					attribute.String("link.direction", "backward"),
					attribute.String("link.trace_relationship", "same_trace"),
				},
			})
		}
	}

	// Default behavior: start the aggregator AFTER workers, so its duration reflects only
	// the aggregation step (not the shard query time).
	if !enableForwardLinksToAggregator {
		aggCtx, aggSpan = tracer.Start(ctx, "AggregateResults",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithAttributes(
				attribute.String("aggregation.mode", "same_trace_span_links"),
				attribute.Bool("demo.agg_started_before_workers", false),
			),
		)
	}

	aggSpan.SetAttributes(attribute.Int("shard.completed", len(links)))
	for _, l := range links {
		aggSpan.AddLink(l)
	}
	time.Sleep(50 * time.Millisecond)
	aggSpan.AddEvent("Aggregation completed")
	aggSpan.End()

	log.Printf("Aggregation completed (trace=%s, linked_shards=%d)",
		root.SpanContext().TraceID(), len(links))

	_ = aggCtx
}

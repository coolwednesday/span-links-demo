package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const maxOrdersToPublish = 10

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OpenTelemetry (traces only)
	providers, err := InitTracer(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer shutdownProviders(providers)

	// Create services
	queue := NewSimpleQueue()
	producer := NewProducerService(queue)
	worker := NewWorkerService(queue)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start worker goroutines
	var wg sync.WaitGroup
	log.Printf("Starting workers (count=%d)", DefaultWorkerCount)

	var spanCtxSink chan OrderSpanContext
	if forwardLinksEnabled() {
		spanCtxSink = make(chan OrderSpanContext, DefaultQueueCapacity)
		worker.SetSpanContextSink(spanCtxSink)
	}

	for i := 1; i <= DefaultWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker.ProcessOrders(ctx, fmt.Sprintf("Worker-%d", workerID))
		}(i)
	}

	if forwardLinksEnabled() {
		runForwardSingleBatch(ctx, cancel, producer, spanCtxSink)
		wg.Wait()
		return
	}

	// Backward-only mode: publish a single batch then exit (same batch size as forward mode)
	runBackwardSingleBatch(ctx, cancel, producer)

	// Wait for shutdown signal or completion
	select {
	case <-sigChan:
		log.Printf("Shutdown signal received, initiating graceful shutdown")
		cancel()
	case <-ctx.Done():
		log.Printf("Completed publishing, shutting down")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("All workers stopped successfully")
	case <-shutdownCtx.Done():
		log.Printf("Shutdown timeout reached, some workers may not have stopped")
	}

	log.Printf("Application shutdown complete")
}

// shutdownProviders gracefully shuts down all OpenTelemetry providers
func shutdownProviders(providers *TelemetryProviders) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := providers.TracerProvider.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown tracer provider: %v", err)
	}
}

// runForwardSingleBatch publishes a single batch, waits for consumer contexts,
// adds per-order forward links, then exits.
func runForwardSingleBatch(ctx context.Context, cancel context.CancelFunc, producer *ProducerService, spanCtxSink chan OrderSpanContext) {
	log.Printf("Forward-link demo enabled: running a single batch and exiting")

	batchSpan, orderSpans, produced, err := producer.PublishOrderBatchWithOpenSpan(ctx, DefaultBatchSize)
	if err != nil {
		log.Fatalf("Failed to publish order batch: %v", err)
	}

	collected := make([]OrderSpanContext, 0, produced)
	timeout := time.After(30 * time.Second)
	for len(collected) < produced {
		select {
		case sc := <-spanCtxSink:
			if sc.Ctx.IsValid() {
				collected = append(collected, sc)
			}
		case <-timeout:
			log.Printf("Timed out waiting for consumer spans; collected=%d expected=%d", len(collected), produced)
			goto doneCollect
		case <-ctx.Done():
			goto doneCollect
		}
	}
doneCollect:

	// Per-order forward links only (PublishOrder -> ProcessOrder)
	for _, sc := range collected {
		if pubSpan, ok := orderSpans[sc.OrderID]; ok && pubSpan != nil {
			pubSpan.AddLink(trace.Link{
				SpanContext: sc.Ctx,
				Attributes: []attribute.KeyValue{
					attribute.String("link.direction", "forward"),
					attribute.String("link.type", "forward_to_consumer"),
					attribute.String("link.level", "order"),
					attribute.String("order.id", sc.OrderID),
				},
			})
			pubSpan.End()
			orderSpans[sc.OrderID] = nil
		}
	}
	// End any publish spans that never received a consumer context
	for oid, s := range orderSpans {
		if s != nil {
			log.Printf("Ending publish span without forward link (order=%s)", oid)
			s.End()
			orderSpans[oid] = nil
		}
	}
	log.Printf("Added %d forward links to PublishOrder spans", len(collected))
	batchSpan.End()

	// Graceful shutdown
	cancel()
}

// runBackwardSingleBatch publishes exactly one batch (DefaultBatchSize) and exits.
// This keeps the run length comparable to forward mode.
func runBackwardSingleBatch(ctx context.Context, cancel context.CancelFunc, producer *ProducerService) {
	log.Printf("Backward-link mode: publishing a single batch (size=%d) and exiting", DefaultBatchSize)
	go func() {
		_, err := producer.PublishOrderBatch(ctx, DefaultBatchSize)
		if err != nil {
			log.Printf("Failed to publish order batch: %v", err)
		}
		cancel()
	}()
}

func forwardLinksEnabled() bool {
	val := os.Getenv("ENABLE_FORWARD_LINKS_TO_PRODUCER")
	if val == "" {
		return false
	}
	enabled, err := strconv.ParseBool(val)
	if err != nil {
		return false
	}
	return enabled
}

func init() {
	// Load .env file if it exists (ignore errors if file doesn't exist)
	_ = godotenv.Load()
}

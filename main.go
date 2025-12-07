package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func init() {
	// Load .env file if it exists (ignore errors if file doesn't exist)
	_ = godotenv.Load()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OpenTelemetry (traces, metrics, logs)
	providers, err := InitTracer(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer shutdownProviders(providers)

	// Setup logging with trace context support and OTLP export
	SetupLogging(providers.LoggerProvider)

	// Create services
	queue := NewSimpleQueue()
	producer := NewProducerService(queue)
	worker := NewWorkerService(queue)

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start worker goroutines
	var wg sync.WaitGroup
	slog.Info("Starting workers",
		slog.Int("worker_count", DefaultWorkerCount),
	)

	for i := 1; i <= DefaultWorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			worker.ProcessOrders(ctx, fmt.Sprintf("Worker-%d", workerID))
		}(i)
	}

	// Start order batch publisher
	slog.Info("Starting order batch publisher",
		slog.Duration("interval", BatchPublishInterval),
		slog.Int("batch_size", DefaultBatchSize),
	)

	go func() {
		ticker := time.NewTicker(BatchPublishInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := producer.PublishOrderBatch(ctx, DefaultBatchSize)
				if err != nil {
					slog.ErrorContext(ctx, "Failed to publish order batch",
						slog.String(LogKeyError, err.Error()),
					)
				}
			}
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	slog.Info("Shutdown signal received, initiating graceful shutdown")

	// Cancel context to stop all goroutines
	cancel()

	// Wait for workers to finish with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All workers stopped successfully")
	case <-shutdownCtx.Done():
		slog.Warn("Shutdown timeout reached, some workers may not have stopped")
	}

	slog.Info("Application shutdown complete")
}

// shutdownProviders gracefully shuts down all OpenTelemetry providers
func shutdownProviders(providers *TelemetryProviders) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := providers.TracerProvider.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown tracer provider",
			slog.String(LogKeyError, err.Error()),
		)
	}

	if err := providers.MeterProvider.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown meter provider",
			slog.String(LogKeyError, err.Error()),
		)
	}

	if providers.LoggerProvider != nil {
		if err := providers.LoggerProvider.Shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown logger provider",
				slog.String(LogKeyError, err.Error()),
			)
		}
	}
}

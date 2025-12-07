# Span Links Demo with SigNoz

A Go demonstration of OpenTelemetry Span Links for correlating operations across multiple traces in asynchronous patterns (queue → worker, batch processing, fan-out/fan-in, retry).

## Prerequisites

- Go 1.21+
- Docker & Docker Compose (for local SigNoz)
- SigNoz Cloud account (optional)

## Quick Start

### Option 1: SigNoz Cloud

1. Get your ingestion key from [SigNoz Cloud](https://signoz.io) → Settings → Ingestion Settings
2. Create `.env` file:
   ```bash
   cp .env.example .env
   # Edit .env with your ingestion key and region
   ```
3. Run:
   ```bash
   go mod download
   go run .
   ```

### Option 2: Local SigNoz

1. Start SigNoz:
   ```bash
   docker-compose up -d
   ```
2. Create `.env` file:
   ```bash
   cp .env.example .env
   # Uncomment local SigNoz lines in .env
   ```
3. Run:
   ```bash
   go mod download
   go run .
   ```
4. View at http://localhost:3301

## Project Structure

```
├── main.go              # Entry point
├── producer.go          # Producer service
├── worker.go            # Worker service (Span Links)
├── queue.go             # Message queue
├── otel.go              # OpenTelemetry setup
├── logging.go           # Structured logging
├── constants.go         # Configuration
├── docker-compose.yml   # Local SigNoz
└── examples/            # Additional examples
    ├── fanout.go        # Fan-out pattern
    ├── fanin.go         # Fan-in pattern
    └── retry.go         # Retry pattern
```

## Running Examples

```bash
# Fan-out: One batch → Multiple workers
go run ./examples/common.go ./examples/fanout.go

# Fan-in: Multiple producers → One aggregator
go run ./examples/common.go ./examples/fanin.go

# Retry: Retry attempts with links
go run ./examples/common.go ./examples/retry.go
```

## Viewing Span Links in SigNoz

1. Navigate to **Traces**
2. Filter by service: `span-links-demo`
3. Click on a `ProcessOrder` trace
4. Look for **"Links"** tab in span details
5. Click the link to navigate to the producer trace

## Configuration

Environment variables (via `.env` file):

| Variable | Description | Default |
|----------|-------------|---------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | SigNoz endpoint | `http://localhost:4317` |
| `OTEL_EXPORTER_OTLP_HEADERS` | Headers (for cloud) | - |
| `OTEL_SERVICE_NAME` | Service name | `span-links-demo` |

## License

MIT

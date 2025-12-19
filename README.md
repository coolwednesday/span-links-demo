# Span Links Demo (SigNoz)

Go demos of OpenTelemetry span links, including queue/worker (backward links), optional forward-link demo, same-trace scatter/gather, and a remote-parent pitfall visualisation in SigNoz.

## Run (SigNoz Cloud)
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.<REGION>.signoz.cloud:443"
export OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<YOUR_KEY>"
export OTEL_SERVICE_NAME="span-links-demo"
go mod download
go run .
```

Tip: copy `ENV.example` → `.env` and edit it, then just run `go run .` (this repo auto-loads `.env` if present).

## Modes (root app)
- Backward links (default): runs **one batch of 10 orders**. Consumers link back to producer (backward links).
- Forward-link demo (single batch, same size):  
  `ENABLE_FORWARD_LINKS_TO_PRODUCER=true go run .`  
  Adds forward links from each `PublishOrder` to its matching `ProcessOrder`.

## Quick Decision Guide
- Parent-child (same trace): synchronous steps in one request.
- Span link, same trace: N:1 in one transaction (scatter/gather).
- Span link, different trace: async/queue/batch/retry/long-running/trust boundary.
- Forward links: only if you keep producer span open (inflates duration); backward is default for async.

## Project Layout
```
├── main.go / producer.go / worker.go / queue.go / otel.go / constants.go
├── docker-compose.yml
├── otel-collector-config.yaml
├── Makefile
└── examples/
    ├── fanout.go
    ├── fanin.go
    ├── retry.go
    ├── same_trace_span_links.go          # same-trace links (N:1)
    ├── README.md
    └── cmd/
        ├── fanout/main.go                # runnable fanout example
        ├── fanin/main.go                 # runnable fanin example
        ├── retry/main.go                 # runnable retry example
        ├── same_trace_span_links/main.go # runnable same-trace example
        └── remote-parent-gap/main.go     # parent-child async pitfall (remote context)
```

## View in SigNoz
- Traces → filter by `span-links-demo` (or `remote-parent-gap` for the pitfall).
- Open spans and check **Links** for backward/forward links.
- To find children from producer side, filter by `batch.id` or `order.id`.

## Running Examples

### Interactive Test Runner (Recommended)
Run the interactive test script that prompts for OTEL endpoint and headers, then executes all example programs:

```bash
./run_tests.sh
```

The script will:
1. Prompt for `OTEL_EXPORTER_OTLP_ENDPOINT` (SigNoz Cloud or local)
2. Prompt for `OTEL_EXPORTER_OTLP_HEADERS` (optional, needed for SigNoz Cloud)
3. Prompt for `OTEL_SERVICE_NAME` (optional, defaults to `span-links-demo`)
4. Run all example programs and verify they complete successfully

### Examples Run by the Script
- ✅ Same-trace scatter/gather (`examples/cmd/same_trace_span_links`)
- ✅ Fan-out pattern (`examples/cmd/fanout`)
- ✅ Fan-in pattern (`examples/cmd/fanin`)
- ✅ Retry pattern (`examples/cmd/retry`)
- ✅ Remote parent gap pitfall (`examples/cmd/remote-parent-gap`)
- ✅ Producer/consumer with backward links (main app, default mode)
- ✅ Producer/consumer with forward links (main app, `ENABLE_FORWARD_LINKS_TO_PRODUCER=true`)

### Manual Execution
Run individual examples manually:

```bash
# Set up environment
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.<REGION>.signoz.cloud:443"
export OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<YOUR_KEY>"

# Run examples
export OTEL_SERVICE_NAME="same-trace-span-links" && go run ./examples/cmd/same_trace_span_links
export OTEL_SERVICE_NAME="fanout" && go run ./examples/cmd/fanout
export OTEL_SERVICE_NAME="fanin" && go run ./examples/cmd/fanin
export OTEL_SERVICE_NAME="retry" && go run ./examples/cmd/retry
export OTEL_SERVICE_NAME="remote-parent-gap" && go run ./examples/cmd/remote-parent-gap

# Run main producer/consumer
export OTEL_SERVICE_NAME="span-links-demo" && go run .
export OTEL_SERVICE_NAME="span-links-demo-forward" && ENABLE_FORWARD_LINKS_TO_PRODUCER=true go run .
```

## License
MIT
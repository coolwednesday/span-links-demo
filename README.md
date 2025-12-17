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

## License
MIT
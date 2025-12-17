# Examples

This folder contains small, focused span-link demos plus runnable `cmd/` entrypoints.

## Configure export (SigNoz Cloud)

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingest.<REGION>.signoz.cloud:443"
export OTEL_EXPORTER_OTLP_HEADERS="signoz-ingestion-key=<YOUR_KEY>"
```

## Configure export (Local)

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"
unset OTEL_EXPORTER_OTLP_HEADERS
```

## Run the examples (recommended: use the cmd runners)

### Same-trace span links (scatter/gather)

```bash
export OTEL_SERVICE_NAME="same-trace-span-links"
go run ./examples/cmd/same_trace_span_links
```

What to look for in SigNoz:
- One trace with multiple shard spans + an aggregator span with links (same TraceID).

### Fan-out (one producer → many workers; different traces linked)

```bash
export OTEL_SERVICE_NAME="fanout"
go run ./examples/cmd/fanout
```

### Fan-in (many producers → one aggregator; different traces linked)

```bash
export OTEL_SERVICE_NAME="fanin"
go run ./examples/cmd/fanin
```

### Retry chain (attempts linked)

```bash
export OTEL_SERVICE_NAME="retry"
go run ./examples/cmd/retry
```

### Remote parent pitfall (parent-child across async via remote context)

```bash
export OTEL_SERVICE_NAME="remote-parent-gap"
go run ./examples/cmd/remote-parent-gap
```

What to look for in SigNoz:
- One trace where the parent ends immediately and the child starts later via remote parent context (gap / inflated apparent end-to-end duration).

## Source files (library-style examples)

These files expose functions you can call from your own `main` if you prefer:

- `fanout.go` — Fan-out: one producer → many workers (workers link back to producer)
- `fanin.go` — Fan-in: many producers → one aggregator (aggregator links to all producers)
- `retry.go` — Retry chain (attempt links to previous attempt)
- `same_trace_span_links.go` — Same-trace span links (scatter/gather within one trace)



#!/bin/bash

# Integration test runner for span-links-signoz-demo
# Prompts for OTEL endpoint and headers, then runs all example programs
# Verifies they complete successfully

echo "=========================================="
echo "Span Links Demo - Integration Test Runner"
echo "=========================================="
echo ""

# Prompt for OTEL endpoint
echo "Enter OTEL Exporter Endpoint:"
echo "  - SigNoz Cloud: https://ingest.<REGION>.signoz.cloud:443"
echo "  - Local SigNoz: http://localhost:4317"
echo "  - Leave empty for unit tests (no network calls)"
read -p "OTEL_EXPORTER_OTLP_ENDPOINT: " OTEL_ENDPOINT

if [ -n "$OTEL_ENDPOINT" ]; then
    export OTEL_EXPORTER_OTLP_ENDPOINT="$OTEL_ENDPOINT"
    echo "✓ Using endpoint: $OTEL_ENDPOINT"
else
    echo "✓ Using invalid endpoint (unit test mode)"
fi

echo ""

# Prompt for OTEL headers (optional, needed for SigNoz Cloud)
echo "Enter OTEL Exporter Headers (optional):"
echo "  - SigNoz Cloud: signoz-ingestion-key=<YOUR_KEY>"
echo "  - Local SigNoz: leave empty"
read -p "OTEL_EXPORTER_OTLP_HEADERS: " OTEL_HEADERS

if [ -n "$OTEL_HEADERS" ]; then
    export OTEL_EXPORTER_OTLP_HEADERS="$OTEL_HEADERS"
    echo "✓ Using headers: $OTEL_HEADERS"
else
    unset OTEL_EXPORTER_OTLP_HEADERS
    echo "✓ No headers (local SigNoz or unit test mode)"
fi

echo ""

# Prompt for service name (optional)
echo "Enter Service Name (optional, defaults to 'span-links-demo'):"
read -p "OTEL_SERVICE_NAME: " OTEL_SERVICE_NAME

if [ -n "$OTEL_SERVICE_NAME" ]; then
    export OTEL_SERVICE_NAME="$OTEL_SERVICE_NAME"
    echo "✓ Using service name: $OTEL_SERVICE_NAME"
else
    export OTEL_SERVICE_NAME="span-links-demo"
    echo "✓ Using default service name: span-links-demo"
fi

echo ""
echo "=========================================="
echo "Running All Examples..."
echo "=========================================="
echo ""

# Track failures
FAILED=0
PASSED=0

# Function to run a command and verify success
run_example() {
    local name=$1
    local cmd=$2
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Running: $name"
    echo "Command: $cmd"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    if eval "$cmd"; then
        echo "✓ PASSED: $name"
        ((PASSED++))
    else
        echo "✗ FAILED: $name"
        ((FAILED++))
    fi
    echo ""
}

# Run all example commands
run_example "Same Trace Span Links (Scatter/Gather)" \
    "export OTEL_SERVICE_NAME='same-trace-span-links' && go run ./examples/cmd/same_trace_span_links"

run_example "Fan-Out Example" \
    "export OTEL_SERVICE_NAME='fanout' && go run ./examples/cmd/fanout"

run_example "Fan-In Example" \
    "export OTEL_SERVICE_NAME='fanin' && go run ./examples/cmd/fanin"

run_example "Retry Example" \
    "export OTEL_SERVICE_NAME='retry' && go run ./examples/cmd/retry"

run_example "Remote Parent Gap (Pitfall Demo)" \
    "export OTEL_SERVICE_NAME='remote-parent-gap' && go run ./examples/cmd/remote-parent-gap"

# Run main producer/consumer with backward links (default)
run_example "Producer/Consumer - Backward Links" \
    "export OTEL_SERVICE_NAME='span-links-demo' && unset ENABLE_FORWARD_LINKS_TO_PRODUCER && go run ."

# Run main producer/consumer with forward links
run_example "Producer/Consumer - Forward Links" \
    "export OTEL_SERVICE_NAME='span-links-demo-forward' && export ENABLE_FORWARD_LINKS_TO_PRODUCER='true' && go run ."

echo "=========================================="
echo "Summary"
echo "=========================================="
echo "Passed: $PASSED"
echo "Failed: $FAILED"
echo ""

if [ $FAILED -eq 0 ]; then
    echo "✓ All examples completed successfully!"
    exit 0
else
    echo "✗ Some examples failed. Check output above."
    exit 1
fi

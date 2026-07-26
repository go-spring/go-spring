# starter-otel Example

Demonstrates OpenTelemetry tracing and log correlation.

## Features

- **Trace-Log correlation**: After creating a Span, logs automatically carry `trace_id`/`span_id`
- **Span export**: Spans are recorded and exported to the configured exporter

## Manual Testing

```bash
cd starter-otel/example
go run . -manual
```

The service keeps running. Press Ctrl+C to exit. Without `-manual`, `runTest()` executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.

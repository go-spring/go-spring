# starter-otel Example

Demonstrates OpenTelemetry tracing and log correlation for starter-otel.

## Features

- **Trace-Log correlation**: After creating a Span, logs automatically carry trace_id/span_id
- **Span export**: Spans are correctly recorded and exported to the configured exporter

## Manual Testing

```bash
cd starter-otel/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without -manual, runTest() executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
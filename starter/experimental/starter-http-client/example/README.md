# starter-http-client Example

Run this example to test the HTTP client for `starter-http-client`, including service discovery and distributed tracing.

## Features

- **Direct mode**: Call backend via fixed address
- **Service discovery**: Discover service instances via discovery
- **Distributed tracing**: Integrated OpenTelemetry, automatic traceparent propagation
- **Request/Response**: Verify Echo round-trip

## Manual Testing

```bash
cd starter-http-client/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without `-manual`, `runTest()` executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.

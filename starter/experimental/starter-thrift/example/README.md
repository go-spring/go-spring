# starter-thrift Example

Demonstrates Thrift RPC service registration, middleware decorators, and multi-protocol configuration.

## Features

- **RPC Calls**: Client round-trips via `Echo` method, compact protocol + framed transport
- **Middleware Decorator**: `loggingProcessor` wraps the generated Processor, logging every RPC call
- **Multiple Calls**: Two independent RPC calls, verifying middleware count = 2

## Manual Testing

Terminal 1, start the service:
```bash
cd starter-thrift/example
go run . -manual
```

The service listens on :9292 (compact protocol + framed transport). Connect and verify using Thrift-generated client code.

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
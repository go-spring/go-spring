# starter-grpc Example

Demonstrates gRPC service registration, interceptors, and health checks for starter-grpc.

## Features

- **gRPC call**: `Echo` method round-trip, request message returned as-is
- **Interceptor**: `LoggingInterceptor` logs method name and `x-app` metadata
- **Metadata**: Client sends `x-app`, server returns `x-handler`
- **Health check**: Standard `grpc_health_v1` health service

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-grpc/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
# Echo RPC
grpcurl -plaintext -d '{"message":"hello"}' \
  localhost:9494 EchoService/Echo
# -> {"message":"hello"}

# Health check
grpcurl -plaintext localhost:9494 grpc.health.v1.Health/Check
# -> {"status":"SERVING"}
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
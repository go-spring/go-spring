# starter-trpc Example

Demonstrates tRPC service registration and invocation using starter-trpc.

## Features

- **RPC call**: Client round-trips via the `Greet` method, sending a name and receiving a greeting
- **Service registration**: Binds `GreetServiceImpl` to the tRPC Server via `ServiceRegister`
- **Direct connection**: Uses `ip://` target scheme for direct connection, no registry required

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-trpc/example
go run . -manual
```

The service is listening on :8000. You can connect and verify using tRPC-generated client proxy code.

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

check.sh runs the example and waits for self-test to complete, exit code 0 means pass.

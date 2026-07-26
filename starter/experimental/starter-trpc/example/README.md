# starter-trpc Example

Demonstrates tRPC service registration and invocation.

## Features

- **RPC call**: Client round-trips via the `Greet` method, sending a name and receiving a greeting
- **Service registration**: Binds `GreetServiceImpl` to the tRPC Server via `ServiceRegister`
- **Direct connection**: Uses `ip://` target scheme for direct connection, no registry required

## Manual Testing

Terminal 1, start the service:
```bash
cd starter-trpc/example
go run . -manual
```

The service listens on :8000. Connect and verify using tRPC-generated client proxy code.

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
# starter-kitex Example

Demonstrates Kitex RPC service registration and invocation with starter-kitex.

## Features

- **RPC Invocation**: The client round-trips via the `Echo` method, which returns the request message as-is
- **Service Registration**: Bind `EchoServiceImpl` to the Kitex Server via `ServiceRegister`
- **Direct Connection Mode**: No registry required; the client connects directly via `host:port`

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-kitex/example
go run . -manual
```

The service listens on :8888. You can connect and verify with Kitex-generated client code.

Press `Ctrl+C` to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
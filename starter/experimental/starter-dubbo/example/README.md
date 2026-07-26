# starter-dubbo Example

Dubbo RPC service registration and invocation with starter-dubbo.

## Features

- **RPC Call**: Client calls the `Greet` method via the Triple protocol and verifies request/response correctness
- **Service Registration**: Registers `GreetProvider` as a Dubbo service via `RegisterService`

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-dubbo/example
go run . -manual
```

Terminal 2, run the client verification script:
```bash
go run check_client.go
```

Expected output:
```
Response from server: Hello, Dubbo-Go!
OK: Dubbo RPC verified
```

Press `Ctrl+C` to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
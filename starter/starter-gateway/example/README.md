# starter-gateway Example

API gateway proxying and filters with starter-gateway.

## Features

- **Reverse Proxy**: Forwards requests to backend upstream services
- **Request Filter**: `addRequestHeader` filter injects the `X-From` header
- **End-to-End Verification**: Starts an embedded backend and verifies the full proxy + filter pipeline

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-gateway/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl -i http://127.0.0.1:9440/api/echo
# -> HTTP/1.1 200 OK
# -> echo: /api/echo, from: go-spring-gateway
```

Press `Ctrl+C` to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
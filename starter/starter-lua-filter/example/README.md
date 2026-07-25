# starter-lua-filter Example

Run this example to test Lua script filtering with `starter-lua-filter`.

## Features

- **Lua Filter**: Intercept and modify HTTP requests/responses via Lua scripts
- **Request Interception**: Validates `X-App` header guard logic
- **Response Pass-through**: Legitimate requests pass through to business handlers

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-lua-filter/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl -H 'X-App: go-spring' http://127.0.0.1:9090/hello
# -> 200 OK

curl http://127.0.0.1:9090/hello
# -> 403 Forbidden
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.

# starter-echo Example

HTTP service routing, middleware, and health checks with starter-echo.

## Features

- **Routing**: `/echo/:name` path parameter + JSON response
- **Route Groups**: `/api/greet` query parameter route
- **Middleware**: Custom `X-App` response header injection
- **Built-in Middleware**: `X-Request-Id`, `X-Content-Type-Options` security headers
- **Health Check**: `/healthz` endpoint

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-echo/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl http://localhost:8002/echo/echo
# -> {"message":"Hello, echo"}

curl 'http://localhost:8002/api/greet?name=world'
# -> {"message":"Hi, world"}

curl -v http://localhost:8002/echo/echo 2>&1 | grep -i x-app
# -> X-App: go-spring

curl -v http://localhost:8002/echo/echo 2>&1 | grep -i x-request-id
# -> X-Request-Id: <uuid>

curl http://localhost:8002/healthz
# -> ok
```

Press `Ctrl+C` to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
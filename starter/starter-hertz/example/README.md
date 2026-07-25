# starter-hertz Example

Demonstrates HTTP service routing, middleware, and health checks for starter-hertz.

## Features

- **Routing**: `/echo/:name` path parameter + JSON response
- **Routing**: `/greet` query parameter + JSON response
- **Middleware**: Custom `X-App` response header injection
- **Built-in middleware**: `X-Request-Id`, `X-Content-Type-Options` security headers
- **Health check**: `/healthz` endpoint

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-hertz/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
curl http://localhost:8003/echo/hertz
# -> {"message":"Hello, hertz"}

curl 'http://localhost:8003/greet?name=world'
# -> {"message":"Hi, world"}

curl -v http://localhost:8003/echo/hertz 2>&1 | grep -i x-app
# -> X-App: go-spring

curl http://localhost:8003/healthz
# -> ok
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
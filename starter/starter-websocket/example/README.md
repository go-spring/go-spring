# starter-websocket Example

[English](README.md) | [中文](README_CN.md)

Demonstrates WebSocket text/JSON echo and middleware authentication.

## Features

- **Text echo**: `/echo` endpoint receives text messages and echoes them back, with subprotocol negotiation
- **JSON echo**: `/json` endpoint receives JSON messages and returns a greeting
- **Middleware authentication**: `X-App: go-spring` header check, returns 403 Forbidden when missing

## Manual Testing

Terminal 1, start the service:
```bash
cd starter-websocket/example
go run . -manual
```

Terminal 2, test with websocat. Every route is guarded by the `requireApp`
middleware, so the `X-App: go-spring` header is mandatory; `Origin` must also
match `spring.websocket.allowedOrigins`:
```bash
# Text echo on /echo
websocat ws://127.0.0.1:9696/echo -H "X-App: go-spring" -H "Origin: http://127.0.0.1:9696"
# Type any text, receive echo

# JSON echo on /json  (type {"name":"world"} -> {"message":"Hi, world"})
websocat ws://127.0.0.1:9696/json -H "X-App: go-spring" -H "Origin: http://127.0.0.1:9696"

# Without the headers the handshake is rejected with HTTP 403
websocat ws://127.0.0.1:9696/echo
```

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
# starter-websocket-coder Example

Demonstrates WebSocket text/JSON echo and middleware authentication using starter-websocket-coder (using the coder/websocket library).

## Features

- **Text echo**: `/echo` endpoint receives text messages and echoes them back, with subprotocol negotiation
- **JSON echo**: `/json` endpoint receives JSON messages and returns a greeting
- **Middleware authentication**: `X-App: go-spring` header check, returns 403 Forbidden when missing

## Manual Testing

Terminal 1, start the service and keep it running:
```bash
cd starter-websocket-coder/example
go run . -manual
```

Terminal 2, test with websocat:
```bash
websocat ws://127.0.0.1:9797/echo
# Type any text, receive echo
```

Press Ctrl+C to stop the service after verification.

## Smoke Test

```bash
./check.sh
```

check.sh runs the example and waits for self-test to complete, exit code 0 means pass.

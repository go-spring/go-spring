# starter-nats Example

Demonstrates NATS messaging client.

## Features

- **Connection Health**: Verify NATS connection health status
- **Multi-Connection Isolation**: Two independent connections, main and work
- **Message Publish/Subscribe**: Publish and subscribe to messages via NATS

> Requires NATS service running. `check.sh` starts NATS via docker compose.

## Manual Testing

```bash
cd starter-nats/example
go run . -manual
```

Expected output:
```
main connection: healthy
work connection: healthy
message published
message received
```

Start NATS first:
```bash
# Start NATS
docker compose up -d

# Run example (manual mode, keep running)
go run . -manual
```

The service keeps running. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts NATS via docker compose, runs the example and verifies messages, exit code 0 means pass.

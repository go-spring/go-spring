# starter-pulsar Example

Demonstrates Apache Pulsar messaging client for starter-pulsar.

## Features

- **Message publishing**: Sends messages to a Pulsar topic
- **Message consumption**: Subscribes to a topic and receives messages
- **Message acknowledgment**: Acknowledges (ack) after consumption

> Requires Pulsar service running. `check.sh` starts Pulsar via docker compose.

## Manual Testing

```bash
cd starter-pulsar/example
go run . -manual
```

Expected output:
```
subscribed
published
message received
```

Start Pulsar first:
```bash
# Start Pulsar
docker compose up -d

# Wait for Pulsar to be ready
sleep 15

# Run example
go run . -manual
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Pulsar via docker compose, runs the example and verifies messages, exit code 0 means pass.
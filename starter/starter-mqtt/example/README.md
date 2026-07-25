# starter-mqtt Example

Run this example to test the MQTT client with `starter-mqtt`.

## Features

- **Connection Status**: Verify MQTT broker connection health
- **Message Publishing**: Publish messages to MQTT topics
- **Message Subscription**: Subscribe to topics and receive messages

> Requires MQTT Broker running. `check.sh` starts Mosquitto via docker compose.

## Manual Testing

```bash
cd starter-mqtt/example
go run . -manual
```

Expected output:
```
connected
published
message received
```

Start MQTT Broker first:
```bash
# Start Mosquitto
docker compose up -d

# Run example (manual mode, keep running)
go run . -manual
```

Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Mosquitto via docker compose, runs the example and verifies messages, exit code 0 means pass.

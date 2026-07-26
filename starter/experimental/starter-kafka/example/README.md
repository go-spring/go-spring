# starter-kafka Example

Run this example to test Kafka message production and consumption with `starter-kafka`.

## Features

- **Message Publishing**: Send messages to a Kafka topic
- **Message Consumption**: Consume messages from a Kafka topic and verify content

> Requires Kafka service running. `check.sh` starts Kafka via docker compose.

## Manual Testing

```bash
cd starter-kafka/example
go run . -manual
```

Expected output:
```
published: value
consumed: value
```

Start Kafka first:
```bash
# Start Kafka
docker compose up -d

# Wait for Kafka to be ready
sleep 10

# Run the example (manual mode, keeps running)
go run . -manual
```

Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Kafka via docker compose, runs the example and verifies messages, exit code 0 means pass.

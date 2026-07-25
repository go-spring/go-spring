# starter-rabbitmq Example

Demonstrates the RabbitMQ messaging client of starter-rabbitmq.

## Features

- **Default Exchange**: Publish/consume messages via the default exchange
- **Queue Declaration**: Automatically declare queues
- **Message Acknowledgment**: Auto ack after consuming

> Requires RabbitMQ service running. `check.sh` starts RabbitMQ via docker compose.

## Manual Testing

```bash
cd starter-rabbitmq/example
go run . -manual
```

Expected output:
```
published to queue "hello"
consumed from queue "hello": value
```

Start RabbitMQ first:
```bash
# Start RabbitMQ
docker compose up -d

# Wait for RabbitMQ to be ready
sleep 10

# Run the example
go run . -manual
```

Open `http://127.0.0.1:15672` (guest/guest) in browser to view the management UI.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts RabbitMQ via docker compose, runs the example and verifies the messages, exit code 0 means pass.
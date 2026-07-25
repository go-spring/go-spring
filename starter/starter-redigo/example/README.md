# starter-redigo Example

Demonstrates the Redigo Redis client of starter-redigo.

## Features

- **String SET/GET**: Write and read string key-value pairs
- **Multi-Driver Support**: Switchable redigo/go-redis driver

> Requires Redis service running. `check.sh` starts Redis via docker compose.

## Manual Testing

```bash
cd starter-redigo/example
go run . -manual
```

Expected output:
```
SET foo bar: OK
GET foo: bar
```

Start Redis first:
```bash
# Start Redis
docker compose up -d

# Run the example
go run . -manual
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Redis via docker compose, runs the example and verifies the operations, exit code 0 means pass.
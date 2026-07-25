# starter-go-redis Example

Demonstrates the go-redis Redis client for starter-go-redis.

## Features

- **String SET/GET**: Write and read string key-value pairs
- **Hash operations**: HSET/HGET operations
- **List operations**: LPUSH/LRANGE operations
- **Multi-driver support**: Switch between go-redis/redigo drivers

> Requires Redis service running. `check.sh` starts Redis via docker compose.

## Manual Testing

```bash
cd starter-go-redis/example
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

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Redis via docker compose, runs the example and verifies operations, exit code 0 means pass.
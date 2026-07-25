# starter-memcached Example

Demonstrates the Memcached client with starter-memcached.

## Features

- **String SET/GET**: Write and read string key-value pairs
- **Multi-Driver Support**: Switch between different Memcached drivers

> Requires Memcached service running. check.sh starts Memcached via docker compose.

## Manual Testing

```bash
cd starter-memcached/example
go run . -manual
```

Expected output:
```
SET foo bar: OK
GET foo: bar
```

Start Memcached first:
```bash
# Start Memcached
docker compose up -d

# Run example (manual mode, keep running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

check.sh starts Memcached via docker compose, runs the example and verifies operations, exit code 0 means pass.
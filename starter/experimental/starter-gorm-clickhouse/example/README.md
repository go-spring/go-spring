# starter-gorm-clickhouse Example

Run this example to test ClickHouse connectivity via `starter-gorm-clickhouse`.

## Features

- **Database connection**: Connect to ClickHouse via GORM
- **Version query**: Run `SELECT version()` to verify connectivity

> Requires ClickHouse service running. `check.sh` starts ClickHouse via docker compose.

## Manual Testing

```bash
cd starter-gorm-clickhouse/example
go run . -manual
```

Expected output:
```
ClickHouse version: ...
```

Start ClickHouse first:
```bash
# Start ClickHouse
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts ClickHouse via docker compose, runs the example and verifies connectivity, exit code 0 means pass.

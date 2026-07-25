# starter-gorm-postgres Example

Demonstrates PostgreSQL database connectivity for starter-gorm-postgres.

## Features

- **Database connection**: Connect to PostgreSQL via GORM
- **Version query**: Run `SELECT version()` to verify connectivity

> Requires PostgreSQL service running. `check.sh` starts PostgreSQL via docker compose.

## Manual Testing

```bash
cd starter-gorm-postgres/example
go run . -manual
```

Expected output:
```
PostgreSQL version: ...
```

Start PostgreSQL first:
```bash
# Start PostgreSQL
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts PostgreSQL via docker compose, runs the example and verifies connectivity, exit code 0 means pass.
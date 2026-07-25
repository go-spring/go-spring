# starter-gorm-mysql Example

Demonstrates MySQL database connectivity for starter-gorm-mysql.

## Features

- **Database connection**: Connect to MySQL via GORM
- **Version query**: Run `SELECT VERSION()` to verify connectivity

> Requires MySQL service running. `check.sh` starts MySQL via docker compose.

## Manual Testing

```bash
cd starter-gorm-mysql/example
go run . -manual
```

Expected output:
```
MySQL version: ...
```

Start MySQL first:
```bash
# Start MySQL
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts MySQL via docker compose, runs the example and verifies connectivity, exit code 0 means pass.
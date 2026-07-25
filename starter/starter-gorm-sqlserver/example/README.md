# starter-gorm-sqlserver Example

Demonstrates SQL Server database connectivity for starter-gorm-sqlserver.

## Features

- **Database connection**: Connect to SQL Server via GORM
- **Version query**: Run `SELECT @@VERSION` to verify connectivity

> Requires SQL Server service running. `check.sh` starts SQL Server via docker compose.

## Manual Testing

```bash
cd starter-gorm-sqlserver/example
go run . -manual
```

Expected output:
```
SQL Server version: ...
```

Start SQL Server first:
```bash
# Start SQL Server
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts SQL Server via docker compose, runs the example and verifies connectivity, exit code 0 means pass.
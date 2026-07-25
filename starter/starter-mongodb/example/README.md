# starter-mongodb Example

Run this example to test the MongoDB client with `starter-mongodb`.

## Features

- **Health Check**: Verify MongoDB cluster connectivity
- **CRUD Operations**: Insert documents, query documents
- **Connection Pool**: Connection pool configuration and management

> Requires MongoDB service running. `check.sh` starts MongoDB via docker compose.

## Manual Testing

```bash
cd starter-mongodb/example
go run . -manual
```

Expected output:
```
health check: OK
document inserted
document found
```

Start MongoDB first:
```bash
# Start MongoDB
docker compose up -d

# Run example (manual mode, keep running)
go run . -manual
```

Press Ctrl+C to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts MongoDB via docker compose, runs the example and verifies operations, exit code 0 means pass.

# starter-elasticsearch Example

Demonstrates the Elasticsearch client with starter-elasticsearch.

## Features

- **Cluster Connectivity**: Verifies ES cluster connection via HealthCheck
- **Index Operations**: Create index, write document, query document

> Requires a running Elasticsearch service. `check.sh` starts Elasticsearch via docker compose.

## Manual Testing

```bash
cd starter-elasticsearch/example
go run . -manual
```

Expected output:
```
cluster health: green
document indexed
document found
```

Start Elasticsearch first:
```bash
# Start ES
docker compose up -d

# Run example (manual mode, keep running)
go run . -manual
```

The service keeps running, you can verify with the corresponding CLI tools. Press `Ctrl+C` to exit the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Elasticsearch via docker compose, runs the example and verifies operations, exit code 0 means pass.
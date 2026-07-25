# starter-neo4j Example

Demonstrates Neo4j graph database client for starter-neo4j.

## Features

- **Connection health**: Verifies Neo4j database connectivity
- **Cypher queries**: Executes Cypher query statements
- **Multi-driver support**: Supports switching between different Neo4j drivers

> Requires Neo4j service running. `check.sh` starts Neo4j via docker compose.

## Manual Testing

```bash
cd starter-neo4j/example
go run . -manual
```

Expected output:
```
health check: OK
query executed
```

Start Neo4j first:
```bash
# Start Neo4j
docker compose up -d

# Run example
go run . -manual
```

Open `http://127.0.0.1:7474` in browser to view Neo4j Browser.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Neo4j via docker compose, runs the example and verifies the query, exit code 0 means pass.

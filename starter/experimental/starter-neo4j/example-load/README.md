# starter-neo4j / example-load

Load-test binary for starter-neo4j. Drives a MERGE + count Cypher round-trip
through a Neo4j driver (resilience + fault hot-reloadable) using the shared
`cloud/experimental/loadtest` harness, printing throughput / latency percentiles / error
breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a Neo4j
```

## Set fire

Edit `conf/app.properties`:

```properties
fault.enabled=true
fault.rate=0.5
fault.error=generic       # or: timeout / reset
```

then re-run — the error breakdown's `injected`/`circuit` counts climb and p99
rises as retries stack against the injected faults.

## Notes

- The op is `MERGE (n:LoadNode {id:$id}) RETURN count(n)` — an idempotent write
  plus a read in a single Cypher call; the node is reused across iterations.
- Resilience for Neo4j is enforced at the dial layer (the Bolt-protocol driver
  exposes no per-query hook), so the breaker/limiter protects connection
  acquisition, not in-flight queries.

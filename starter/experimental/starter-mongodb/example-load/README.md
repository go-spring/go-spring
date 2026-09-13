# starter-mongodb / example-load

Load-test binary for starter-mongodb. Drives an upsert + findOne round-trip on a
MongoDB collection (resilience + fault hot-reloadable) using the shared
`cloud/experimental/loadtest` harness, printing throughput / latency percentiles / error
breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a MongoDB
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

- The op is idempotent: `UpdateOne` upserts `{_id:"load", v:"v"}`, then `FindOne`
  reads it back — safe to hammer and leaves a single doc behind.
- Resilience for MongoDB is enforced at the dial layer (the v2 driver exposes no
  per-command hook), so the breaker/limiter protects connection establishment.

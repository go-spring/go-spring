# starter-elasticsearch / example-load

Load-test binary for starter-elasticsearch. Drives an Index + Get round-trip on
an Elasticsearch index (resilience + fault hot-reloadable) using the shared
`cloud/experimental/loadtest` harness, printing throughput / latency percentiles / error
breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against an ES node
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

- The op indexes document id "1" (no refresh, for throughput) then reads it back
  via Get — idempotent and leaves a single doc behind.
- Resilience for Elasticsearch is enforced at the HTTP transport layer (the
  client has no per-operation hook), so the breaker/limiter protects HTTP
  requests end-to-end.

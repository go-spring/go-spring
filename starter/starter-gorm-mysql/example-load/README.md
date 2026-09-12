# starter-gorm-mysql / example-load

Load-test binary for starter-gorm-mysql. Drives `SELECT 1` round trips through
a gorm MySQL client (resilience + fault hot-reloadable) using the shared
`cloud/loadtest` harness, printing throughput / latency percentiles / error
breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a MySQL
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

- The `op` is a `SELECT 1` round trip (via `db.WithContext(ctx).Raw(...).Scan`).
  The load-test targets client-side overhead (resilience / fault / observability
  callbacks in the gorm processor chain), not raw DB throughput, so a trivial
  query isolates the client path cleanly.
- Connection keys (`spring.gorm.mysql.instances.load.*`) live under the instance prefix;
  `resilience.*` and `fault.*` are top-level absolute property refs shared
  across all starters.

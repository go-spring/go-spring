# starter-gorm-clickhouse / example-load

Load-test binary for starter-gorm-clickhouse. Drives `SELECT 1` round trips
through a gorm ClickHouse client (resilience + fault hot-reloadable) using the
shared `cloud/loadtest` harness, printing throughput / latency percentiles /
error breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a ClickHouse
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
  ClickHouse is a column store whose gorm driver's AutoMigrate/Create semantics
  differ from row-store dialects (table ENGINE, no auto-increment PK, batch-oriented
  inserts), so the load keeps the op a pure read round trip to stay portable.
  The load-test targets client-side overhead (resilience / fault / observability
  callbacks in the gorm processor chain), not raw DB throughput.
- The driver dials the **native TCP port 9000** (not the HTTP port 8123); this
  mirrors the starter's existing `example/conf/app.properties`.
- Connection keys (`spring.gorm.clickhouse.instances.load.*`) live under the instance
  prefix; `resilience.*` and `fault.*` are top-level absolute property refs
  shared across all starters.
- Assumption: the `clickhouse/clickhouse-server` default image ships a passwordless
  `default` user and `default` database (verified against the starter's existing
  example). If your image requires auth, set `spring.gorm.clickhouse.instances.load.password`.

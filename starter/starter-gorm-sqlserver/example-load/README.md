# starter-gorm-sqlserver / example-load

Load-test binary for starter-gorm-sqlserver. Drives `SELECT 1` round trips
through a gorm SQL Server client (resilience + fault hot-reloadable) using the
shared `cloud/loadtest` harness, printing throughput / latency percentiles /
error breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a SQL Server
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
- The `SA_PASSWORD=RootRoot1` meets the SQL Server 2022 complexity policy
  (upper + lower + digit). The `db=master` uses the built-in system database so
  no extra CREATE DATABASE step is needed.
- Connection keys (`spring.gorm.sqlserver.instances.load.*`) live under the instance
  prefix; `resilience.*` and `fault.*` are top-level absolute property refs
  shared across all starters.
- SQL Server is slow to bootstrap in docker (the watchdog in `check.sh` allows
  90s; the TCP-wait loop allows 60s of retries).

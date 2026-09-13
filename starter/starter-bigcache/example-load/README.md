# starter-bigcache / example-load

Load-test binary for starter-bigcache. Drives SET/GET through a BigCache
instance (resilience + fault hot-reloadable) using the shared `cloud/experimental/loadtest`
harness, printing throughput / latency percentiles / error breakdown.

BigCache is an embedded in-process cache, so there is no external service to
bring up — `check.sh` just runs the binary.

## Run

```bash
./check.sh                                   # smoke (no docker needed)
go run . -concurrency=32 -duration=10s       # or run directly
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

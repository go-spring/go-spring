# starter-memcached / example-load

Load-test binary for starter-memcached. Drives SET/GET through a Memcached
client (resilience + fault hot-reloadable) using the shared `cloud/experimental/loadtest`
harness, printing throughput / latency percentiles / error breakdown.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a Memcached
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

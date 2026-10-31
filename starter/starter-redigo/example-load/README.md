# starter-redigo / example-load

A **load-test** binary for starter-redigo. It builds a redigo pool armed with
resilience + fault (both hot-reloadable), fans out N workers running SET/GET
against a real Redis for a fixed duration, and prints throughput, latency
percentiles, and an error breakdown that distinguishes
`circuit-open / rate-limited / bulkhead / injected / other`.

It is the load-test companion to `cloud/governance/capability/fault`: setting a
`spring.fault.client.rate` in `conf/governance.properties` makes the
closed loop — fault → resilience → observe — visible in the numbers.

## Run

```bash
./check.sh                                   # docker-gated smoke (needs docker)
go run . -concurrency=32 -duration=10s       # or run directly against a Redis
go run . -manual                             # keep up for ad-hoc probing
```

## Set fire

Edit `conf/governance.properties`:

```properties
spring.fault.client.rate=0.5
spring.fault.client.error=generic       # or: timeout / reset
```

then re-run. Expect the error breakdown's `injected`/`circuit` counts to climb
and p99 to rise as the configured retries stack up against the injected faults.

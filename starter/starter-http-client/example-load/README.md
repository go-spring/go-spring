# starter-http-client / example-load

Load-test binary for starter-http-client. It runs an in-process HTTP backend
(no docker), wires a declarative http-client against it, and drives GETs through
the shared `cloud/experimental/loadtest` harness.

## Run

```bash
./check.sh                                   # no docker needed — builds and runs
go run . -concurrency=8 -duration=5s         # or run directly
```

## Set fire

Edit `conf/governance.properties`:

```properties
spring.governance.client.fault.rate=0.5
spring.governance.client.fault.error=generic       # or: timeout / reset
```

then re-run — the error breakdown's `injected` count climbs as a fraction of
GETs are made to fail.

## Notes

- For http-client, `resilience.*` / `fault.*` are **not** bound under the instance
  prefix: they live in the governance rules document (`spring.governance.client.*`),
  matched by the client's service label — the same place every other client starter
  reads them from. The instance Config carries only transport knobs (addr, tls, …).
- This example demonstrates fault in **fault-only mode** (resilience off): fault
  wraps a zero-policy executor and injects into the RoundTripper directly. The
  interaction between the resilience RoundTripper and the httpx declarative
  transport is a separate, pre-existing issue (not related to fault wiring) and
  is not exercised here.


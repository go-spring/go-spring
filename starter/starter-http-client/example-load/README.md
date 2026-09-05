# starter-http-client / example-load

Load-test binary for starter-http-client. It runs an in-process HTTP backend
(no docker), wires a declarative http-client against it, and drives GETs through
the shared `cloud/loadtest` harness.

## Run

```bash
./check.sh                                   # no docker needed — builds and runs
go run . -concurrency=8 -duration=5s         # or run directly
```

## Set fire

Edit `conf/app.properties`:

```properties
spring.http-client.load.fault.enabled=true
spring.http-client.load.fault.rate=0.5
spring.http-client.load.fault.error=generic       # or: timeout / reset
```

then re-run — the error breakdown's `injected` count climbs as a fraction of
GETs are made to fail.

## Notes

- For http-client, `resilience.*` / `fault.*` live **under the instance prefix**
  (`spring.http-client.load.*`) because they are fields of the ctor-bound Config,
  unlike redigo's field-injected `gs.Dync` (which is a top-level absolute ref).
- This example demonstrates fault in **fault-only mode** (resilience off): fault
  wraps a zero-policy executor and injects into the RoundTripper directly. The
  interaction between the resilience RoundTripper and the httpx declarative
  transport is a separate, pre-existing issue (not related to fault wiring) and
  is not exercised here.


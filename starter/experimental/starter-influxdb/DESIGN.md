# starter-influxdb Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A Client-archetype starter (`starter/DESIGN.md` §2.2) for InfluxDB 2.x.
Structurally a sibling of starter-s3 (HTTP-transport client with a
`dynamicTransport` Init arms); the InfluxDB-specific decisions are the write
paths and the async error drain.

## 1. Responsibilities & Boundaries

- **Owns**: bean lifecycle for the `spring.influxdb.<name>` group, the
  fail-fast `/health` probe and per-instance indicator, the observe transport,
  the resilience-guarded blocking write, the managed async writer (creation +
  error drain + shutdown flush).
- **Does not own**: Flux query building (raw Flux passes through), bucket/org
  administration, downsampling tasks, and v1-compatibility mode (a custom
  driver concern).

## 2. Key Abstractions & Seams

- **`Client` wrapper bean** — embeds `influxdb2.Client`. The SDK fixes the
  `*http.Client` at construction
  (`Options.SetHTTPClient`), so DefaultDriver installs a `dynamicTransport`
  indirection and Init swaps the observe+resilience round-tripper in — the
  same mechanism starter-s3/starter-elasticsearch use.
- **`WritePoints` vs `ManagedWriteAPI`** — the two write shapes the SDK
  offers, kept as two explicit methods instead of one policy flag: blocking
  writes are per-call and belong under the governance executor; buffered
  writes batch on a background goroutine whose retries are the SDK's own, and
  guarding them per point would double-count. `ManagedWriteAPI`'s
  `Errors()` channel is drained into go-spring's log because an undrained
  channel blocks the writer on its first failure.
- **`HealthError`** — the `/health` status mapping shared by the fail-fast
  probe and the indicator lives in the `health` subpackage, exported once.

## 3. Constraints

- `org`/`bucket` are required for the write helpers but not for connection —
  a client configured without them still serves Query/Delete APIs; the
  helpers fail with a pointed message instead of at wiring time.
- The async path is unguarded by design (see §2); its failures surface as
  log lines, not caller errors.
- `/health` on an uninitialized OSS server reports differently — the probe
  accepts only `pass`, so bootstrap-order races surface at boot rather than
  as intermittent write failures.

## 4. Trade-offs / Alternatives Rejected

- **Two explicit write methods vs. one configurable writer**: a single
  `WriteAPI` with a `mode` config would hide that failure semantics differ
  fundamentally (per-call error vs. logged background failure); explicit
  methods make the contract visible at the call site.
- **Drain errors into the log vs. expose the channel**: exposing
  `Errors()` through the wrapper invites the classic deadlock (nobody
  drains); the starter drains by default and callers needing custom handling
  use the embedded client's `WriteAPI` directly.
- **Query-time resilience**: `QueryRaw` was left unguarded (the blocking
  write is the overload-sensitive path); adding a `GuardedQuery` later is
  additive, not breaking.

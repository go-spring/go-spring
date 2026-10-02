# starter-milvus Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A Client-archetype starter for Milvus (vector database) over gRPC.

## 1. Responsibilities & Boundaries

- **Owns**: per-instance wiring, the per-RPC governance guard (gRPC client
  interceptors on the SDK dial options), the declaration of each RPC's
  observability identity, the fail-fast `HealthCheck` (`ListCollections`) probe,
  the health indicator, connection teardown.
- **Does not own**: schema design (collection/field/index are the app's),
  search/query semantics, and the emission of per-RPC signals — the resilience
  layer inside the executor emits those (see §4).

## 2. Key Abstractions & Seams

- **Client wrapper** — embeds the SDK's `client.Client` interface, so its full
  surface is promoted; the bean is a thin holder because the SDK's own client is
  already the full surface.
- **Guard interceptors** — a unary and a stream-open gRPC interceptor route every
  RPC through the resilience executor. They are also the **declaration seam**:
  each puts the RPC's identity on the caller's context before the executor runs
  (`observe.go`), so the single emitter inside the executor names the span, the
  `db.client.*` metrics and the access log. The starter emits nothing itself.
- **Fail-fast probe** — `HealthCheck` (`ListCollections`) at construction when the
  instance sets `ping=true`; a wrong address or bad credential then fails at boot,
  not on first query (the probe is off by default, so it fails on first query).

## 3. Constraints

- gRPC-only; there is no HTTP transport to swap.

## 4. Trade-offs / Alternatives Rejected

- **Declaration here, emission in the resilience layer** — the guard's
  interceptors are the natural per-RPC seam (they ride the transport the SDK
  actually uses), so per-operation resilience and observability need no facade
  over the whole `client.Client` interface. The starter only declares what each
  RPC is (`observe.go`); the resilience layer emits the call-level
  `db.client.operation.duration` and attempt-level `db.client.attempt.duration`
  histograms plus the `_app_milvus_access` log from the one point that sees a
  whole call, retries included. The rejected alternative — a hand-written facade
  that wraps every vector op itself — would duplicate that seam and re-emit
  signals the executor already produces.
- **No TLS/auth dial-option escape hatch** — `Config` exposes no way to append
  custom dial options, so an app needing extra interceptors (auth tokens, custom
  tracing) must fork `newClient`.

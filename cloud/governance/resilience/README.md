# resilience

[English](README.md) | [中文](README_CN.md)

`resilience` is a framework-agnostic abstraction for client-side fault
tolerance: rate limiting, circuit breaking, bulkhead isolation, retry,
per-attempt timeout, and fallback. Client starters plug the single
`ClientExecutor` seam into their own request hook (HTTP RoundTripper, Redis
Hook, GORM plugin, ...).

## Features

- `ClientPolicy` fields (OUTBOUND): `RateLimit` / `Burst` / `Algorithm` / `Window` /
  `RateLimitMaxWait`, `ErrorThreshold` / `OpenDuration`, `MaxConcurrent`,
  `MaxRetries`, `AttemptTimeout`.
- `ServerPolicy` fields (INBOUND): the same limiter / breaker / bulkhead / budget
  knobs, minus the retry family — a handler that already produced side effects
  cannot be replayed, so `ServerPolicy` has no way to express a retry (and no
  endpoint selection either: there is no endpoint to choose). Two models rather
  than one with dead fields, so the constraint is structural.
- Neutral rejection errors: `ErrRateLimited`, `ErrCircuitOpen`,
  `ErrBulkheadFull`.
- Bundled `"default"` driver — in-process, zero dependencies. Recommended
  production driver `sentinel` lives in `starter/starter-governance-sentinel`.
- Two client seams for opt-in adaptation:
  - `NewRoundTripper` — HTTP client `http.RoundTripper` (widest coverage).
  - `NewDialer` — connection-level `DialFunc` (matches
    a dial closure over a round-robin pick pool).
- Inbound admission runs on the SAME engine, through its own seam:
  `Manager.ServerExecutorFor` yields an `ServerExecutor` built by
  `Driver.NewServerExecutor`, and protocol starters build their middleware on it
  (429 / 503 on rejection; see starter-gin / starter-grpc admission). The two
  directions read separate resolvers, so an outbound policy change never
  retunes inbound admission.
- `Fallback(ctx, exec, fn, degrade)` — graceful degradation
  helper that composes with any executor.
- Rate limiting is a stage of a protected call, not an API of its own: the
  counters are the executor's own budget for its service, or, with a
  `Counters` store handed in (Redis, in `starter-go-redis`), one budget
  across replicas.

## Pluggable backends

This package holds **no registry**. An engine backend is a bean named after
itself and exported as `Driver`; the consumer collects them into a name-keyed
directory the center resolves the configured name against. `Driver` has TWO
methods — `NewClientExecutor(service, Policy)` for outbound and
`NewServerExecutor(service, ServerPolicy)` for inbound — because the two directions do
not share a model, and a sentinel-style backend maps them onto different native
primitives. One object still answers for both, so `spring.governance.driver=sentinel`
remains a single key that switches the whole process. The bundled engine answers
to `"default"` without a bean, via `NewDefaultDriver(nil)`; it projects an
`ServerPolicy` onto its `ClientPolicy` engine (`ServerPolicy.AsPolicy`), which is exact for
it because it has no inbound-only primitives.

```go
// contribute a backend (typically from a starter's init)
gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
    Name("sentinel").
    Export(gs.As[resilience.Driver]())
```

The `Export` is load-bearing: gs indexes beans by their exact type, so a
concrete driver without it is invisible to the directory.

The counter store is a backend too, but an OPTIONAL one — how wide a rate limit
reaches, nothing more. With no `Counters` bean in the container, each executor
counts in a budget of its own, exactly like its breaker and bulkhead state: an
executor is built once per service label, so one budget still covers every
caller of that label. Contribute a store over a shared backend (Redis) and the
driver injects it, so the limit holds across replicas — one budget per service.
A store reaches only the bundled engine's executors: an engine that brings its
own flow control (sentinel) counts on its own and ignores it.

## Installation

```
go get go-spring.org/cloud
```

## Usage

Guard an HTTP client:

```go
import (
    "net/http"

    "go-spring.org/cloud/governance/resilience"
)

exec, _ := resilience.NewDefaultDriver(nil).NewClientExecutor("http:orders", resilience.ClientPolicy{
    RateLimit:      100,
    ErrorThreshold: 5,
    MaxRetries:     2,
    AttemptTimeout: 2 * time.Second,
})

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec),
}
```

Combine with `cloud/discovery` at the dial layer:

```go
// dialOf reads a live endpoint and dials it: the closure a discovery
// Resolver + loadbalance Pool composes into.
dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
    ep, err := pool.Pick(loadbalance.PickInfo{})
    if err != nil { return nil, err }
    return (&net.Dialer{}).DialContext(ctx, network, ep.Addr)
}
dial = resilience.NewDialer(dial, exec)
```

A bare budget check is the same call with only the rate-limit stage
configured — the empty body is the whole work being admitted:

```go
exec := mgr.ClientExecutorFor("gateway", "route:orders")
err := exec.Execute(ctx, func(context.Context) error { return nil })
if errors.Is(err, resilience.ErrRateLimited) { /* 429 */ }
```

---

# Design

`resilience` is the client-side fault-tolerance abstraction of the governance
family (`go-spring.org/cloud/governance/resilience`). It defines the neutral
contract every adapter and driver satisfies, and ships an in-tree driver so
the framework works out of the box; production installs typically swap in the
sentinel driver from `starter/starter-governance-sentinel`.

## 1. Responsibilities & Boundaries

- **Does:** define `ClientPolicy`, `ClientExecutor`, `Driver`, `Counters`, `Algorithm`;
  provide the built-in `default` driver; expose the client adapters
  (`NewRoundTripper`, `NewDialer`); offer a `Fallback` composition helper.
- **Refuses:**
  - No universal per-request seam. Every client library exposes a
    different call-time hook; the package supplies the two most reusable
    (`http.RoundTripper`, `DialFunc`) and lets each client
    adapter wire the executor into whatever hook it has (redis.Hook,
    gorm plugin, ...). §4 has the reasoning.
  - No metrics, no tracing, no logging. Adapters and drivers decide how to
    surface state.
  - No third-party dependencies. The recommended sentinel driver lives in
    its own module so the framework itself stays stdlib-only.

## 2. Key Abstractions / Seams

- **Two-tier abstraction: `ClientPolicy` + `Driver` + `ClientExecutor`.** `ClientPolicy` is a
  backend-neutral, declarative wish list; `Driver.NewClientExecutor(service, Policy)`
  builds a concrete backend runtime. The bundled builtin driver reads the
  policy directly; a sentinel driver translates it into sentinel-golang's
  flow/circuit-breaker rules. Adapters depend only on `ClientExecutor`.
- **The container is the driver directory.** This package holds no registry:
  a backend is contributed as a bean named after itself, exported as
  `Driver`, and the governance wiring bean collects every such bean into a
  name-keyed map. `spring.governance.driver` selects one, resolved through
  that directory — the same "container as directory" shape `discovery`
  and the client starter `Driver`s use. The bundled builtin answers to
  `"default"` without a bean, so nothing has to contribute it.
- **Neutral rejection errors** (`ErrRateLimited`, `ErrCircuitOpen`,
  `ErrBulkheadFull`) let adapters make protocol-specific decisions (429
  vs 503 in a protocol starter's admission middleware) without importing
  a driver package.
- **Two client adapter seams cover the practical space:**
  - `NewRoundTripper` — widest coverage; any `*http.Client` gains
    protection by swapping its `Transport`. Retries clone with the rewound
    body (`Request.GetBody`); 5xx responses count as failures for the
    breaker.
  - `NewDialer` — coarser but universal at the connection layer; pairs
    naturally with a dial closure over a round-robin pick pool. Service is fixed
    because a dialer is already scoped to one service.

  Inbound **middleware** is not in this package: each protocol starter
  builds its own on the `resilience.Manager.ServerExecutorFor` seam (see
  starter-gin / starter-grpc admission), mapping neutral rejections to
  429/503 and serving each request exactly once.
- **`Fallback` is a helper, not an interface method.** Adding a `degrade`
  parameter to `Executor.Execute` would ripple through every driver and
  adapter; a standalone helper composes with any executor (including nil)
  and keeps the core surface small.
- **Rate limiting is a stage, not a seam of its own.** It runs inside
  `Execute`, so the caller never builds a limiter: it configures the stage
  through `ClientPolicy` and reads the outcome through `ErrRateLimited`. What
  makes a limit local or global is where its counters live — the executor's
  own budget, or a `Counters` store — not a second API: by default one
  executor holds one budget for its service, a shared backend (Redis)
  counts per fleet.
- **Counters are scoped by the service an executor is bound to.** One
  budget covers that service for every caller of it, which is what makes a
  limit protect a downstream rather than one client of it.

## 3. Constraints

- Nil executor / nil transport / empty policy is a transparent pass-through
  across every adapter and helper. Wiring stays free until a policy is
  configured.
- Adapters must expose an `io.Closer` on their transport so a starter's
  destroy hook can release the executor. `roundTripper.Close` implements
  that already.
- `runOnce`'s per-attempt timeout must derive from the caller's context,
  never the background context — cancellation must propagate.
- Inbound admission middlewares (built by the protocol starters) must guard
  against retry re-invocation of an already-served request; the response is
  committed after the first `Write`.
- Under the builtin driver, the bulkhead is held across retries (one slot
  per Execute, not per attempt) — a slow downstream must not be amplified.
  The sentinel driver upholds the same invariant by registering its
  isolation rule under a `$bulkhead`-suffixed service and acquiring that
  Entry once per Execute (its flow + breaker Entry stays per-attempt).
- `redis.Nil` / `gorm.ErrRecordNotFound`-style "no data" errors from a
  client adapter must not feed the breaker; the adapter maps them to
  success before returning through `Execute`.
- Half-open admits exactly one trial under the builtin driver (a
  single-permit channel, not a boolean flag, so two concurrent callers
  arriving right after cool-down cannot both be admitted). The sentinel
  driver approximates this with `ProbeNum=1`; sentinel's own probe
  accounting does not guarantee strict single-admission under tight
  concurrency, which is the one residual behavioral difference between
  the two drivers.
- Retry is paced by explicit backoff fields (`InitialInterval`,
  `Multiplier`, `MaxInterval`, `RandomizationFactor`) — a zero
  `InitialInterval` keeps the legacy back-to-back retry. Whether an error
  is retried is decided by `Policy.ShouldRetry`: a `Retryable`-implementing
  error wins; otherwise every error retries. A `MaxDuration` caps the whole
  Execute across retries; the per-attempt `AttemptTimeout` is applied inside
  that budget.
- Breaker strategy is selectable: `consecutive` (default, the historical
  consecutive-count) or `error-rate` (trips on failure ratio over
  `BreakerWindow` once `MinRequests` is reached). Both drivers honor the
  same selection so switching backends does not silently change which
  failures trip the breaker.

## 4. Trade-offs / Alternatives Rejected

- **No single universal per-request seam.** HTTP clients have
  `RoundTripper`, but redis-go has `redis.Hook`, GORM has plugin callbacks,
  and MQ producers vary by library. Trying to unify these under one
  `Interceptor` interface would break down at the first library whose hook
  is call-site-only (NATS, pulsar). The chosen answer is a small, shared
  `ClientExecutor` core plus a family of hand-written adapters — the analogous
  layering to `discovery` + `Resolver`.
- **Executor over decorator per stage.** A single `Execute` bundles rate
  limit + breaker + bulkhead + retry + timeout in one call so per-service
  state (token bucket / breaker / semaphore) is coherent. Decorating each
  stage independently would double-count retries against the limiter and
  make retry-inside-breaker semantics ambiguous.
- **Duplicate breaker with `loadbalance.Tracker`.** Different job (LB
  eviction is a queryable candidate-set filter, resilience rejects a call
  in flight). Feeding both from the same per-call error keeps them
  consistent without a code dependency.
- **No sliding-window in the Redis counter store (elsewhere).** Only token
  bucket is cleanly expressible as an atomic Lua script; sliding-window
  would either race or need heavy per-key data. The executor's own budget
  offers both; the Redis store in `starter-go-redis` is intentionally
  token-bucket only, and counts a sliding-window scope as a token bucket.
- **No retry inside inbound admission.** Retrying an already-written response is
  impossible; retry is only meaningful on the client seams.

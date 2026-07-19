# resilience Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`resilience` is the client-side fault-tolerance abstraction of the stdlib
(zero-dependency foundation) layer. It defines the neutral contract every
adapter and driver satisfies, and ships an in-tree driver so the framework
works out of the box; production installs typically swap in the sentinel
driver from `starter/starter-resilience`.

## 1. Responsibilities & Boundaries

- **Does:** define `Policy`, `Executor`, `Driver`, `RateLimiter`,
  `LimiterDriver`; provide the built-in `default` driver; expose three
  opt-in adapters (`NewRoundTripper`, `NewDialer`, `NewHandler`); offer a
  `Fallback` composition helper; ship a limiter registry with in-process
  built-ins (token bucket + sliding window).
- **Refuses:**
  - No universal per-request seam. Every client library exposes a
    different call-time hook; the package supplies the three most reusable
    (`http.RoundTripper`, `DialFunc`, `http.Handler`) and lets each client
    adapter wire the executor into whatever hook it has (redis.Hook,
    gorm plugin, ...). §4 has the reasoning.
  - No metrics, no tracing, no logging. Adapters and drivers decide how to
    surface state.
  - No third-party dependencies. The recommended sentinel driver lives in
    its own module so the framework itself stays stdlib-only.

## 2. Key Abstractions / Seams

- **Two-tier abstraction: `Policy` + `Driver` + `Executor`.** `Policy` is a
  backend-neutral, declarative wish list; `Driver.NewExecutor(Policy)`
  builds a concrete backend runtime. The bundled builtin driver reads the
  policy directly; a sentinel driver translates it into sentinel-golang's
  flow/circuit-breaker rules. Adapters depend only on `Executor`.
- **Driver registry** (`RegisterDriver` / `MustGetDriver`) panics on
  empty/nil/duplicate registration — the same idiom used across the stdlib
  (see discovery, cache, loadbalance). Applications select the driver by
  name in configuration.
- **Neutral rejection errors** (`ErrRateLimited`, `ErrCircuitOpen`,
  `ErrBulkheadFull`) let adapters make protocol-specific decisions (429
  vs 503 in `NewHandler`) without importing a driver package.
- **Three adapter seams cover the practical space:**
  - `NewRoundTripper` — widest coverage; any `*http.Client` gains
    protection by swapping its `Transport`. Retries clone with the rewound
    body (`Request.GetBody`); 5xx responses count as failures for the
    breaker.
  - `NewDialer` — coarser but universal at the connection layer; pairs
    naturally with `discovery.LiveDialer.DialContext`. Resource is fixed
    because a dialer is already scoped to one service.
  - `NewHandler` — inbound admission; serves each request exactly once
    (inbound is not idempotent), maps neutral rejections to 429/503,
    treats 5xx as failure.
- **`Fallback` is a helper, not an interface method.** Adding a `degrade`
  parameter to `Executor.Execute` would ripple through every driver and
  adapter; a standalone helper composes with any executor (including nil)
  and keeps the core surface small.
- **`RateLimiter` is a separate seam.** It answers the standalone flow-
  control question (per-tenant quota, background job pacing, inbound
  admission) without dragging in breaking/retry/timeout. A Redis-backed
  driver enforces a single global budget across replicas; the builtin
  driver limits per replica.

## 3. Constraints

- Nil executor / nil transport / empty policy is a transparent pass-through
  across every adapter and helper. Wiring stays free until a policy is
  configured.
- Adapters must expose an `io.Closer` on their transport so a starter's
  destroy hook can release the executor. `roundTripper.Close` implements
  that already.
- `runOnce`'s per-attempt timeout must derive from the caller's context,
  never the background context — cancellation must propagate.
- `NewHandler` must guard against retry re-invocation of an already-served
  request; the response is committed after the first `Write`.
- Under the builtin driver, the bulkhead is held across retries (one slot
  per Execute, not per attempt) — a slow downstream must not be amplified.
- `redis.Nil` / `gorm.ErrRecordNotFound`-style "no data" errors from a
  client adapter must not feed the breaker; the adapter maps them to
  success before returning through `Execute`.

## 4. Trade-offs / Alternatives Rejected

- **No single universal per-request seam.** HTTP clients have
  `RoundTripper`, but redis-go has `redis.Hook`, GORM has plugin callbacks,
  and MQ producers vary by library. Trying to unify these under one
  `Interceptor` interface would break down at the first library whose hook
  is call-site-only (NATS, pulsar). The chosen answer is a small, shared
  `Executor` core plus a family of hand-written adapters — the analogous
  layering to `discovery` + `LiveDialer`.
- **Executor over decorator per stage.** A single `Execute` bundles rate
  limit + breaker + bulkhead + retry + timeout in one call so per-resource
  state (token bucket / breaker / semaphore) is coherent. Decorating each
  stage independently would double-count retries against the limiter and
  make retry-inside-breaker semantics ambiguous.
- **Duplicate breaker with `loadbalance.Tracker`.** Different job (LB
  eviction is a queryable candidate-set filter, resilience rejects a call
  in flight). Feeding both from the same `DoneInfo.Err` keeps them
  consistent without a code dependency.
- **No sliding-window in the Redis limiter (elsewhere).** Only token bucket
  is cleanly expressible as an atomic Lua script; sliding-window would
  either race or need heavy per-key data. The built-in in-process driver
  offers both; the Redis distributed driver in `starter-go-redis` is
  intentionally token-bucket only.
- **No retry inside `NewHandler`.** Retrying an already-written response is
  impossible; retry is only meaningful on the client seams.

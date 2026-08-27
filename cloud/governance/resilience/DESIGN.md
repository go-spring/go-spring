# resilience Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`resilience` is the client-side fault-tolerance abstraction of the governance
family (`go-spring.org/cloud/governance/resilience`). It defines the neutral
contract every adapter and driver satisfies, and ships an in-tree driver so
the framework works out of the box; production installs typically swap in the
sentinel driver from `starter/experimental/starter-resilience`.

## 1. Responsibilities & Boundaries

- **Does:** define `Policy`, `Executor`, `Driver`, `RateLimiter`,
  `LimiterDriver`; provide the built-in `default` driver; expose the client
  adapters (`NewRoundTripper`, `NewDialer`); offer a
  `Fallback` composition helper; ship a limiter registry with in-process
  built-ins (token bucket + sliding window).
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
  vs 503 in a protocol starter's admission middleware) without importing
  a driver package.
- **Two client adapter seams cover the practical space:**
  - `NewRoundTripper` — widest coverage; any `*http.Client` gains
    protection by swapping its `Transport`. Retries clone with the rewound
    body (`Request.GetBody`); 5xx responses count as failures for the
    breaker.
  - `NewDialer` — coarser but universal at the connection layer; pairs
    naturally with a dial closure over `discovery.Resolver.Pick`. Resource is fixed
    because a dialer is already scoped to one service.

  Inbound admission is NOT in this package: each protocol starter builds
  its own middleware on the `governance.ExecutorFor` seam (see
  starter-gin / starter-grpc admission), mapping neutral rejections to
  429/503 and serving each request exactly once.
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
- Inbound admission middlewares (built by the protocol starters) must guard
  against retry re-invocation of an already-served request; the response is
  committed after the first `Write`.
- Under the builtin driver, the bulkhead is held across retries (one slot
  per Execute, not per attempt) — a slow downstream must not be amplified.
  The sentinel driver upholds the same invariant by registering its
  isolation rule under a `$bulkhead`-suffixed resource and acquiring that
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
  error wins, then `Policy.RetryPredicate` (nil ⇒ retry on every error,
  the historical behavior). A `MaxDuration` caps the whole Execute across
  retries; the per-attempt `Timeout` is applied inside that budget.
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
  `Executor` core plus a family of hand-written adapters — the analogous
  layering to `discovery` + `Resolver`.
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
- **No retry inside inbound admission.** Retrying an already-written response is
  impossible; retry is only meaningful on the client seams.

# resilience

[English](README.md) | [中文](README_CN.md)

`resilience` is a framework-agnostic abstraction for **client-side** fault
tolerance: rate limiting, circuit breaking, retry, timeout, and fallback. Client starters plug the single
`chain.Executor` seam into their own request hook (HTTP RoundTripper, Redis
Hook, GORM plugin, ...).

It is **outbound-only**. Inbound requests run through
[`cloud/governance/guard`](../guard/), whose per-instance `Guard` has its own policy
vocabulary, two-phase shape (`Admit` + `Done`) and bundled engine. The old
inbound half of this package (`ServerManager`, `ServerEngine`,
`Driver.NewServerExecutor`) has been removed; the `Driver` seam now answers for
outbound calls alone.

## Features

- `ClientPolicy` fields (OUTBOUND): `RateLimit` / `Burst` / `Algorithm` / `Window` /
  `RateLimitMaxWait` / `RateLimitWarmUpDuration` / `WarmUpColdFactor`,
  `ErrorThreshold` / `OpenDuration` / `BreakerStrategy`
  (`consecutive` | `error-rate` | `slow-call` | `error-count`) /
  `ErrorRateThreshold` / slow-call pair / `HalfOpenRequests`, `Retry` (a
  `chain.RetryPolicy` node), `Timeout`.
- Neutral rejection errors: `ErrRateLimited`, `ErrCircuitOpen`,
  `ErrBulkheadFull`.
- Bundled `"default"` driver — in-process, zero dependencies. Recommended
  production driver `sentinel` lives in `starter/starter-governance-sentinel`.
- One client seam for opt-in adaptation: `NewRoundTripper`, an HTTP client
  `http.RoundTripper`. (A connection-level `DialFunc` seam was tried and
  retired to the one starter that needed it — see §2; request-level protection
  is where the four policies mean what their names say.)
- `Fallback(ctx, exec, fn, degrade)` — graceful degradation
  helper that composes with any executor.
- Rate limiting is a stage of a protected call, not an API of its own: the
  counters are the executor's own budget for its service, or, with a
  `ClusterLimiter` store handed in (Redis, in `starter-go-redis`), one budget
  across replicas.

## Alignment: sentinel & OpenSergo

The policy model is the neutral superset both engines translate from, and it
maps onto [sentinel-golang](https://github.com/alibaba/sentinel-golang) rules
and the [OpenSergo](https://opensergo.io) fault-tolerance CRDs:

| Policy knob | sentinel rule (v1.0.4) | OpenSergo strategy |
| --- | --- | --- |
| `RateLimit` + `Window` | flow `Threshold` (per-interval, normalized) + `StatIntervalInMs` | RateLimitStrategy |
| `RateLimit` + `RateLimitMaxWait` | flow `Throttling` + `MaxQueueingTimeMs` | ThrottlingStrategy |
| `RateLimitWarmUpDuration` / `WarmUpColdFactor` | flow `WarmUp` + `WarmUpPeriodSec` / `WarmUpColdFactor` | (warm-up) |
| `breaker-strategy=error-rate` | circuitbreaker `ErrorRatio` | CircuitBreakerStrategy ErrorRatio |
| `breaker-strategy=slow-call` (+ slow pair) | circuitbreaker `SlowRequestRatio` + `MaxAllowedRtMs` | CircuitBreakerStrategy SlowRequestRatio |
| `breaker-strategy=error-count` | circuitbreaker `ErrorCount` (exact) | CircuitBreakerStrategy ErrorCount |
| `breaker-strategy=consecutive` (default) | `ErrorCount` — **approximation**: sentinel has no consecutive-run breaker; a run resets on any success, sentinel counts over the window. Prefer `error-count` for an exact mapping | — |
| `HalfOpenRequests` | `ProbeNum` | — |
| `Retry` (`MaxRetries` / backoff) | outside sentinel (executor-owned retry loop) | RetryStrategy |

Per-driver legitimate ignores (documented, not faked): `Burst` and `Algorithm`
have no sentinel flow counterpart; the sliding-window limiter ignores
`RateLimitMaxWait` (only the token bucket queues); the default driver's warm-up
is a cold-start approximation (capacity ramp), not sentinel's rate ramp;
sentinel's system / hotspot / cluster rules are not exposed: they are inbound-
or parameter-dimension and have no policy field. OpenSergo's `RetryOnSame`
(endpoint-level retry hints) belongs to loadbalance, not here.

## Pluggable backends

This package holds **no registry**. An engine backend is a bean named after
itself and exported as `Driver`; the consumer collects them into a name-keyed
directory the center resolves the configured name against. `Driver` has ONE
method — `NewClientExecutor(service, Policy)` — because this package is
outbound-only. `spring.resilience.driver=sentinel` therefore selects the
outbound backend; inbound always uses guard's bundled engine. The bundled
driver answers to `"default"` without a bean, via `NewDefaultDriver(nil)`.

```go
// contribute a backend (typically from a starter's init)
gs.Provide(func() *sentinelDriver { return &sentinelDriver{} }).
    Name("sentinel").
    Export(gs.As[resilience.Driver]())
```

The `Export` is load-bearing: gs indexes beans by their exact type, so a
concrete driver without it is invisible to the by-name lookup.

The counter store is a backend too, but an OPTIONAL one — how wide a rate limit
reaches, nothing more. With no `ClusterLimiter` bean in the container, each executor
counts in a budget of its own, exactly like its breaker state: an
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
    Timeout: 2 * time.Second,
    Retry:          chain.RetryPolicy{MaxRetries: 2},
})

client := &http.Client{
    Transport: resilience.NewRoundTripper(http.DefaultTransport, exec, service, nil),
}
```

A bare budget check is the same call with only the rate-limit stage
configured — the empty body is the whole work being admitted:

```go
exec := mgr.ClientExecutorFor("gateway", "route:orders")
err := exec.Execute(ctx, chain.Call{}, func(context.Context) error { return nil })
if errors.Is(err, resilience.ErrRateLimited) { /* 429 */ }
```

---

# Design

`resilience` is go-spring's client-side fault-tolerance abstraction
(`go-spring.org/cloud/governance/resilience`), configured by the [governance](../README.md)
control plane. It defines the neutral
contract every adapter and driver satisfies, and ships an in-tree driver so
the framework works out of the box; production installs typically swap in the
sentinel driver from `starter/starter-governance-sentinel`.

## 1. Responsibilities & Boundaries

- **Does:** define `ClientPolicy`, `ClientExecutor`, `Driver`, `ClusterLimiter`, `Algorithm`;
  provide the built-in `default` driver; expose the client adapter
  (`NewRoundTripper`); offer a `Fallback` composition helper.
- **Refuses:**
  - No universal per-request seam. Every client library exposes a
    different call-time hook; the package supplies the most reusable one
    (`http.RoundTripper`) and lets each client
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
- **The driver is a named bean, injected at startup.** This package holds no
  registry: a backend is contributed as a bean named after itself and exported
  as `Driver`, and the manager injects the one whose name
  `spring.resilience.driver` selects (defaulting to the bundled `default`). The
  bundled builtin answers to
  `"default"` without a bean, so nothing has to contribute it.
- **Neutral rejection errors** (`ErrRateLimited`, `ErrCircuitOpen`,
  `ErrBulkheadFull`) let adapters make protocol-specific decisions (429
  vs 503 in a protocol starter's admission middleware) without importing
  a driver package.
- **One client adapter seam covers the practical space:**
  - `NewRoundTripper` — widest coverage; any `*http.Client` gains
    protection by swapping its `Transport`. Retries clone with the rewound
    body (`Request.GetBody`); 5xx responses count as failures for the
    breaker.

  A second, connection-level seam (`NewDialer`) once lived here too, for
  clients with no per-request hook. It was retired to the one starter that
  needed it (`starter-mongodb`, whose driver offers only `SetDialer`): the four
  policies — timeout, retry, breaker, fallback — describe an *operation*, so
  keying them on a dial makes the breaker miss the failures an operator cares
  about (slow or erroring requests over a healthy connection), retry re-attempt
  a handshake instead of the work, and leave fallback with nothing to degrade.
  It is a degradation for a client with nowhere else to put protection, not a
  recommended seam — so it belongs at that client, and will return here only if
  a second client proves the need general.

  Inbound **middleware** is not in this package: each protocol starter
  builds its own on the `resilience.ServerManager.ServerGuardFor` seam (see
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
  own budget, or a `ClusterLimiter` store — not a second API: by default one
  executor holds one budget for its service, a shared backend (Redis)
  counts per fleet.
- **ClusterLimiter are scoped by the service an executor is bound to.** One
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
- Outbound concurrency isolation (a client-side bulkhead) is NOT part of the
  bundled engine today; see `KNOWN_ISSUES.md` §2 (F-6) for the design and why
  it is deferred. A backend that brings its own isolation rules may still
  return `chain.ErrBulkheadFull`; adapters already map it.
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
- Retry is the one stage that is OPT-IN and declared by the caller, not a
  framework default: it lives in a `chain.RetryPolicy` node carried on
  `chain.Operation` (`op.Retry`), and its zero value means "no retry". A
  governance center MAY also push a node on the policy
  (`ClientPolicy.Retry`) for the whole service; when it declares a schedule
  it wins WHOLE over the call's node (a pushed value is how an operator
  hot-fixes a live incident), while the `NonIdempotent` flag is preserved
  from BOTH (see `EffectiveRetry`, the single merge point). Retry is paced
  by the node's backoff fields (`InitialInterval`, `Multiplier`,
  `MaxInterval`, `RandomizationFactor`) — a zero `InitialInterval` keeps the
  back-to-back retry; `RetryBudget` is process-level and read only from the
  policy node. Whether an error is retried is decided by
  `resilience.ShouldRetry`: a `Retryable`-implementing error wins; otherwise
  every error retries. A `Timeout` caps the whole Execute — every round and the
  backoff between them — as one deadline applied at the entry, so the rounds
  share it rather than each getting their own.
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
  limit + breaker + retry + timeout in one call so per-service state (token
  bucket / breaker) is coherent. Decorating each
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

## 5. Engine structure & governance wiring

- **One engine, one contract.** This package has a single outbound engine,
  `defaultExecutor` (with retry), behind `chain.Executor` (defined in the leaf
  package `cloud/chain`). Inbound is not expressed here at all; it lives in
  `cloud/governance/guard`.
- **Organs, neutral parameters.** The breaker state machine lives in
  `cloud/governance/breaker`; the rate-limit vocabulary, local budget, `ClusterLimiter`
  seam and counting algorithms live in `cloud/governance/ratelimit`. Both are leaf
  packages shared with inbound `guard`, so both directions map their own policy
  into the same organs and a fix lands once. `ClientPolicy.breakerSpec()` and
  `RateSpec()` are the outbound mappings.
- **Strictly one-way dependencies:** `chain ← observability ← resilience`. The
  rejection sentinels live in `chain`; `BreakerState` /
  `BreakerEventListener(Setter)` are defined in `observability/breaker.go`, and
  resilience refers to them through aliases for compatibility.
- **Naming:** the two halves are a `side` (`clientSide` / `serverSide`, fields
  `m.client` / `m.server`); the word `lane` is rejected.
- **The manager is un-generic:** two concrete types `clientSide` / `serverSide`,
  each holding one published snapshot of its direction's settings
  (`ClientSettings` / `ServerSettings`) plus the per-label generation memoized
  from it; the registry holds `*entry{exec}` pointers and is an identity table
  only — `Apply` never walks it. What decides whether a label rebuilds is the
  CONTENT-derived generation each executor compares for itself on the call path.
- **The executor binds one service at construction; `call` does not change that.**
  `Execute(ctx, call, fn)` takes the call's identity (`chain.Call`: target,
  scenario, idempotency) explicitly, so every link reads it from the signature
  rather than from the context — but protection STATE stays bound at
  construction: one breaker and one budget per executor.
  `call.Target` / `call.Scenario` are carried for readers (and a future step that
  keys state on them); they partition nothing yet. Never let `Execute` *select*
  protection state by an arbitrary per-call key — to partition counters by key,
  hand in a `ClusterLimiter` and put the key in the scope.
- **The breaker must be built lazily on the first call** (`snapshot()` /
  `buildState()`), never at executor construction — otherwise
  `SetBreakerEventListener` cannot attach (the `WrapClientExecutor` in
  `ClientManager.ClientExecutorFor` is handed over by hand).
- **One label, one executor on the managed path.** Clients always go through
  `ClientManager.ClientExecutorFor`; none builds its own via `Subscribe` +
  `NewExecutor`. Don't give a single client a "needs to see the resolved policy"
  escape (that forces it around the manager) — change things at the govern-rule
  layer instead. httpx was the only client that bypassed the manager with a
  `min-requests=5` floor; that floor is deleted and it now uses
  `ClientExecutorFor("http", service)`. `Policy.MinRequests`'s `<=0 → 1` default
  is unchanged.
- **The executor is a stable shell; its state is a replaceable generation.**
  There is no refresh protocol any more. `ClientManager.Apply` publishes a snapshot
  and stops; each executor holds a `PolicySource` — the manager — and asks it on
  the call path what this operation runs under and which generation that is
  (`PolicySource.Resolve`). A generation it has not materialized yet makes it
  rebuild everything that generation owns — breaker, rate budget, retry
  budget — in one go. Nothing is retuned in place, so there is no "which organ
  keeps what" rule to get wrong: **a config change is the state of the label
  starting over.**
  `loadbalance.Chain` is the same shape (`SelectionSource` over a swapped
  `*Pool`), and it is the template this was aligned to.
  Two consequences worth stating. **Evict + rebuild of the EXECUTOR is still not
  an option outbound**: a client caches the executor its constructor got
  (`go-redis`, `gorm`, `memcached`, `redigo`, `httpx` all hold it for life). What
  moves is the state behind the executor, never the object. And **a call already
  in flight when a generation moves settles against the generation it loaded**,
  which costs one lost sample and nothing else.
  The generation is derived from the CONTENT of the rule set a label resolves
  against, never from a push counter, so a document re-pushed unchanged does not
  reset a live breaker.
  Backends whose state the framework cannot reach keep their own boundary:
  sentinel's rules live in the SDK, keyed by resource name, so for it a
  generation change is a RULE SWAP that preserves the SDK's statistics.
- **Rate limiting is one stage**, offered as 1+3: layer 1 is the executor's own
  per-service budget (`rateState`, the default path); layer 3 is the `ClusterLimiter`
  interface, which serves cross-replica shared stores only (scope = the service
  the executor is bound to). Layer 2 (in-process sharing) is rejected — don't
  add a memory `ClusterLimiter` implementation back (`NewMemoryCounters` is deleted);
  counting by key is the caller's (gateway's) own job; document `ClusterLimiter` as
  cross-replica only.
- **The breaker records once per logical call; rate-limit counts per attempt.**
- **`ClientPolicy` / `ServerPolicy` are single defs** carrying all their own
  value tags; governance's `ClientRule` / `ClientDefaultPolicy` / `ServerRule`
  embed them directly, with no binding twin and no translation layer. New
  governable knobs go straight on the Policy (with a tag); function fields do
  not belong in a Policy.
- **`Timeout` was renamed `Timeout`** (matching the key); the
  `RetryPredicate` function field is deleted (never assigned anywhere) — retry
  classification goes only through the `Retryable` interface plus
  default-retry-all. Retry lives in the `Retry` node (`chain.RetryPolicy`)
  rather than as flat policy fields. Policy fields: backoff
  `InitialInterval` / `Multiplier` / `MaxInterval` / `RandomizationFactor`
  (inside `Retry`), whole-call `Timeout`, breaker strategy
  `BreakerStrategy` / `ErrorRateThreshold` / `MinRequests` / `BreakerWindow`;
  all zero = the old behavior. A `Policy` cannot be compared with `== Policy{}`
  — use `IsZero()`. A second split type is allowed only for a hard structural
  reason (today only the loadbalance factory's argument translation, consuming
  `Selection.Params`).
- **The driver is a named bean, injected at startup.** The registry is gone
  (`RegisterDriver` / `GetDriver` / `RegisterLimiter` / `GetLimiter` and both
  init-registered `"default"`s are deleted); resilience is a zero-global,
  zero-gs-dependency contract package. A backend is a named bean +
  `Export(gs.As[resilience.Driver]())`; the manager injects the one whose name
  `spring.resilience.driver` selects — `resilience.NewClientManager(driver)`, wired via
  `gs.TagArg("${spring.resilience.driver:=default}")`, with a nil driver falling
  back to the builtin `resilience.NewDefaultDriver(limiter ClusterLimiter)`. The key is
  a plain `app.properties` property read at wiring, **not** a governance-document
  key: the backend is fixed for the process's lifetime, because swapping it drops
  every executor's breaker/limiter state. An unresolvable name leaves the required
  `Driver` parameter unsatisfied, so startup fails — no silent noop fallback. A
  `gs.Provide` whose ctor returns a concrete type must `Export` (gs indexes by
  exact type). `httpx.Config.ResilienceDriver` is the `resilience.Driver` object
  itself (httpx is container-free).
- **Backend-only capabilities.** Cluster-wide rate limiting, warm-up,
  per-caller limiting and system-adaptive protection (Sentinel's SystemRule
  family) are Driver-backend features, not core `Policy` fields; a sentinel
  installation exposes them along its native config surface.
- **Observability bridge.** Instrumentation wrapping for the outbound/inbound
  executors lives in **`cloud/observability`**
  (`WrapClientExecutor(inner, system, service)` / `WrapServerExecutor(inner,
  system, service)`, returning `chain.Executor`); outbound fire lives in
  `cloud/governance/fault.ClientManager.WrapClientExecutor(inner, in)`; the rejection sentinels
  live in `cloud/chain`. The standalone `observe-resilience` module is deleted.
  Inbound wrappers over gRPC must use `grpc.ChainUnaryInterceptor` /
  `ChainStreamInterceptor` — `grpc.UnaryInterceptor` is a setter, not an
  append, so a later one replaces an earlier one.

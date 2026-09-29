# Design: starter-ratelimit-redis

## 1. Problem

In-process counting keeps a scope's budget inside the process, so N replicas each allow N
times the configured budget. A shared quota needs the state outside any single process, i.e. Redis.

## 2. Why a counter store, not a driver switch

resilience has two seams, and they answer different questions:

- `Driver` answers *which engine* runs a policy — it builds an **Executor** with limit + breaker +
  retry + timeout as one bundle. Reaching for Redis through this seam would drag breaker and retry
  off `default`/`sentinel` too, which is not what anyone wants: distributed limiting is orthogonal
  to in-process circuit breaking.
- `Counters` answers *where the counters live* — the rate-limit stage's own seam (`Allow(ctx, scope,
  policy, n)`). It changes the state a limit is made of without touching which engine enforces it.

The store is optional, and the container picks it: with no `resilience.Counters` bean the driver
leaves each executor counting in a budget it holds itself, and a backend starter contributing one —
this module — makes every executor the driver builds spend that instead, the bundled "default" one
as much as any other backend, since the wiring injects the store into the driver bean. Result:
"sentinel executor + redis counters" composes freely, and no route-level or client-level switch
exists to forget.

## 3. Reuse, not reimplementation

The Lua token bucket already exists in `starter-go-redis/experimental` (atomic refill+consume EVAL,
hash state, `ratelimit:` key namespace, burst/rate TTL, unlimited pass-through). Duplicating it here
would fork the script. This module is therefore a thin Contributor starter: config in, one
`resilience.Counters` bean out. The trade-off is a dependency on starter-go-redis's experimental
subpackage; if that package ever moves, this starter follows it.

## 4. Wiring

One block, `${spring.ratelimit.redis}`, armed by `gs.Module(gs.OnProperty("spring.ratelimit.redis"),
setup)` so an imported-but-unconfigured starter contributes nothing:

- `client` (required, fail-fast): the starter-go-redis bean name; injected via `gs.TagArg(client)` —
  the same by-name seam the other redis-consuming starters use.

The bean is contributed by

```go
r.Provide(func(client *goredis.Client) (resilience.Counters, error) {
    return experimental.NewCounters(client.UniversalClient)
}, gs.TagArg(c.Client)).Caller(1)
```

Returning the interface type is what indexes the bean under `resilience.Counters`: gs takes the
constructor's first return type as the bean type, so no `Export(gs.As[...])` is needed — and that is
the exact type the driver's injected parameter matches. The module (rather than a plain bean ctor)
is required because the block's presence arms it: `gs.Module(gs.OnProperty("spring.ratelimit.redis"),
setup)` gates the contribution, and `setup` binds the block before providing the bean.

## 5. Semantics inherited from the Lua bucket

- One EVAL per Allow: read hash → refill by elapsed ms → consume if enough → write hash → EXPIRE
  ceil(burst/rate)+1s. No client-side locking, so N replicas racing on one key still grant exactly
  the burst between them.
- `n` > 1 is all-or-nothing inside the script.
- Keys are `ratelimit:<scope>`; independent scopes are independent budgets.
- Zero `RateLimit` short-circuits to allow with no Redis round-trip.
- Clock: the caller sends `now` in milliseconds; refill correctness depends on rough clock agreement
  between replicas (skew shifts fairness, never the safety of the atomic consume).

## 6. Documented limits

- A sliding-window scope is counted as a token bucket. The atomic script is what makes a shared
  budget correct, and a window does not map onto one script cheaply, so the store refuses to pretend
  otherwise rather than silently answering with a different burst profile.
- Queueing is not offered: `Policy.RateLimitMaxWait` is ignored and an over-limit unit is rejected
  immediately. Waiting for a token would mean polling Redis; queueing stays with the in-process
  store.

## 7. Failure posture

The store returns its own error when Redis is unreachable, and the executor's rate-limit stage logs
it and lets the call through — a broken counter backend must not become an outage. Rejections stay
the executor's decision (`ErrRateLimited` → 429 at the caller). Keeping the open/closed choice out of
the store matches the "container assembles, not adjudicates" stance: the store is a mechanism, the
response belongs to the caller.

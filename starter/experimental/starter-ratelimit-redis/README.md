# starter-ratelimit-redis

Redis-backed counters for Go-Spring's rate-limit stage: it contributes a
[`resilience.Counters`](../../../cloud/governance/resilience/ratelimit.go) store whose token
buckets live in Redis, so every replica of a service spends one budget per scope instead of each
replica counting its own.

## What it does

- Contributes exactly one bean of type `resilience.Counters`, over the `*goredis.Client` bean named
  by [`spring.ratelimit.redis.client`](#configuration).
- Nothing to step aside for: with no `resilience.Counters` bean in the container each executor
  counts in a budget it holds itself (one per service label all the same — the manager
  builds one executor per label). Contributing this one is the whole switch: the resilience driver
  injects it, so every executor it builds spends the shared budget — a limit is made of its
  counters, not of its executor. What widens is only how far a budget reaches, never what it covers.
- The counting itself (atomic refill + consume in one Lua `EVAL`, bucket state in a hash, key TTL
  against cold-key pileup) is the implementation in `starter-go-redis/experimental`; this starter
  only decides where the counters live.
- The executor/breaker/retry driver is untouched: rejecting or queueing over-limit calls stays the
  executor's job. Cross-replica limiting is therefore "run this starter", with nothing per route and
  nothing per client to switch.

## Configuration

```properties
# The redis client (starter-go-redis)
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# The one key: which *goredis.Client bean backs the shared counters. Required —
# an empty value fails wiring.
spring.ratelimit.redis.client=cache
```

The block's presence is what turns the starter on; an imported but unconfigured starter contributes
nothing and each executor keeps counting in a private budget of its own. The rate-limit knobs
(`rate-limit`, `burst`, `algorithm`, `window`, `rate-limit-max-wait`) are
[`resilience.ClientPolicy`](../../../cloud/governance/resilience/policy.go)
fields on the governance rule document, not starter configuration.

## Consuming it

Nothing is wired per call site: whatever already limits — a client starter's executor, the gateway's
`rateLimit` filter — spends this store once the starter is configured.

```properties
# The filter takes the budget, not a driver: with this starter running, the
# budget is shared by every gateway replica.
spring.gateway.route.api.filters=rateLimit(rate=100)
```

Application code that builds its own policy goes through the same authority:

```go
exec := mgr.ClientExecutorFor("ratelimit-redis", "ratelimit-redis:api")
err := exec.Execute(ctx, func(context.Context) error { return nil })
// errors.Is(err, resilience.ErrRateLimited) → 429
```

## Limits

- A sliding-window scope is counted as a token bucket: the atomic script is what makes a shared
  budget correct, and a window does not map onto one script cheaply. Use a token bucket when the
  counters are shared.
- Queueing is not offered: `resilience.ClientPolicy.RateLimitMaxWait` is ignored, and an over-limit call is
  rejected immediately with `ErrRateLimited`. Waiting for a token would mean polling Redis; keep
  queueing for the in-memory store.
- A Redis outage does not stop calls: the executor logs a failing counter store and lets the call
  through.
- Clock: refill math runs inside Redis with the caller-supplied `now` (millisecond precision), so
  replica clock skew directly affects fairness.

## Testing

`go test ./...` covers the shared-budget semantics (two stores over one Redis), scope isolation,
all-or-nothing batches, the unlimited pass-through, concurrent grants and key TTL using miniredis,
plus the wiring contract: a configured starter contributes the `resilience.Counters` bean while an
unconfigured one contributes none, and an empty client name fails startup. The [example](example)
runs a docker-gated smoke test (`example/check.sh`) with
two "replicas" sharing one budget against a real Redis.

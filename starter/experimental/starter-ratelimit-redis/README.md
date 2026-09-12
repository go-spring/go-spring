# starter-ratelimit-redis

Redis-backed distributed rate limiting for Go-Spring: it contributes
[`resilience.LimiterDriver`](../../../cloud/governance/resilience/ratelimit.go) instances
whose token buckets live in Redis, so every replica of a service shares one
global budget instead of limiting independently per process.

## What it does

- One limiter driver per config entry `spring.ratelimit.redis.instances.<name>`, each
  reusing a `*redis.Client` bean published by
  [starter-go-redis](../../starter-go-redis) under `spring.go-redis.instances.<client>`.
- The token-bucket algorithm (atomic refill + consume in one Lua `EVAL`, bucket
  state in a hash, continuous refill, key TTL against cold-key pileup) is the
  implementation in `starter-go-redis/experimental`; this starter only wires it
  into the resilience limiter registry by driver name.
- Registration is per-name, not a whole-driver swap: the resilience executor
  (breaker/retry/timeout) keeps its `default`/`sentinel` driver while limiting
  alone moves to Redis. See [DESIGN.md](DESIGN.md) for why.

## Configuration

```properties
# The redis client (starter-go-redis)
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# One limiter driver per entry; `client` is required (fail-fast when empty).
spring.ratelimit.redis.instances.gateway.client=cache
# Name consumers select this driver by; defaults to the instance name.
spring.ratelimit.redis.instances.gateway.driver=redis
```

## Consuming the driver

By registry lookup (what starter-gateway's `rateLimit` filter does internally):

```go
d, _ := resilience.GetLimiter("redis")
lim, _ := d.NewRateLimiter(resilience.LimitPolicy{Rate: 100, Burst: 50})
ok, err := lim.Allow(ctx, "tenant-a")
```

or by injecting the exported bean:

```go
gs.Provide(func(d resilience.LimiterDriver) *gs.HttpServeMux { ... },
    gs.TagArg("gateway"))
```

With starter-gateway it is pure config:

```properties
spring.gateway.route.api.filters=rateLimit(rate=100,driver=redis)
```

## Limits

- Token bucket only: `LimitPolicy.Algorithm`/`Window` are ignored — the Lua
  script models a token bucket and nothing else.
- A Redis outage degrades every `Allow` call to an error — callers decide to
  fail-open (like gateway) or fail-closed.
- Clock: refill math runs inside Redis with the caller-supplied `now`
  (millisecond precision), so replica clock skew directly affects fairness.

## Testing

`go test ./...` covers shared-budget semantics, key isolation, all-or-nothing
`AllowN`, continuous refill, concurrent grants and bucket TTL using miniredis.
The [example](example) runs a docker-gated smoke test (`example/check.sh`)
showing two "replicas" sharing one budget against a real Redis.

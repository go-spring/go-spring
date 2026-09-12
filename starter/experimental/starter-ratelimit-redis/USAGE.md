# starter-ratelimit-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`, `driver.go`), the token-bucket
implementation in `starter-go-redis/experimental/ratelimit.go`, the
[resilience limiter registry](../../../cloud/governance/resilience), and the self-asserting
[example/](example) (`example/check.sh`). Rate-limiting semantics (token bucket) are standard —
everything below is go-spring's increment.

**Activation**: any `spring.ratelimit.redis.instances.<name>.*` property registers one
`resilience.LimiterDriver` instance per `<name>`; each reuses the `*redis.Client` bean named by
its `client` field (provided by starter-go-redis under `spring.go-redis.instances.<client>`). Consumers
select the driver by name — starter-gateway's `rateLimit(driver=...)` filter,
`resilience.GetLimiter(name)`, or direct injection.

---

## 1. Complete worked project

Two HTTP "replicas" sharing one global token budget in Redis — the drill-ground for cross-replica
limiting. File tree:

```
demo/
├── go.mod
├── main.go
├── web.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/redis/go-redis/v9        latest
    go-spring.org/spring                v1.3.x
    go-spring.org/starter-go-redis      latest
    go-spring.org/starter-ratelimit-redis latest
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-ratelimit-redis"
)

func main() { gs.Run() }
```

**web.go** — the application's entire limiting surface:

```go
package main

import (
    "net/http"

    "go-spring.org/cloud/governance/resilience"
    "go-spring.org/spring/gs"
)

func init() {
    // Inject the Driver bean by instance name; LimitPolicy is per call site,
    // NOT starter config.
    gs.Provide(func(d resilience.LimiterDriver) *gs.HttpServeMux {
        mk := func() resilience.RateLimiter {
            lim, err := d.NewRateLimiter(resilience.LimitPolicy{
                Rate:  2,  // tokens per second
                Burst: 5,  // bucket capacity; <=0 defaults to max(1, Rate)
            })
            if err != nil {
                panic(err) // "no redis client bound" — wiring bug
            }
            return lim
        }
        limA, limB := mk(), mk() // two "replicas" — ONE shared budget in Redis

        mux := http.NewServeMux()
        serve := func(l resilience.RateLimiter) http.HandlerFunc {
            return func(w http.ResponseWriter, r *http.Request) {
                ok, err := l.Allow(r.Context(), "api") // key "api" under "ratelimit:"
                if err != nil {
                    http.Error(w, "limiter backend error: "+err.Error(), http.StatusInternalServerError)
                    return
                }
                if !ok {
                    http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
                    return
                }
                _, _ = w.Write([]byte("ok"))
            }
        }
        mux.Handle("/a/", serve(limA))
        mux.Handle("/b/", serve(limB))
        return &gs.HttpServeMux{Handler: mux}
    }, gs.TagArg("gateway"))
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- redis client (owned by starter-go-redis; the driver reuses it by name) --
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# --- limiter driver --------------------------------------------------------------
spring.ratelimit.redis.instances.gateway.client=cache
spring.ratelimit.redis.instances.gateway.driver=redis    # name registered in the limiter registry
                                               # (defaults to the instance name when unset)
```

Pure-config consumption with starter-gateway instead of code:

```properties
spring.gateway.route.demo.filters=rateLimit(rate=100,driver=redis)
```

Moving from per-replica to cross-replica limiting is only a driver-name change; breaker/retry/
timeout keep the default driver — the limiter registry is independent of the executor registry.

**Verify** (with a local Redis, e.g. `example/docker-compose.yml`):

```bash
go run . &
for i in $(seq 1 10); do curl -s -o/dev/null -w '%{http_code}\n' localhost:9090/a/; done
# first 5 (burst) → 200, rest → 429; /b/ draws from the SAME budget
```

The runnable [example/](example) asserts shared budget and refill; `example/check.sh` wraps it in
docker compose.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-go-redis + starter-ratelimit-redis
  └─ gs.Module(gs.OnProperty("spring.ratelimit.redis"))
        └─ conf.BindEach("${spring.ratelimit.redis}") per entry <name>:
             ├─ fail fast when client == ""        (boot error naming the instance)
             ├─ driver name = c.Driver, or the instance <name> when unset
             └─ Provide func(client) *Driver → bean "<name>"
                  (gs.TagArg(c.Client); Export resilience.LimiterDriver — the Export
                   keeps the bean root-reachable so the registry side effect ALWAYS runs)

gs.Run()
  ├─ config bind: ${spring.ratelimit.redis.instances.<name>} → Config (value tags)
  ├─ ctor: driverFor(driver, client)
  │    - process-global sync.Map "drivers" holds ONE Driver per driver name
  │    - first use: resilience.RegisterLimiter(name, d) — the registry PANICS on
  │      a duplicate name; subsequent wiring (e.g. gs.RunTest in the same test
  │      binary) rebinds the client instead of panicking
  ├─ bean wiring: consumers' autowire:"<name>" resolved
  └─ on SIGTERM: nothing to release — the registry entry is process-global and
                 the redis client's Close belongs to starter-go-redis.
```

The Driver bean itself is a thin adapter: `NewRateLimiter(p)` captures the bound client at call
time (RWMutex-guarded) and delegates to `experimental.NewRateLimiter` from
starter-go-redis — that is where the Lua token bucket actually lives.

### 2.2 One Allow call, layer by layer

`lim.Allow(ctx, "api")` with `LimitPolicy{Rate: 2, Burst: 5}`:

1. Zero `Rate` short-circuits to an unlimited pass-through with **no Redis round-trip**
   (misconfiguration danger: a lost policy = no limiting).
2. The atomic Lua script runs entirely inside Redis on key `ratelimit:api` (hash of
   `tokens` + `ts`): refill by `elapsed × rate` capped at `burst`, consume one token if
   available, `HSET` the new state, `EXPIRE` the key to `ceil(burst/rate)+1` seconds (idle keys
   self-delete — abandoned budgets never leak).
3. Returns `1`/allowed or `0`/limited. Because state lives in Redis, every replica drawing on
   the same key shares one budget — the whole point of this starter.
4. `Burst <= 0` defaults to `max(1, int(Rate))` at limiter construction.
5. A Redis outage makes `Allow` return an error — the consumer decides fail-open vs fail-closed
   (the example answers 500; gateway treats it per its own policy).

### 2.3 What the driver does NOT do

- `LimitPolicy.Algorithm` is **rejected loudly**: `NewRateLimiter` returns an error when the policy
  asks for anything other than token bucket (`""` or `token-bucket`) — a `SlidingWindow` policy used
  to be silently downgraded to a token bucket with a different burst profile. `Window` is still
  ignored (it is meaningless for a token bucket, per the policy contract).
- Replica clock skew affects refill fairness: `now` is caller-supplied into the script.
- No metrics of its own — limiter observability, if any, surfaces at the consumer
  (gateway/resilience executor).

---

## 3. Per-key behavior reference

All keys live under `spring.ratelimit.redis.instances.<name>` (exact-match, no relaxed forms).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `client` | string | — | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.instances.<client>`; checked before bean registration. `TagArg(c.Client)` is the seam tying driver to redis instance. | Empty → boot fails naming the instance; typo → bean-wiring failure at boot. |
| `driver` | string | instance name | Name registered in the resilience limiter registry — the string passed to `resilience.GetLimiter` and gateway's `rateLimit(driver=...)`. ⚠ Defaults to the instance name when unset: `spring.ratelimit.redis.instances.web.*` silently registers a driver named `web`, which may collide with an unrelated name. Duplicate names fail fast at boot: two instances of this starter claiming one name get a startup error naming both; a name already registered by another module (e.g. the built-in `default`) is a clear ctor error instead of the registry panic. | Same-name instances → boot error naming both; unintended default name → consumers resolve a limiter you did not plan for. |

⚠ Limit parameters (`rate`, `burst`, key) come per call site via `resilience.LimitPolicy`, not
from this config.

---

## 4. Verification & fault drills

### 4.1 Shared budget across "replicas"

```bash
for i in $(seq 1 10); do
  p=/a/; [ $((i%2)) -eq 1 ] || p=/b/
  curl -s -o/dev/null -w "%{http_code} " localhost:9090$p
done; echo   # exactly burst 200s interleaved across /a/ and /b/, rest 429
```

State in Redis:

```bash
redis-cli hgetall ratelimit:api          # tokens + ts
redis-cli ttl ratelimit:api              # ceil(burst/rate)+1
```

### 4.2 Burst + refill drill

With `Rate: 2, Burst: 5`: drain the budget (5×200), then wait ~2.2s and confirm continuous
refill (`curl` loop → at least `rate × seconds` new 200s). The example automates exactly this.

### 4.3 Fail-open/fail-closed drill

Stop Redis (`docker compose stop redis`): every `Allow` returns an error. Your handler decides —
the example answers 500 (fail-closed). Choose explicitly per endpoint sensitivity.

### 4.4 Registry lookup drill

From app code (what gateway's filter does internally):

```go
d, ok := resilience.GetLimiter("redis")   // the configured driver name
lim, _ := d.NewRateLimiter(resilience.LimitPolicy{Rate: 100, Burst: 50})
ok, err := lim.Allow(ctx, "tenant-a")
```

### 4.5 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up redis, run self-asserting example
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `ratelimit-redis: instance "<n>" missing required property ...client` | instance without `client` | Set it to an existing `spring.go-redis.instances.<name>`. |
| Boot fails: driver name `x` claimed by both instance `a` and `b` | two instances resolved to the same driver name (remember the instance-name default) | Give each instance an explicit, distinct `driver` (the error names both). |
| `NewRateLimiter` errors `no redis client bound` | driver bean constructed without the client injection path | Only construct via the starter (or bind before first use in tests). |
| No limiting at all, everything 200 | `LimitPolicy.Rate` zero → unlimited pass-through | Set an explicit positive Rate. |
| `NewRateLimiter` errors on `Algorithm: SlidingWindow` | unsupported algorithms are now rejected loudly (token bucket only) | Model the cap as Rate/Burst, or pick a driver that implements sliding windows. |
| 500s instead of 429s | Redis unreachable; `Allow` errors and the handler surfaces it as 500 | Decide fail-open vs fail-closed per endpoint; fix Redis. |
| Different replicas limit independently | instances point at different Redis clients/keys, or key strings differ | Same `client` + same Allow key across replicas. |
| Test binary panics on second wiring | registry registration is once-per-process | The starter rebinds the client for the same name; avoid registering two different names pointing at one driver string. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 2 |
| Required | 1 (`client`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger):

- RESOLVED (2026-08): duplicate driver names no longer panic or silently share — same-starter
  collisions fail at boot naming both instances; cross-module collisions (e.g. `default`) are a
  clear ctor error.
- The process-global `drivers` sync.Map + registry makes behavior depend on wiring order across
  tests in one binary (mitigated by rebind, but only for the client, not the registered set).
- RESOLVED (2026-08): `Algorithm` values other than token bucket are now rejected with an error
  from `NewRateLimiter` instead of being silently ignored.

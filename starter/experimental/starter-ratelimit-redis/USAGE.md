# starter-ratelimit-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is verified
against the starter source (`starter.go`, `config.go`), the counter implementation in
`starter-go-redis/experimental/ratelimit.go`, the [governance wiring](../../starter-governance-file/wiring.go)
that injects the store into the driver bean, and the self-asserting [example/](example)
(`example/check.sh`). Rate-limiting semantics themselves are documented in
`cloud/resilience` — everything below is this starter's increment.

**Activation**: any `spring.ratelimit.redis.*` key arms the starter's module, and the block's one key
must be set: `spring.ratelimit.redis.client` names the `*goredis.Client` bean (provided by
starter-go-redis under `spring.go-redis.instances.<client>`) whose Redis instance backs the
counters. The starter contributes ONE bean of type `resilience.Counters`. With none in the container
each executor counts in a budget of its own, so contributing this one is the whole switch:
[starter-governance-file](../../starter-governance-file)'s driver bean injects it, and every executor in the
process then spends one Redis budget per scope — across replicas, which an executor-local budget
cannot do.

---

## 1. Complete worked project

Two HTTP "replicas" sharing one global token budget in Redis — the drill-ground for cross-replica
limiting. File tree (this is the checked-in example, verbatim):

```
demo/
├── go.mod
├── main.go
└── conf/
    ├── app.properties
    └── governance.yaml
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring                   v1.3.x
    go-spring.org/starter-go-redis         latest
    go-spring.org/starter-ratelimit-redis  latest
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

**web.go** — the application's entire limiting surface. The budget is a
`resilience.ClientPolicy` read from the governance rules, and the call goes through the executor:

```go
package main

import (
    "context"
    "errors"
    "net/http"

    "go-spring.org/cloud/resilience"
    "go-spring.org/spring/gs"
)

// The service label the handlers share; the rule in conf/governance.yaml is keyed
// by it, and it is the scope the budget is kept per.
const service = "ratelimit-redis:api"

func init() {
    gs.Provide(func(mgr *resilience.Manager) *gs.HttpServeMux {
        mux := http.NewServeMux()
        // Two handlers model two replicas: no shared in-process state. Both
        // spend the one Redis-backed counter store this process contributed.
        mux.Handle("/a/", serve(mgr.ClientExecutorFor("ratelimit-redis", service)))
        mux.Handle("/b/", serve(mgr.ClientExecutorFor("ratelimit-redis", service)))
        return &gs.HttpServeMux{Handler: mux}
    })
}

func serve(exec resilience.ClientExecutor) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        err := exec.Execute(r.Context(), func(context.Context) error { return nil })
        switch {
        case err == nil:
            _, _ = w.Write([]byte("ok"))
        case errors.Is(err, resilience.ErrRateLimited):
            http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
        default:
            http.Error(w, "executor error: "+err.Error(), http.StatusInternalServerError)
        }
    }
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- redis client (owned by starter-go-redis; the counters count through it) --
spring.go-redis.instances.cache.addr=127.0.0.1:6379

# --- the starter's one key ---------------------------------------------------
# Which *goredis.Client bean backs the shared counters. Required.
spring.ratelimit.redis.client=cache

# --- governance rules (their own file, their own refresh channel) ------------
spring.governance.source.file.path=conf/governance.yaml
```

**conf/governance.yaml** — the budget itself, a policy, not starter config:

```yaml
spring:
  governance:
    enabled: true
    client:
      rules:
        - service: ratelimit-redis:api
          rate-limit: 2      # sustained 2/s
          burst: 5           # momentary allowance
```

A rule fully replaces `spring.governance.client.default` for the matched service (no field-wise merge), so the rule
carries every knob the service needs.

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
        └─ conf.Bind("${spring.ratelimit.redis}") → Config{Client}
             ├─ fail fast when Client == ""   (boot error naming the property)
             └─ Provide func(client *goredis.Client) (resilience.Counters, error)
                  (gs.TagArg(c.Client); the interface return type IS the bean type —
                   no Export is needed)

gs.Run()
  ├─ this store is now the container's only resilience.Counters bean
  ├─ bean wiring: the *goredis.Client bean named by client is injected by name;
  │     NewCounters wraps its UniversalClient (a nil client is a ctor error)
  ├─ the resilience driver bean injects resilience.Counters, so every executor
  │     it builds spends this store — and the store itself instantiates because
  │     that driver needs it
  └─ on SIGTERM: nothing to release — the store holds no state of its own and
                 the redis client's Close belongs to starter-go-redis
```

### 2.2 One Execute call, layer by layer

`exec.Execute(ctx, fn)` with the rule `rate-limit: 2, burst: 5`:

1. The executor's rate-limit stage calls `Counters.Allow(ctx, service, policy, 1)` — the executor
   charges every attempt, and the scope is the service the executor was built for.
2. Zero `RateLimit` short-circuits to an unlimited pass-through with **no Redis round-trip**. A lost
   rule therefore means no limiting at all: that is a governance-config symptom, not a store one.
3. The atomic Lua script runs entirely inside Redis on key `ratelimit:<scope>` (hash of `tokens` +
   `ts`): refill by `elapsed × rate` capped at `burst`, consume if enough, `HSET` the new state,
   `EXPIRE` the key to `ceil(burst/rate)+1` seconds (idle keys self-delete — abandoned budgets never
   leak).
4. The script answers 1 or 0. Because the state lives in Redis, every replica drawing on the same
   key shares one budget — the whole point of this starter.
5. A 0 comes back as `resilience.ErrRateLimited`; the caller answers 429 (the example/gateway do).
6. `Burst <= 0` defaults to `max(1, int(RateLimit))` inside the store.
7. A Redis outage makes `Allow` return an error: the executor logs it and lets the call through, so a
   broken counter backend degrades to no limiting rather than an outage.

### 2.3 What the store does NOT do

- It does not implement sliding windows: an `algorithm: sliding-window` policy is counted as a token
  bucket. The atomic script is what makes a shared budget correct, and a window does not map onto one
  script cheaply — use a token bucket when the counters are shared.
- It does not queue: `rate-limit-max-wait` is ignored and an over-limit unit is rejected immediately.
  Waiting for a token would mean polling Redis; keep queueing for the in-memory store.
- It keeps no metrics of its own — a rate-limited call shows up as `resilience.outcome=rate_limited` on the
  executor's observe layer.
- It does not adjudicate: rejecting over-limit calls and choosing open-vs-closed on a counter error
  are the executor's decisions.

---

## 3. Configuration reference

All keys live under `spring.ratelimit.redis` (exact-match, no relaxed forms). The block's presence is
what arms the starter; no other key belongs under it.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `client` | string | — | **Required.** Name of the `*goredis.Client` bean under `spring.go-redis.instances.<client>`; checked before bean registration and injected with `TagArg(c.Client)` at construction. | Empty → boot fails naming the property; a name with no such bean → bean-wiring failure at boot. |

⚠ The rate-limit knobs (`rate-limit`, `burst`, `algorithm`, `window`, `rate-limit-max-wait`) are NOT
starter config: they are `resilience.ClientPolicy` fields on the governance rule document, per service.

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
redis-cli hgetall ratelimit:ratelimit-redis:api   # tokens + ts
redis-cli ttl ratelimit:ratelimit-redis:api       # ceil(burst/rate)+1
```

### 4.2 Burst + refill drill

With `rate-limit: 2, burst: 5`: drain the budget (5×200), then wait ~2.2s and confirm continuous
refill (`curl` loop → at least `rate × seconds` new 200s). The example automates exactly this.

### 4.3 Redis outage drill

Stop Redis (`docker compose stop redis`): every `Allow` errors and the executor allows the call, so
the endpoints keep answering 200 — a broken counter backend means no limiting, not an outage. Fix
Redis to restore enforcement.

### 4.4 Through starter-gateway

Nothing per route: with this starter configured, a route's budget is already shared by every gateway
replica, and the filter takes the budget only.

```properties
spring.gateway.route.api.filters=rateLimit(rate=100)
```

### 4.5 Smoke test

```bash
cd example && ./check.sh    # docker-gated: compose up redis, run self-asserting example
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `ratelimit-redis: missing required property "spring.ratelimit.redis.client"` | the block is present but `client` is empty | Set it to an existing `spring.go-redis.instances.<name>`. |
| Boot fails while wiring the store (no bean named `<x>` of type `*goredis.Client`) | `client` names a redis instance that is not configured | Add `spring.go-redis.instances.<x>` (or fix the name). |
| Different replicas limit independently | no store is contributed (each executor kept its private budget), or replicas point at different Redis instances | Configure this starter on every replica and point them at one Redis. |
| No limiting at all, everything 200 | the service's policy has `rate-limit: 0`, or Redis is unreachable (the executor fails open) | Set an explicit positive `rate-limit` in the rules file; check Redis. |
| The burst is larger/smaller than the window arithmetic suggests | the rule asks for `algorithm: sliding-window`, which the shared store counts as a token bucket | Model the cap as `rate-limit`/`burst`, or keep sliding windows on the in-memory store. |
| Calls pile up instead of being rejected | `rate-limit-max-wait` is set but the shared store does not queue | Keep queueing for the in-memory store, or drop the knob. |
| A `spring.governance.enabled=false` app has no limiting | the center is switched off, so every executor is a pass-through | Enable governance; this starter only moves the counters. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 1 |
| Required | 1 (`client`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 2 (sliding window → token bucket, no queueing) |

Design suspects (for the audit ledger):

- RESOLVED (2026-09): the per-instance limiter registry is gone. At most one counter store exists in
  the container, and the container chooses it (none is contributed unless a backend starter does
  so), so the old failure modes — a driver name claimed twice, a route citing a driver name nobody
  registered, limiter state depending on wiring order across tests in one binary — no longer exist.
- The store's two documented limits (sliding window counted as a token bucket, no queueing) are
  deliberate: the atomic script is what makes a shared budget correct. They are re-stated wherever
  the store is described so a caller cannot be surprised by them.

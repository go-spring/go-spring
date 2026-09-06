# starter-go-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`health/health.go`, `bytecache/bytecache.go`) and the runnable [example/](example/) — file:line
spot-checks in brackets below. **Redis semantics and the go-redis API are [go-redis's own
documentation](https://redis.io/docs/latest/develop/clients/go/)** — everything below is
go-spring's increment.

**Activation**: any `spring.go-redis.*` key (the module is `OnProperty("spring.go-redis")`, a
prefix check). Each `spring.go-redis.<name>` entry creates one `*StarterGoRedis.Client` bean
named `<name>`, plus a health indicator named `redis:<name>`.

---

## 1. Complete worked project

Three topologies in one service, plus a cache façade and probes/metrics/tracing. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/redis/go-redis/v9   latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-go-redis latest
    go-spring.org/starter-cache    latest   // cache façade (spring.cache.*)
    go-spring.org/starter-actuator latest   // optional: readiness + /metrics
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
    go-spring.org/starter-governance latest // optional: resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cache"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper, use the full go-redis command surface, and demonstrate
the cache driver:

```go
package service

import (
    "context"

    "go-spring.org/cloud/cache"
    "go-spring.org/spring/gs"
    StarterGoRedis "go-spring.org/starter-go-redis"
)

type Service struct {
    // Always the wrapper type *StarterGoRedis.Client. It embeds
    // redis.UniversalClient, so Get/Set/Incr/Pipeline/PoolStats promote
    // unchanged whether the instance is single, sentinel, or cluster.
    Main     *StarterGoRedis.Client `autowire:"main"`     // single
    Sentinel *StarterGoRedis.Client `autowire:"sentinel"` // sentinel (still *redis.Client)
    Cluster  *StarterGoRedis.Client `autowire:"cluster"`  // cluster (*redis.ClusterClient)

    // The cache façade exposed by spring.cache.main.driver=go-redis:main.
    // NOTE the bean is named after the REDIS instance ("main"), not the
    // spring.cache map key — see §3 of starter-cache's USAGE.
    Cache *cache.Cache `autowire:"main"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            _ = s.Main.Set(ctx, "key", "value", 0).Err()
            var v string
            _ = s.Main.Get(ctx, "key").Scan(&v)
            _ = s.Cache.Set(ctx, "user:1", map[string]string{"name": "demo"}, 0)
        }
    })
}
```

**conf/app.properties** — the complete surface actually used above:

```properties
# --- single ---------------------------------------------------------------
spring.go-redis.main.addr=127.0.0.1:6379
spring.go-redis.main.pool-size=20
spring.go-redis.main.conn-max-lifetime=2m

# --- sentinel: master group resolved through the sentinel nodes -----------
spring.go-redis.sentinel.mode=sentinel
spring.go-redis.sentinel.master-name=mymaster
spring.go-redis.sentinel.sentinel-addrs=127.0.0.1:26379,127.0.0.1:26380

# --- cluster: seed nodes; client learns the full topology itself ----------
spring.go-redis.cluster.mode=cluster
spring.go-redis.cluster.addrs=127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002
spring.go-redis.cluster.route-by-latency=true

# --- cache façade: expose "main" as a typed cache.Cache bean --------------
spring.cache.main.driver=go-redis:main

# --- observability ---------------------------------------------------------
# Access log (tag _app_redis_access) emits by default; filter via logger config.
# redisotel spans/pool-metrics are ON by default and ride starter-otel's globals.

# --- actuator + otel ------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
```

**Verify** (start Redis first: `docker run -d -p 6379:6379 redis`, and a sentinel/cluster via
the [example's docker-compose.yml](example/docker-compose.yml) if you use all three):

```bash
go run .                          # boot fails fast if any topology is unreachable
curl -s :9370/readyz              # readiness folds in redis:main/redis:sentinel/redis:cluster
curl -s :9370/metrics | grep -E 'redis'   # redisotel pool metrics + hit ratios
grep _app_redis_access app.log | tail -3  # one access record per command
redis-cli GET key                 # "value" — written through the hook chain
redis-cli GET user:1              # JSON written via the cache façade
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-go-redis
  └─ gs.Module(OnProperty("spring.go-redis")) fires when any spring.go-redis.* key exists
        └─ conf.BindEach("${spring.go-redis}") → one Config per <name> entry
              ├─ mode single/sentinel → Provide(newClient).Name(<name>)
              │                          .Init((*Client).Init).Destroy((*Client).Destroy)
              ├─ mode cluster          → Provide(newClusterClient).Name(<name>) (same wrapper type)
              └─ Provide health.Indicator named "redis:<name>" (gated by health.enabled, default on)

gs.Run()
  ├─ ctor newClient [starter.go:97]: validateConfig → driver lookup → driver.CreateClient
  │   → instrument() (redisotel tracing+metrics, gated by otel.* keys)
  │   → failFastPing (unconditional, bounded by dial-timeout or 5s) [starter.go:218]
  ├─ Init [client.go:58]: resourceLabel → fault.WrapExecutor(resilience.ExecutorFor(resource))
  │   → resilience.WrapExecutor → applyObservability (access-log hook)
  │   → AddHook(resilienceHook) — command chain complete
  ├─ readiness: probes flip UP (indicator runs client.Ping)
  └─ SIGTERM → Destroy [client.go:82]: exec.Close → stop discovery watch → client.Close
```

A misconfigured mode (`mode=foo`) or a failed startup ping fails the boot — the process never
reaches "serving" with a dead Redis.

### 2.2 Command hook chain — exact order and why

go-redis hooks are FIFO: first added is outermost. Order as built:

```
redisotel (span + pool metrics) → observeHook (access log) → resilienceHook (breaker/...)
→ go-redis core → network
```

Rationale (source comments, [client.go:63-73] and [command.go:17-27]):

- **redisotel outermost**: added in the ctor (`instrument()`), before Init adds the rest. The
  span therefore covers everything the starter adds, and the access log rides redisotel's span
  context for trace_id correlation.
- **observeHook outside the breaker**: one access-log line covers the whole retry loop — you
  log the outcome, not each attempt. It is log-only by construction [observe.go]: redisotel
  already owns trace/metric, so the hook emits only the access log.
- **resilienceHook innermost**: protection decisions sit closest to the wire; its rejections are
  what the outer layers then observe.
- `DialHook` is left untouched in both hooks — connection establishment is discovery's concern,
  not command-level protection.

### 2.3 One command through the chain: `GET user:1` on a miss

1. redisotel starts the client span (no-op without starter-otel's globals).
2. observeHook starts an access-log record named `get` (cmd.FullName()).
3. resilienceHook asks the executor for a permit (rate limiter / breaker scoped to the resource
   label, e.g. `redis:127.0.0.1:6379` — per instance, not per command [client.go:94-100]).
4. go-redis executes; the key is absent so it returns `redis.Nil`.
5. `run()` classifies `redis.Nil` as success via the nil-as-success predicate [command.go:94] —
   **a cache miss never trips the breaker**; retries are not driven for it either.
6. observeHook ends the record with `nilAsSuccess(err)` [command.go:147] — the miss is logged
   as a successful op, not an error.
7. redisotel ends the span; the caller sees `redis.Nil` exactly as plain go-redis would return it.

For a **pipeline**, resilienceHook wraps the whole batch in one executor run; `cmd.SetErr` fires
on every command only when the batch never actually ran (a rejection or injected fault) — a real
per-command failure is already recorded by go-redis and is not overwritten [command.go:84-90].

### 2.4 Discovery addressing & connection recycling (single mode)

When `service-name` is set, DefaultDriver replaces go-redis's dialer: every new connection
calls `lb.Pick()` (a round-robin loadbalance.Pool) for a live endpoint [driver.go:141-154]. The resolver keeps the endpoint
set fresh in the background. Combined with `conn-max-lifetime` (default 2m), connections recycle
onto updated addresses **without rebuilding the client** — that is why the default is a short 2m
rather than "unlimited" [config.go:95-97]. `addr` then never takes effect — when both are set the
starter logs a WARN naming the ignored `addr` at startup (the example sets a dummy `0.0.0.0:0`).
Setting `service-name` in sentinel or cluster mode is
rejected at startup: those topologies discover their own nodes [starter.go:170-194].

---

## 3. Per-key behavior reference

All keys live under `spring.go-redis.<name>.` — field-injection here is per-instance prefix
binding via `conf.BindEach` (NOT the absolute-property starter-Pool rule).

### 3.1 Topology & addressing

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `mode` | string | `single` | `single`/`sentinel` → bean embeds `*redis.Client`; `cluster` → `*redis.ClusterClient`. Any other value fails BindEach with "invalid mode". | Typo → boot error naming the instance. |
| `addr` | string | — | Single-mode target. ⚠ Exactly one of `addr` / `service-name` required in single mode (RequireAny). | Neither → boot error; both → service-name wins, startup WARN names the ignored `addr`. |
| `master-name` | string | — | Required in sentinel mode. | Missing → boot error "master-name and sentinel-addrs are required". |
| `sentinel-addrs` | list | — | Required in sentinel mode. | Missing → same boot error as above. |
| `sentinel-password` | string | — | Auth for the sentinels themselves (distinct from `password`). | Sentinel ACL on → probe errors at boot. |
| `addrs` | list | — | Cluster seed nodes; seeds only, the client learns the topology. Required in cluster mode. | Missing → boot error "addrs is required in cluster mode". |
| `max-redirects` | int | 0 | MOVED/ASK follow count in cluster mode; 0 → go-redis default (3). | — |
| `route-by-latency` / `route-randomly` | bool | false | Cluster read routing. Both set → go-redis semantics. | — |
| `service-name` | string | — | Single mode only: resolve addr via discovery. ⚠ Rejected with sentinel/cluster modes. | Wrong combo → boot error; see §2.4. |
| `scheme` | string | — | Narrows discovery endpoints to one transport scheme. Only consulted when service-name set. | — |
| `discovery` | string | `default` | Which registered discovery backend resolves service-name. | Unregistered backend → boot error from discovery. |

### 3.2 Connection & auth

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `password` / `username` | string | — | Server/ACL auth (master in sentinel mode). | Wrong → failFastPing fails at boot. |
| `db` | int | 0 | SELECT on connect (single/sentinel only). Redis Cluster has no databases: `db != 0` with `mode=cluster` → boot error "db is not supported in cluster mode". | Out-of-range → first command errors. |
| `pool-size` | int | 10 | Max socket connections. | Too low → wait contention under burst. |
| `max-idle` | int | 5 | Max idle conns (go-redis MaxIdleConns). | — |
| `max-retries` | int | 0 | go-redis command retries. ⚠ Keep the RESILIENCE retry at 0 too — double retry loops amplify latency and can re-send non-idempotent commands (config.go:152-154 comment). | Large value + resilience retry → multiplied attempts. |
| `dial-timeout` / `read-timeout` / `write-timeout` | duration | 5s / 3s / 3s | Passed through; dial-timeout also bounds the startup ping [starter.go:229]. | — |
| `conn-max-lifetime` | duration | 2m | Conn reuse window; short values smooth discovery traffic switching. | Very large + discovery → stale-endpoint conns linger. |
| `tls.*` | group | off | `tlsconf` client TLS (enabled/ca-file/cert-file/key-file/server-name/insecure-skip-verify). | Partial config → `tls.Build` error at boot. |
| `health.enabled` | bool | true | Contributes the `redis:<name>` health.Indicator for the instance — the same switch starter-redigo exposes. | false → no indicator bean for the instance; readiness of that Redis is no longer reported. |

### 3.3 Instrumentation

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `otel.tracing.enabled` | bool | true | Attach redisotel spans. No-op without starter-otel. | Off + expecting traces → silence, no warning. |
| `otel.metrics.enabled` | bool | true | Attach redisotel pool/hit metrics. Same no-op rule. | — |

### 3.4 Cache driver reference syntax

`spring.cache.<name>.driver = go-redis:<redis-instance-name>` registers a `*cache.Cache` bean
(wrapping `bytecache.NewByteCache(c.UniversalClient)`) **named after the redis instance**
[starter.go:80-89]. `redis.Nil` is mapped to `cache.ErrMiss` at this boundary
[bytecache/bytecache.go:42-51].

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .      # components include "redis:main", "redis:cluster", ...
docker stop <redis>              # indicator runs client.Ping → component flips DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <redis>
```

### 4.2 Observe access log + metrics

```bash
redis-cli SET probe 1
grep _app_redis_access app.log | tail -1
# op=set status=ok duration=...; keyed successes log at Debug, errors at Warn
curl -s :9370/metrics | grep -E 'redis.*pool|hits'   # redisotel gauges/counters
```

### 4.3 Discovery address recycling

```properties
spring.go-redis.main.service-name=redis-cluster
spring.go-redis.main.conn-max-lifetime=30s
```

Scale/move the backing instance; within conn-max-lifetime new connections dial the updated
endpoint (round-robin pool pick per dial). Watch `PoolStats()` (`TotalConns`/`Hits`) or redisotel pool
metrics to confirm recycling without a restart.

### 4.4 Cache driver wiring (SET via façade, GET via raw client)

```go
_ = s.Cache.Set(ctx, "k", "v", time.Minute)   // typed, JSON codec
val, _ := s.Main.Get(ctx, "k").Result()       // raw client reads the same key: "v" (JSON-quoted string)
```

The façade stores JSON bytes (`cache.Cache` default codec) — the raw GET returns the encoded
form. A miss through the façade returns `cache.ErrMiss`, not `redis.Nil`.

### 4.5 Fault / resilience drill

With starter-governance configured, set a breaker/limiter policy for resource `redis:<addr>`;
hammer the instance and watch rejections surface in `_app_redis_access` records and the
resilience observer's outcome counters. Flip policy at runtime — the executor hot-reloads
without restart.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "startup ping failed" | Unreachable addr / wrong password / TLS mismatch | failFastPing is unconditional; fix connectivity or credentials. |
| Boot fails "invalid mode ... (want single/sentinel/cluster)" | Typo in `mode` | Correct the value — modes are exact-match. |
| Boot fails "service-name is not supported in sentinel/cluster mode" | Discovery + self-discovering topology | Remove service-name; sentinel/cluster discover nodes themselves. |
| Boot fails "db is not supported in cluster mode" | Redis Cluster has no database select | Drop `db` (cluster only has db 0). |
| Startup WARN "addr ... is ignored" | Both `addr` and `service-name` set in single mode | Harmless; remove `addr` or keep it as a label — discovery owns addressing. |
| Boot fails "... does not support cluster mode" | A provided Driver bean isn't cluster-capable but a `mode=cluster` instance exists | Have the Driver implement `ClusterDriver` (the bundled `DefaultDriver` does); the one process-wide Driver must cover every topology in use. |
| Health DOWN though commands work | Indicator pings with ctx; check ACL/readonly replica | Inspect the component error body in /readiness. |
| Injected bean has no spans/metrics | starter-otel not imported | redisotel rides the OTel globals; import starter-otel. |
| No access log lines | logger config filters `_app_redis_access` or the Debug level (keyed successes log at Debug) | Check logger config for `_app_redis_access`. |
| Breaker trips on every GET miss | It does not — redis.Nil is success [command.go:94] | Look for a real backend error; misses are excluded. |
| Cache bean inject fails | façade bean is named after the redis instance, not the spring.cache key | Autowire by `<redis-instance-name>`; see starter-cache USAGE. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 25 instance keys + tls group + otel(2) |
| Required | 1 per mode (addr/service-name, master-name+sentinel-addrs, or addrs) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 6 |

Design suspects (audit ledger): ~~health indicator has no opt-out key~~ (fixed: `health.enabled`
now mirrors redigo); cache façade bean named after the backend instance rather
than the `spring.cache` map key (surprising inject name; two spring.cache entries on one
instance would collide); `max-retries` (go-redis) vs resilience retry is a foot-gun documented
only in a config comment.

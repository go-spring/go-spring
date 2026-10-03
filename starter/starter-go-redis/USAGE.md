# starter-go-redis Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`,
`driver.go`, `health.go`, `bytecache.go`) and the runnable [example/](example/) — file:line
spot-checks in brackets below. **Redis semantics and the go-redis API are [go-redis's own
documentation](https://redis.io/docs/latest/develop/clients/go/)** — everything below is
go-spring's increment.

**Activation**: any `spring.go-redis.instances.*` key (the module is `OnProperty("spring.go-redis")`, a
prefix check). Each `spring.go-redis.instances.<name>` entry creates one `*StarterGoRedis.Client` bean
named `<name>`, plus a health indicator named `redis:<name>`.

---

## 1. Complete worked project

Three topologies in one service, plus a cache façade and probes/metrics/traces. File tree:

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
    go-spring.org/starter-actuator latest   // optional: readiness + /metrics
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — inject the wrapper, use the full go-redis command surface, and the
cache façade:

```go
package service

import (
    "context"

    "go-spring.org/cloud/cache"
    "go-spring.org/spring/gs"
    StarterGoRedis "go-spring.org/starter-go-redis"
)

type Service struct {
    // Always the wrapper type *StarterGoRedis.Client. It embeds the raw
    // redis.UniversalClient, so Get/Set/Incr/Pipeline/PoolStats are promoted
    // and available unchanged whether the instance is single, sentinel, or
    // cluster.
    Main     *StarterGoRedis.Client `autowire:"main"`     // single
    Sentinel *StarterGoRedis.Client `autowire:"sentinel"` // sentinel
    Cluster  *StarterGoRedis.Client `autowire:"cluster"`  // cluster

    // The typed cache façade over the "main" instance — the *cache.Cache
    // bean named "go-redis:main" (see §3.4).
    Cache *cache.Cache `autowire:"go-redis:main"`
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
spring.go-redis.instances.main.addr=127.0.0.1:6379
spring.go-redis.instances.main.pool-size=20
spring.go-redis.instances.main.conn-max-lifetime=2m

# --- sentinel: master group resolved through the sentinel nodes -----------
spring.go-redis.instances.sentinel.mode=sentinel
spring.go-redis.instances.sentinel.master-name=mymaster
spring.go-redis.instances.sentinel.sentinel-addrs=127.0.0.1:26379,127.0.0.1:26380

# --- cluster: seed nodes; client learns the full topology itself ----------
spring.go-redis.instances.cluster.mode=cluster
spring.go-redis.instances.cluster.addrs=127.0.0.1:7000,127.0.0.1:7001,127.0.0.1:7002
spring.go-redis.instances.cluster.route-by-latency=true

# --- observability ---------------------------------------------------------
# Access log (tag _app_redis_access) is emitted by the resilience layer; filter via logger config.
# redisotel pool-metrics are ON by default and ride starter-otel's globals.

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
  └─ gs.Module(OnProperty("spring.go-redis")) fires when any spring.go-redis.instances.* key exists
        └─ conf.BindEach("${spring.go-redis}") → one Config per <name> entry
              ├─ mode single/sentinel → Provide(newClient).Name(<name>).Destroy((*Client).Destroy)
              ├─ mode cluster          → Provide(newClusterClient).Name(<name>) (same wrapper type)
              └─ Provide health.Indicator named "redis:<name>" (gated by health, default on)

gs.Run()
  └─ ctor newClient [starter.go:143] — assembles the client COMPLETELY, then probes:
       ├─ validateConfig → driver lookup → driver.CreateClient(ctx, c, disc, params)
       │    (the ctor bundles the injected beans into
       │     cloud.ClientParams{Resilience: mgr, Fault: inj, Loadbalance: lbMgr})
       │    └─ DefaultDriver builds the raw client, hands it (with params) to NewClient [client.go:88]
       │         → instrument() (redisotel pool metrics, gated by otel.metrics.enabled)
       │         → applyDeclaration (identity hook)   — declaration is part of construction
       │         → serviceLabel → params.ExecutorFor("redis", service)
       │              (= fault.WrapClientExecutor(mgr.ClientExecutorFor("redis", service), service, inj);
       │               a zero bundle degrades to an observe-only Unmanaged executor)
       │         → lbMgr.Bind(pool, label) for a discovery-routed entry
       │         → AddHook(resilienceHook) — command chain complete
       └─ if ping: startup HealthCheck on the raw client (bounded by dial-timeout or 5s); on failure
          Destroy releases what was just assembled
  ├─ readiness: probes flip UP (indicator runs HealthCheck)
  └─ SIGTERM → Destroy [client.go:148]: exec.Close → detach selection → client.Close
```

There is no `Init` hook: everything the old Init did (label, executor, selection binding,
hook order) happens inside the ctor, so gs only has to know how to DESTROY the bean.

A misconfigured mode (`mode=foo`) always fails the boot; so does a failed startup ping — but that
probe only runs when `ping=true`. With the default `false` a dead Redis is not caught at boot; the
first command fails instead.

### 2.2 Command hook chain — exact order and why

go-redis hooks are FIFO: first added is outermost. Order as built:

```
redisotel (pool metrics) → operationHook (declares identity) → resilienceHook (breaker/... + emission)
→ go-redis core → network
```

Rationale (source comments, [client.go] and [command.go]):

- **redisotel outermost**: added by `NewClient` (`instrument()`), before `NewClient` adds the
  resilience hook. Only its pool metrics remain — the per-command span it used to attach is gone,
  because it duplicated the call span the resilience layer now opens.
- **operationHook outside the breaker**: it puts the command's identity on the ctx
  (`observability.WithOperation`, [observe.go]) and emits nothing. A skipped op (PING) declares no
  identity and is forwarded untouched.
- **resilienceHook innermost**: protection decisions sit closest to the wire, and it is the single
  emitter — reading the declared identity off the ctx, it opens the one call span (covering the
  whole retry loop), records the call-level and attempt-level duration histograms and the status
  counter, and writes one access-log line.
- `DialHook` is left untouched in both hooks — connection establishment is discovery's concern,
  not command-level protection.

### 2.3 One command through the chain: `GET user:1` on a miss

1. operationHook puts the command's identity on the ctx: span name `get` (cmd.FullName()), labels
   `db.system`/`db.operation`, and the key as `db.statement` span/log detail — bounded to 512 bytes
   [observe.go]. The key rides as span/log detail, never as a metric label, because keys are unbounded.
2. resilienceHook asks the executor for a permit (rate limiter / breaker scoped to the service
   label, e.g. `redis:127.0.0.1:6379` — per instance, not per command [client.go:165]).
3. go-redis executes; the key is absent so it returns `redis.Nil`.
4. `run()` classifies `redis.Nil` as success via the nil-as-success predicate (Tolerate)
   [command.go] — **a cache miss never trips the breaker**; retries are not driven for it either.
5. The resilience layer emits: the span ends, `db.client.operation.duration` (whole call) and
   `db.client.attempt.duration` (per try) record, and the access-log line is written with a
   *success* status — a miss is a successful op, not an error. It rides the caller's ctx, so the
   span links to the request trace.
6. The caller sees `redis.Nil` exactly as plain go-redis would return it.

For a **pipeline**, resilienceHook wraps the whole batch in one executor run; `cmd.SetErr` fires
on every command only when the batch never actually ran (a rejection or injected fault) — a real
per-command failure is already recorded by go-redis and is not overwritten [command.go:84-90].

### 2.4 Discovery addressing & connection recycling (single mode)

When `service-name` is set, DefaultDriver replaces go-redis's dialer: every new connection
calls `lb.Pick()` (a round-robin loadbalance.Pool) for a live endpoint [driver.go]. The resolver keeps the endpoint
set fresh in the background. Combined with `conn-max-lifetime` (default 2m), connections recycle
onto updated addresses **without rebuilding the client** — that is why the default is a short 2m
rather than "unlimited" [config.go]. `addr` then never takes effect — when both are set the
starter logs a WARN naming the ignored `addr` at startup (the example sets a dummy `0.0.0.0:0`).
Setting `service-name` in sentinel or cluster mode is
rejected at startup: those topologies discover their own nodes [starter.go].

**The pool's strategy is governed, not hardcoded.** It is built with a suspension tracker, passed by the Driver to
`NewClient` (which keeps it on the wrapper), and bound to
`redis:<service-name|master-name|addr>` by `NewClient` via the governance bundle's `lbMgr.Bind(pool, label)` — the
binding lives inside the constructor so a company Driver only has to pass the bundle through, and
`spring.governance.client.rules[N].balancer` / `outlier-threshold` / `outlier-suspend-for` for that label drive it
in place — the next dial uses the new strategy. The dialer feeds `Complete` with the dial
outcome, so `outlier-threshold` evicts instances that keep refusing *connections*. Sentinel and
cluster clients self-discover and have no pool, so these keys do not reach them. See
`cloud/governance/README.md` §3.1.

---

## 3. Per-key behavior reference

All keys live under `spring.go-redis.instances.<name>.` — field-injection here is per-instance prefix
binding via `conf.BindEach` (NOT the absolute-property starter-Pool rule).

### 3.1 Topology & addressing

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `mode` | string | `single` | Every mode registers the same bean type, `*StarterGoRedis.Client`; the raw client it wraps is `*redis.Client` for `single`/`sentinel` and `*redis.ClusterClient` for `cluster`. Any other value fails BindEach with "invalid mode". | Typo → boot error naming the instance. |
| `addr` | string | — | Single-mode target. ⚠ Exactly one of `addr` / `service-name` required in single mode (RequireAny). | Neither → boot error; both → service-name wins, startup WARN names the ignored `addr`. |
| `master-name` | string | — | Required in sentinel mode. | Missing → boot error "master-name and sentinel-addrs are required". |
| `sentinel-addrs` | list | — | Required in sentinel mode. | Missing → same boot error as above. |
| `sentinel-password` | string | — | Auth for the sentinels themselves (distinct from `password`). | Sentinel ACL on → probe errors at boot. |
| `addrs` | list | — | Cluster seed nodes; seeds only, the client learns the topology. Required in cluster mode. | Missing → boot error "addrs is required in cluster mode". |
| `max-redirects` | int | 0 | MOVED/ASK follow count in cluster mode; 0 → go-redis default (3). | — |
| `route-by-latency` / `route-randomly` | bool | false | Cluster read routing. Both set → go-redis semantics. | — |
| `service-name` | string | — | Single mode only: resolve addr via discovery. ⚠ Rejected with sentinel/cluster modes. | Wrong combo → boot error; see §2.4. |
| `scheme` | string | — | Narrows discovery endpoints to one transport scheme. Only consulted when service-name set. | — |
| `discovery` | string | — | Which registered discovery backend resolves service-name. Falls back to `${spring.go-redis.default.discovery}` when unset. | Both unset or an unregistered name while service-name is set → boot error. |

### 3.2 Connection & auth

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `password` / `username` | string | — | Server/ACL auth (master in sentinel mode). | Wrong → the startup ping fails at boot. |
| `db` | int | 0 | SELECT on connect (single/sentinel only). Redis Cluster has no databases: `db != 0` with `mode=cluster` → boot error "db is not supported in cluster mode". | Out-of-range → first command errors. |
| `pool-size` | int | 10 | Max socket connections. | Too low → wait contention under burst. |
| `max-idle` | int | 5 | Max idle conns (go-redis MaxIdleConns). | — |
| `max-retries` | int | 0 | go-redis command retries. ⚠ Keep the RESILIENCE retry at 0 too — double retry loops amplify latency and can re-send non-idempotent commands (config.go:152-154 comment). | Large value + resilience retry → multiplied attempts. |
| `dial-timeout` / `read-timeout` / `write-timeout` | duration | 5s / 3s / 3s | Passed through; dial-timeout also bounds the startup HealthCheck when `ping=true` [starter.go]. | — |
| `conn-max-lifetime` | duration | 2m | Conn reuse window; short values smooth discovery traffic switching. | Very large + discovery → stale-endpoint conns linger. |
| `tls.*` | group | off | `security` client TLS (enabled/ca-file/cert-file/key-file/server-name/insecure-skip-verify). | Partial config → `tls.Build` error at boot. |
| `ping` | bool | false | Opt-in startup probe: `HealthCheck` pings once and fails the boot on an unreachable server; off by default so a backend that is not up yet only surfaces on first use. | Expecting fail-fast without setting it → boot "succeeds", first request fails. |
| `health` | bool | true | Contributes the `redis:<name>` health.Indicator for the instance — the same switch starter-redigo exposes. | false → no indicator bean for the instance; readiness of that Redis is no longer reported. |

### 3.3 Instrumentation

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `otel.metrics.enabled` | bool | true | Attach redisotel pool/hit metrics. No-op without starter-otel. | — |
| `otel.tracing.enabled` | bool | — | **REMOVED.** Still binds (a config that kept it does not fail to start) and is ignored; setting it logs one warning at construction. Per-command spans are emitted by the resilience layer, so a redisotel span here would be a second one for the same call — delete the key. | — |

The per-command span, the duration histograms and the access log are not configured here: the
starter only declares each command's identity (`observe.go`), and the resilience layer emits them.
There is no per-instance opt-out for that — it is the family-wide default.

### 3.4 Cache abstraction bean

Alongside the wrapper, each instance is provided as a `*cache.Cache` bean (wrapping
`NewByteCache(c)`, which accepts the wrapper directly) named `go-redis:<redis-instance-name>`
[starter.go] — inject `*cache.Cache` with the autowire tag `go-redis:<instance-name>`.
Un-injected, the bean never instantiates, so there is no config switch to set.
`redis.Nil` is mapped to `cache.ErrMiss` at this boundary
[`bytecache.go`].

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz | jq .      # components include "redis:main", "redis:cluster", ...
docker stop <redis>              # indicator runs HealthCheck → component flips DOWN
curl -s :9370/readyz             # 503 OUT_OF_SERVICE
docker start <redis>
```

### 4.2 Observe access log + metrics

```bash
redis-cli SET probe 1
grep _app_redis_access app.log | tail -1
# db.system=redis db.operation=set db.statement=probe status=ok duration_ms=...
# keyed successes log at Debug, failures at Warn — emitted by the resilience layer
curl -s :9370/metrics | grep -E 'redis.*pool|hits'   # redisotel pool gauges/counters
curl -s :9370/metrics | grep -E 'db.client'          # call-level + attempt-level histograms
```

### 4.3 Discovery address recycling

```properties
spring.go-redis.instances.main.service-name=redis-cluster
spring.go-redis.instances.main.conn-max-lifetime=30s
```

Scale/move the backing instance; within conn-max-lifetime new connections dial the updated
endpoint (round-robin pool pick per dial). Watch `PoolStats()` (`TotalConns`/`Hits`) or redisotel pool
metrics to confirm recycling without a restart.

### 4.4 Cache abstraction wiring (SET via façade, GET via raw client)

```go
_ = s.Cache.Set(ctx, "k", "v", time.Minute)   // typed, JSON codec
val, _ := s.Main.Get(ctx, "k").Result()       // raw client reads the same key: "v" (JSON-quoted string)
```

The façade stores JSON bytes (`cache.Cache` default codec) — the raw GET returns the encoded
form. A miss through the façade returns `cache.ErrMiss`, not `redis.Nil`.

### 4.5 Fault / resilience drill

With a configured governance rules source, set a breaker/limiter policy for service `redis:<addr>`;
hammer the instance and watch rejections surface in `_app_redis_access` records and the
resilience observer's outcome counters. Flip policy at runtime — the executor hot-reloads
without restart.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "startup ping failed" | Unreachable addr / wrong password / TLS mismatch | Emitted only when `ping=true`; fix connectivity or credentials. With `ping` off a dead Redis surfaces on the first command instead. |
| Boot fails "invalid mode ... (want single/sentinel/cluster)" | Typo in `mode` | Correct the value — modes are exact-match. |
| Boot fails "service-name is not supported in sentinel/cluster mode" | Discovery + self-discovering topology | Remove service-name; sentinel/cluster discover nodes themselves. |
| Boot fails "db is not supported in cluster mode" | Redis Cluster has no database select | Drop `db` (cluster only has db 0). |
| Startup WARN "addr ... is ignored" | Both `addr` and `service-name` set in single mode | Harmless; remove `addr` or keep it as a label — discovery owns addressing. |
| Boot fails "... does not support cluster mode" | A provided Driver bean isn't cluster-capable but a `mode=cluster` instance exists | Have the Driver implement `ClusterDriver` (the bundled `DefaultDriver` does); the one process-wide Driver must cover every topology in use. |
| Health DOWN though commands work | Indicator pings with ctx; check ACL/readonly replica | Inspect the component error body in /readiness. |
| Injected bean has no spans/metrics | starter-otel not imported | The resilience layer and redisotel ride the OTel globals; import starter-otel. |
| No access log lines | logger config filters `_app_redis_access` or the Debug level (keyed successes log at Debug) | Check logger config for `_app_redis_access`. |
| Breaker trips on every GET miss | It does not — redis.Nil is success [command.go:94] | Look for a real backend error; misses are excluded. |
| Cache bean inject fails | the `*cache.Cache` bean is named `go-redis:<redis-instance-name>`, not `<instance-name>` | Autowire by `go-redis:<redis-instance-name>`; see §3.4. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 26 instance keys + tls group + otel(1) |
| Required | 1 per mode (addr/service-name, master-name+sentinel-addrs, or addrs) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 6 |

Design suspects (audit ledger): ~~health indicator has no opt-out key~~ (fixed: `health`
now mirrors redigo); ~~cache façade bean named after the backend instance rather
than the `spring.cache` map key~~ (resolved: the `go-redis:<name>` bean name is now the
contract, injected directly by name); `max-retries` (go-redis) vs resilience retry is a
foot-gun documented only in a config comment.

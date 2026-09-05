# starter-redigo Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `pool.go`, `conn.go`, `driver.go`,
`health/health.go`, `bytecache/`) and the runnable [example/](example/) — file:line spot-checks
in brackets. **Redigo semantics (the Do / connection-borrow model, reply helpers) are
[redigo's own documentation](https://github.com/gomodule/redigo)** — everything below is
go-spring's increment. Field layout deliberately mirrors starter-go-redis single mode, so
switching between the two is an import + prefix change.

**Activation**: any `spring.redigo.*` key. Each `spring.redigo.<name>` entry creates one
`*StarterRedigo.Pool` bean named `<name>`, plus a health indicator named `redigo:<name>`
(when `health.enabled`, default true).

---

## 1. Complete worked project

A service with a static pool, a discovery-backed pool, a user command interceptor, and the
cache façade. File tree:

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
    github.com/gomodule/redigo     latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-redigo   latest
    go-spring.org/starter-cache    latest   // cache façade
    go-spring.org/starter-actuator latest   // optional
    go-spring.org/starter-otel     latest   // optional
    go-spring.org/starter-governance latest // optional
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-cache"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-redigo"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — the borrow/Do pattern plus an interceptor:

```go
package service

import (
    "context"
    "time"

    "github.com/gomodule/redigo/redis"
    "go-spring.org/cloud/cache"
    "go-spring.org/spring/gs"
    StarterRedigo "go-spring.org/starter-redigo"
)

type Service struct {
    // Always the wrapper *StarterRedigo.Pool; it embeds *redis.Pool, so
    // Get/Stats promote unchanged. Connections it hands out are instrumented Conns.
    Main      *StarterRedigo.Pool `autowire:"main"`
    Discovery *StarterRedigo.Pool `autowire:"discovery"`

    // spring.cache.demo.driver=redigo:main — the typed façade over the "main" pool.
    Cache *cache.Cache `autowire:"main"`
}

func init() {
    gs.Provide(func(s *Service) gs.Init {
        return func(ctx context.Context) {
            // Register interceptors in an Init (before the pool hands out
            // connections in anger); FIRST registered = OUTERMOST layer.
            s.Main.UseCommandInterceptor(func(next StarterRedigo.CommandHandler) StarterRedigo.CommandHandler {
                return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
                    if cmd == "GET" && args[0] == "local:skip" {
                        return "served-locally", nil // short-circuit: no span, no breaker permit
                    }
                    return next(ctx, cmd, args)
                }
            })
        }
    })

    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // The canonical borrow/Do pattern.
            c := s.Main.Get()
            defer func() { _ = c.Close() }()
            if _, err := redis.String(c.(*StarterRedigo.Conn).DoContext(ctx, "SET", "key", "value")); err != nil {
                panic(err)
            }
            _ = s.Cache.Set(ctx, "user:1", "demo", time.Minute)
        }
    })
}
```

**conf/app.properties**:

```properties
# --- main pool: static address, fail-fast dial check ------------------------
spring.redigo.main.addr=127.0.0.1:6379
spring.redigo.main.startup-ping=true
spring.redigo.main.pool-size=20
spring.redigo.main.conn-max-lifetime=2m

# --- discovery pool: addr ignored, address picked per dial -------------------
spring.redigo.discovery.service-name=redis-cluster
spring.redigo.discovery.conn-max-lifetime=30s

# --- instrumentation ---------------------------------------------------------
# Span + duration metric + access log are built in and unconditional; they are
# no-ops unless starter-otel installs providers.

# --- health -------------------------------------------------------------------
# default true; set false to keep a non-critical cache out of aggregate health
spring.redigo.main.health.enabled=true

# --- cache façade ------------------------------------------------------------
spring.cache.demo.driver=redigo:main

# --- actuator + otel ----------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
```

**Verify**:

```bash
docker run -d -p 6379:6379 redis
go run .                          # startup-ping fails boot on a bad address
curl -s :9370/readyz              # components include redigo:main, redigo:discovery
redis-cli GET key                 # "value" — written through the onion
redis-cli GET user:1              # JSON written via the cache façade
grep _app_redigo_access app.log   # one access record per Do
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-redigo
  └─ gs.Module(OnProperty("spring.redigo")) fires when any spring.redigo.* key exists
        └─ conf.BindEach("${spring.redigo}") → one Config per <name>
              ├─ Provide(createPool).Name(<name>).Destroy(destroyPool)
              │    ctor args: ContextProvider, Config (IndexArg 1)
              └─ if health.enabled → Provide health.Indicator named "redigo:<name>"

gs.Run()
  ├─ ctor createPool [starter.go:107]: RequireAny(addr|service-name) → driver lookup
  │   → d.CreateClient (= NewPool): TLS build → discovery resolver → raw pool
  │     → observer → resilience executor → setupDial
  │     → startup-ping (ONLY if startup-ping=true) [starter.go:144-149]
  │   NOTE: there is NO separate InitMethod — the pool is fully armed on return [pool.go:36-37]
  ├─ your bean Inits may call UseCommandInterceptor (affects conns dialed from then on)
  └─ SIGTERM → destroyPool → Pool.Close: exec.Close → resolver.Stop → pool.Close [pool.go:141-149]
```

`Pool.Close` shadows the embedded `(*redis.Pool).Close` so a plain Close cannot leak the
discovery-resolver watch [pool.go:137-140].

### 2.2 The command onion — folded at connection construction

Every connection the pool dials is wrapped ONCE, at dial time, by `Pool.wrapConn`
[pool.go:212-222]. The layers are folded into a single composed `CommandInterceptor`
(`NewConn` folds from the innermost, so the FIRST layer ends up OUTERMOST [conn.go:93-106]):

```
user interceptors (first-registered outermost)
  → observe layer (span + duration metric + access log)
    → resilience executor (breaker / limiter / retry / timeout)
      → the inner Do call → Redis
```

Rationale (source comments [pool.go:205-211], [conn.go:44-53]):

- **User interceptors outermost**: a layer can short-circuit WITHOUT starting a span or
  consuming a breaker permit, rewrite ctx/cmd/args, or just observe the outcome. An
  observer-style layer that wants the command to count must call `next`.
- **Span outside the executor**: one Execute — including any retries the policy drives —
  shares a single span and one access-log line.
- **Executor innermost**: it derives the per-attempt context and gates the actual wire call.

Because the fold happens per dial, interceptors registered via `UseCommandInterceptor` **after**
some connections were already dialed apply only to connections dialed later; already-dialed
connections keep the chain they were built with [pool.go:151-157]. Register them from a bean's
Init, before traffic.

### 2.3 One command through the onion: `DoContext(ctx, "GET", "key")` on a hit

1. Your interceptor (if any) runs first; may rewrite or short-circuit.
2. observe layer starts a span named `get` with a summarized argument `GET key`
   (command + first arg only — values are never logged; bounded to 512 bytes
   [observe.go]). ctx is the CALLER's context, so the span links to the request trace
   and an attempt-timeout can interrupt the call.
3. resilience layer asks the executor (resource label `redigo:<addr-or-service-name>`,
   per pool [pool.go:190]) for a permit; on retry-able failures it re-drives the inner call.
4. The inner `Do` writes/reads Redis; a hit returns the bulk string.
5. `redis.ErrNil` (a miss) is classified success via the nil-as-success predicate
   [conn.go:201-204] — **a miss never trips the breaker**.
6. The span ends; the access-log record (tag `_app_redigo_access`) emits with duration/status.

`Do` / `DoWithTimeout` carry no context: their spans are roots and an attempt-timeout cannot
interrupt them — prefer `DoContext` when either matters [conn.go:74-79]. `Send`/`Flush`/`Receive`
(pipelining) are left uninstrumented by design.

### 2.4 One pooled connection's lifecycle

```
pool.Get() (your code)
  ├─ idle conn available? → reuse (MaxConnLifetime=conn-max-lifetime bounds reuse)
  └─ else Dial: credentials/TLS/SELECT db → the round-robin loadbalance pool picks a
     endpoint when service-name is set [pool.go:104-120] → wrapConn folds the onion
  ├→ you Do/DoContext commands (each flows user → observe → resilience → wire)
  └→ conn.Close(): returns to the idle pool (redigo semantics)
shutdown: Pool.Close() — executor, resolver watch, then the pool itself
```

`Wait: true` is hardcoded [pool.go:86]: when the pool is exhausted, borrowers block instead of
erroring — size `pool-size` accordingly.

---

## 3. Per-key behavior reference

All keys live under `spring.redigo.<name>.`.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `addr` | string | — | Static target. ⚠ Exactly one of `addr` / `service-name` required (RequireAny [starter.go:111]). | Neither → boot error; both → service-name wins, addr ignored. |
| `service-name` | string | — | Discovery-resolved address; `addr` becomes a label only. Per-dial endpoint pick + conn-max-lifetime recycling. | Unregistered backend → boot error. |
| `scheme` | string | — | Narrows discovery endpoints to one scheme. Only with service-name. | — |
| `discovery` | string | `default` | Which discovery backend resolves service-name. | — |
| `password` / `username` | string | — | Dial auth; username only appended when non-empty [pool.go:94-96]. | Wrong → first dial (or startup-ping) fails. |
| `db` | int | 0 | `SELECT` executed on each fresh conn (non-zero only) [pool.go:125-131]. | Out-of-range → dial fails. |
| `pool-size` | int | 10 | MaxActive. `Wait:true` → borrowers block when exhausted. | Too low → latency, not errors. |
| `max-idle` | int | 5 | MaxIdle. | Higher than pool-size is pointless. |
| `dial-timeout` / `read-timeout` / `write-timeout` | duration | 5s / 3s / 3s | Dial options. | — |
| `conn-max-lifetime` | duration | 2m | MaxConnLifetime; short values smooth discovery traffic switching. | Very large + discovery → stale endpoints linger. |
| `tls.*` | group | off | Client TLS; keys mirror starter-go-redis. | Partial → tls.Build boot error. |
| `driver` | string | `DefaultDriver` | Selects a registered Driver. | Unknown → boot error "redigo driver not found". |
| `startup-ping` | bool | false | Opt-in boot probe: dials ONE bare conn and PINGs [pool.go:266-279]. ⚠ Off by default — the pool is lazy, so a bad address surfaces only on first command. | Expecting fail-fast without setting it → boot "succeeds", first request fails. |
| `health.enabled` | bool | true | Registers `redigo:<name>` indicator; false keeps the pool out of aggregate health. | false → readiness silently excludes this pool. |

**Extension points**:

- `Pool.UseCommandInterceptor(...CommandInterceptor)` — the per-command onion above; first
  registered outermost; nil interceptor panics.
- `StarterRedigo.NewConn(raw, layers...)` — hand-assembly primitive for REPLACE-shaped drivers.
- `Driver` / `RegisterDriver` — owns the full pool assembly; two shapes documented in
  [driver.go:43-51]: ADD (call NewPool, customize via public API) or REPLACE (own it entirely).
  The example's `AnotherRedisDriver` shows the delegate-then-customize shape.

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz            # redigo:main borrows a conn and PINGs [health/health.go:32-39]
docker stop <redis>; curl -s :9370/readyz   # 503
```

### 4.2 Observe access log + spans

```bash
redis-cli SET probe 1
grep _app_redigo_access app.log | tail -1
# op=set status ok duration=... ; "GET probe" appears in detailed level
curl -s :9370/metrics | grep -E 'redigo|db.client'   # duration histogram + in-flight gauge
```

### 4.3 Discovery address recycling

With the `discovery` instance (`service-name` + `conn-max-lifetime=30s`), move/scale the
backing Redis; `Stats()` (`ActiveCount`/`IdleCount`) shows conns recycling onto the new
endpoint within 30s — no restart, no client rebuild.

### 4.4 Cache driver wiring

```go
_ = s.Cache.Set(ctx, "k", "v", time.Minute)
v, _ := redis.String(s.Main.Get().(*StarterRedigo.Conn).Do("GET", "k"))  // reads the JSON form
```

A façade miss returns `cache.ErrMiss`; the raw Do returns `redis.ErrNil` — the boundary maps
one to the other (starter-redigo/bytecache).

### 4.5 Interceptor short-circuit drill

Hit the interceptor's key (`GET local:skip` in §1): the reply comes back with NO new
`_app_redigo_access` line and no breaker permit consumed — proof the user layer sits outside
both. Calling `next` instead restores full instrumentation.

### 4.6 Startup-ping drill

Set `startup-ping=true` and a wrong `addr`: boot fails with "redis: startup ping failed".
Set it false: boot succeeds and the first command fails instead — the trade-off the key controls.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot OK, first command "connection refused" | pool is lazy; `startup-ping` off | Set `startup-ping=true` for fail-fast. |
| Boot fails "redis: startup ping failed" | addr/auth/TLS/discovery broken | It dials one bare conn; fix connectivity. |
| Boot fails "one of addr/service-name required" | neither key set | Set exactly one. |
| Interceptor never runs | registered after conns were dialed | Register from a bean Init before traffic [pool.go:151-157]. |
| Spans not linked to request trace | using `Do`/`DoWithTimeout` (root spans) | Use `DoContext` [conn.go:74-79]. |
| No span/metric/log at all | starter-otel not imported | Import starter-otel to install providers. |
| Latency spikes, no errors | pool exhausted + `Wait:true` | Raise `pool-size`; watch Stats(). |
| Custom driver's pool lacks instrumentation | driver built a raw pool | Return the wrapped Pool from NewPool/NewConn assembly; createPool re-attaches cfg only [starter.go:95-99]. |
| Breaker trips on misses | It does not — ErrNil is success | Look for real failures. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 17 instance keys + tls group |
| Required | 1 (`addr` or `service-name`) |
| Quickstart external deps | 1 (Redis) |
| "Watch out" entries | 6 |

Design suspects (audit ledger): `startup-ping` opt-in here but unconditional in starter-go-redis
(family asymmetry, documented at config.go:85-91); type assertion needed to reach
`DoContext` on a borrowed conn (`pool.Get()` returns `redis.Conn`).

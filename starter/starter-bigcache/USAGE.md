# starter-bigcache Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`health.go`, `bytecache.go`) and the self-asserting [example/](example/) — file:line
spot-checks in brackets. **BigCache semantics (sharding, life-window eviction, hard size cap,
byte-only values) are [the official README](https://github.com/allegro/bigcache)** — everything
below is go-spring's increment.

**Activation**: any `spring.bigcache.instances.*` key. Each `spring.bigcache.instances.<name>` entry creates one
`*StarterBigCache.Cache` bean named `<name>`, plus a health indicator named `bigcache:<name>`
(controlled by `health`, default true).
Pure in-process cache — no external dependency, no address, no pool.

---

## 1. Complete worked project

Two instances with different retention plus the cache façade. File tree:

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
    github.com/allegro/bigcache/v3 latest
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-bigcache latest
    go-spring.org/starter-actuator latest   // optional
    go-spring.org/starter-otel     latest   // optional: metric/span export
    go-spring.org/starter-governance-file latest // optional
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-bigcache"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go**:

```go
package service

import (
    "context"

    "go-spring.org/cloud/cache"
    "go-spring.org/spring/gs"
    StarterBigCache "go-spring.org/starter-bigcache"
)

type Service struct {
    // Inject the WRAPPER *StarterBigCache.Cache, not *bigcache.BigCache:
    // bigcache offers no hook/plugin point, so each operation's identity is
    // declared by the wrapper's Get/Set/Delete [command.go:63-73] and emitted
    // by the resilience layer it runs under. The raw *bigcache.BigCache is
    // unexported with no accessor; the remaining raw methods
    // (Stats/Len/Capacity/Reset/Close/...) are plain delegations.
    Hot  *StarterBigCache.Cache `autowire:"hot"`
    Cold *StarterBigCache.Cache `autowire:"cold"`

    // Typed façade over "hot" — the *cache.Cache bean named "bigcache:hot".
    Cache *cache.Cache `autowire:"bigcache:hot"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            _ = s.Hot.Set("key", []byte("value"))
            v, _ := s.Hot.Get("key")
            _ = s.Cache.Set(ctx, "user:1", "demo", 0)
            _ = v // use value
        }
    })
}
```

**conf/app.properties** — mirrors the example's hot/cold split:

```properties
# --- hot: short retention, stats on -----------------------------------------
spring.bigcache.instances.hot.life-window=1m
spring.bigcache.instances.hot.shards=256
spring.bigcache.instances.hot.stats-enabled=true

# --- cold: long retention -----------------------------------------------------
spring.bigcache.instances.cold.life-window=30m
spring.bigcache.instances.cold.shards=1024

# --- actuator + otel ----------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.metrics.exporter=prometheus
```

**Verify**:

```bash
go run .                                 # no external dependency at all
curl -s :9370/readyz                     # components include bigcache:hot (always UP, §4.1)
curl -s :9370/metrics | grep bigcache    # hits/misses/entries/capacity gauges
grep _app_bigcache_access app.log | tail # one record per Get/Set/Delete
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-bigcache
  └─ gs.Module(OnProperty("spring.bigcache")) fires when any spring.bigcache.instances.* key exists
        └─ conf.BindEach("${spring.bigcache}") → one Config per <name>
              ├─ Provide(newClient, name@1, c@2).Name(<name>)
              │    .Destroy((*Cache).Destroy)
              └─ unless health=false → Provide health.Indicator named "bigcache:<name>"

gs.Run()
  ├─ ctor newClient [starter.go:88]: driver lookup → DefaultDriver.CreateClient(ctx, name, c, params)
  │   (params = cloud.ClientParams{Resilience: mgr, Fault: inj}, bundled by the ctor from the
  │    injected *resilience.Manager / *fault.Injector beans)
  │   (bigcache.DefaultConfig(LifeWindow) + knobs → bigcache.New → NewCache(name, client, params))
  │   → NewCache [client.go:108-131]: register this cache's observable gauges AND build the
  │     resilience executor — the cache is complete here, nothing patches it afterwards
  │     — OTel observable gauges [observe.go:99-107, 159-171]
  │     — service label "bigcache:<name>" → params.ExecutorFor("bigcache", service)
  │       (= resilience executor for mgr + fault wrapper for inj; the executor emits each
  │        declared operation's span/metrics/access log; zero params → resilience.Unmanaged,
  │        observed-only with a one-time warning)
  │   NOTE: no connectivity probe — there is nothing to probe (in-process heap)
  ├─ your Runner uses Get/Set/Delete (each = declared operation + executor; emitted there)
  └─ SIGTERM → Destroy [client.go:136-145]: unregister gauges → exec.Close → client.Close
      (stops the background eviction goroutine — hence the mandatory destroy)
```

### 2.2 Command surface — hand-written, because bigcache has no hook point

Unlike go-redis (hooks) or redigo (interceptor chain), bigcache exposes no extension seam, so
the starter hand-writes Get/Set/Delete on the wrapper, each routed through the same two layers
[command.go:47-73]:

```
declare operation (run/runErr) → resilience executor (guard = emission point) → bigcache core
```

- **`run` / `runErr`** [command.go:47-60]: declares the operation's identity on the context and
  threads it through the executor; `bigcache.ErrEntryNotFound` (a cache miss) is declared with
  `resilience.Tolerate`, so **a miss never trips the breaker**, mirroring `redis.Nil` /
  `gorm.ErrRecordNotFound` elsewhere in the family, while still being returned to the caller
  verbatim. A rejection (rate-limited / circuit-open) surfaces as the executor's error.
- **emission**: the starter emits nothing itself — it only DECLARES the operation. The
  resilience recorder it runs under emits the span, the metrics and the access log. bigcache is
  in-process with no network, so spans are ROOT spans (no caller context to link) and durations
  are sub-microsecond — the value is per-key access visibility and a uniform signal vocabulary
  across client starters [client.go:39-58]. Operations are named `get` / `set` / `delete` with
  the key as `db.statement` (span and log only — never a metric label).

### 2.3 One operation through the layers: `Get("key")` on a miss

1. `run` declares the operation (`get`, key `key`) and puts it on the context [command.go:47].
2. The resilience executor opens the call span (`get`) and asks for a permit
   (service `bigcache:<name>` — per instance).
3. bigcache returns `ErrEntryNotFound`; `Tolerate` classifies it as success (nil to the executor).
4. The executor emits the span, the duration metrics and the access log, then the original error
   is returned to the caller verbatim [command.go:47-60]; the log records the miss as the op's
   outcome without feeding the breaker.

---

## 3. Per-key behavior reference

All keys live under `spring.bigcache.instances.<name>.`.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `shards` | int | 1024 | Shard count; bigcache requires a power of two. | Non-power-of-two → bigcache.New error at boot. |
| `life-window` | duration | 10m | Max entry age before staleness. | Short → churn; long → stale reads. |
| `clean-window` | duration | 1m | Background eviction interval; 0 disables the cleaner goroutine. | 0 → stale entries linger until memory pressure. |
| `max-entries-in-window` | int | 600000 | Pre-allocation hint only — no runtime cap. | Under-guessed → realloc churn at startup. |
| `max-entry-size` | int | 500 | Pre-allocation hint for one entry (bytes). | Under-guessed → realloc churn. |
| `hard-max-cache-size` | int | 0 | Hard memory cap in MB; 0 = unlimited. | Set without need → early eviction (oldest entries dropped). |
| `stats-enabled` | bool | false | bigcache per-key hit/miss stats. ⚠ The starter's OTel gauges read `Stats()` — several show 0 unless this is on (they pull whatever Stats() returns [observe.go:99-107]). | Off → gauges read zero while Len/Capacity still work. |
| `health` | bool | true | Contributes the `bigcache:<name>` health.Indicator (a constant-UP placeholder — an in-process heap cache has no reachability to check); false keeps the instance out of aggregate health. | false → no indicator bean; the instance is absent from the health report. |

The per-instance `driver` key names the Driver bean: empty = inject the single Driver bean by
type (or fall back to the bundled `DefaultDriver` when none is provided); set to a bean name to
select one explicitly — naming a missing bean fails startup.

**Per-operation metrics** (emitted by the resilience layer the declared operation runs under,
meter `go-spring.org/cloud/resilience`): call-level `db.client.operation.duration`
(retries and backoff included), attempt-level `db.client.attempt.duration` (one record per
downstream try — what the cache itself cost), in-flight `db.client.active_requests`, and the
status-classified `resilience.client.calls`. Labels are `db.system=bigcache`,
`db.operation=<get|set|delete>` and `status`; the key rides `db.statement` on the span and log
only, never as a label. The access log is tag `_app_bigcache_access`, the same tag the starter
registers [observe.go:44].

**Cache-statistics gauges** (owner of this starter's own meter, `go-spring.org/starter-bigcache`,
attribute `cache.name=<name>`): observable gauges `bigcache.hits`, `bigcache.misses`,
`bigcache.delete_hits`, `bigcache.delete_misses`, `bigcache.collisions`, `bigcache.entries`,
`bigcache.capacity` [observe.go:99-107]. All are gauges (not counters) because `ResetStats()` can
break monotonicity. No per-operation cost — values are pulled on scrape via callbacks. This is
the one signal the starter still owns: it is per-process, not per-call, so the resilience layer
cannot emit it.

**Cache abstraction bean**: alongside the wrapper, each instance is provided as a
`*cache.Cache` bean named `bigcache:<bigcache-instance-name>` [starter.go:68-71] — inject
`*cache.Cache` with the autowire tag `bigcache:<instance-name>`. Un-injected, the bean never
instantiates, so there is no config switch to set. `ErrEntryNotFound` maps to `cache.ErrMiss`
at this boundary (this package's bytecache.go).

**Assembly extension point**: cache assembly is owned by a `Driver` interface [driver.go:32-58].
A company/umbrella starter may provide its own `Driver` as an **optional container bean**
(`gs.Provide(func() StarterBigCache.Driver{...})`), whose constructor returns the interface and
so may inject config bound from the properties file at wiring time; every cache instance is then
built through it. When no such bean exists the starter falls back to the bundled `DefaultDriver`
inside assembly [driver.go:67-84]. When several Driver beans coexist, an entry selects one by name:
`spring.bigcache.instances.<name>.driver = <bean-name>` (empty = fall back to the family-wide `spring.<family>.default.driver`, then to the single Driver bean by type;
naming a missing bean fails startup).

---

## 4. Verification & fault drills

### 4.1 Health via actuator

```bash
curl -s :9370/readyz    # bigcache:hot / bigcache:cold present
```

The indicator always reports UP [`health.go`]: an in-process heap cache has no
connectivity to probe — the instance existing means it is ready. Its value is confirming the
instance is registered and folded into readiness.

### 4.2 Observe access log + stats gauges

```bash
# generate traffic via your Runner, then:
curl -s :9370/metrics | grep 'bigcache_'
# bigcache_hits{cache.name="hot"} grows only with stats-enabled=true
grep _app_bigcache_access app.log | tail -3   # op=get/set/delete + key (debug level)
```

### 4.3 Eviction drill (example's "evict" instance)

```properties
spring.bigcache.instances.evict.shards=2
spring.bigcache.instances.evict.life-window=1m
spring.bigcache.instances.evict.hard-max-cache-size=1
spring.bigcache.instances.evict.max-entry-size=1024
```

Write past the 1MB cap: oldest entries are evicted; `bigcache.entries` plateaus at capacity and
`bigcache.misses` climbs for evicted keys — the example asserts exactly this shape.

### 4.4 Cache abstraction wiring (SET via façade, GET via wrapper)

```go
_ = s.Cache.Set(ctx, "k", "v", time.Minute)  // JSON-encoded bytes through the *cache.Cache façade
raw, _ := s.Hot.Get("k")                     // the same JSON bytes, read through the wrapper
```

A façade miss returns `cache.ErrMiss`; the wrapper returns `bigcache.ErrEntryNotFound`. The raw
`*bigcache.BigCache` is not reachable — only the wrapper (which declares every access's
operation) is.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|------|--------------|-----|
| Boot error from `bigcache.New` (shards) | `shards` not a power of two | Fix the value. |
| Gauges all zero | `stats-enabled=false` | Turn it on for instances you watch. |
| No access log | level unset + tag filtered, or `off` | Set `detailed`; check logger config for `_app_bigcache_access`. |
| Values truncated / realloc churn | `max-entry-size` under-guessed | It is a pre-allocation hint — size to real entries. |
| Entries disappear early | `hard-max-cache-size` cap evicting oldest | Raise or remove the cap. |
| Health shows component but "probe seems fake" | By design — always UP [`health.go`] | Watch gauges/log instead for real signal. |
| Stale values served | `clean-window=0` disabled the cleaner | Set a non-zero clean-window. |
| Breaker trips on every miss | It does not — ErrEntryNotFound is tolerated [command.go:47-51] | Look for a real failure. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 7 instance keys |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 4 |

Design suspects (audit ledger): `stats-enabled` defaults false yet the starter's headline
metrics depend on it (silent zeros); health indicator is a constant-UP placeholder (cost: one
bean; value: registration confirmability — candidate for deletion per "delete > privatize");
example conf comments mention an OnRemove demo the DefaultDriver never wires.

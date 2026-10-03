# starter-bigcache Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). Every behavior claim below is
verified against this starter's source (`starter.go`, `config.go`, `client.go`, `driver.go`,
`bytecache.go`, `observe.go`) and the self-asserting [example/](example/) — spot-checks are
given as `file:line`. **BigCache semantics — sharding, life-window eviction, the hard size
cap, byte-only values — are [the official README](https://github.com/allegro/bigcache)**;
everything below is go-spring's increment: the wiring, the config binding, the signal
vocabulary.

**Activation**: any `spring.bigcache.instances.*` key ([starter.go:34](starter.go#L34)). Each
`spring.bigcache.instances.<name>` entry creates one `*StarterBigCache.Cache` bean named
`<name>`. This is a purely in-process cache: no external dependency, no address, no pool, no
health indicator — there is no reachability to probe.

---

## 1. Complete worked project

The runnable project is [example/](example/); this is what it contains, in order.

### 1.1 Dependency

```bash
go get go-spring.org/starter-bigcache
```

**Prerequisite external systems: none.** The example is self-contained; `example/check.sh`
runs it and exits non-zero if any assertion fails.

### 1.2 Configuration

Copy [example/conf/app.properties](example/conf/app.properties) — three instances, each
showing a different shape:

```properties
# The HTTP handlers are served by gs's built-in server, whose address is declared
# here rather than left to the framework default.
spring.http.server.addr=127.0.0.1:9090

# Family-wide defaults: every instance inherits keys it does not set itself.
spring.bigcache.default.shards=1024
spring.bigcache.default.life-window=10m
spring.bigcache.default.stats-enabled=true
spring.bigcache.default.driver=hook

# hot overrides only its retention; shards and stats-enabled are inherited.
spring.bigcache.instances.hot.life-window=1m

# cold: a different life-window is the whole reason for a second instance.
spring.bigcache.instances.cold.life-window=30m

# cleaned / uncleaned: the same 1s life-window, differing only in clean-window -
# they pin down that a stale entry is still served unless a cleaner removes it.
spring.bigcache.instances.cleaned.life-window=1s
spring.bigcache.instances.cleaned.clean-window=100ms
spring.bigcache.instances.uncleaned.life-window=1s
spring.bigcache.instances.uncleaned.clean-window=0

# evict: tiny and hard-capped, and the one instance that needs the hooking
# driver - filling it past 1MB forces evictions.
spring.bigcache.instances.evict.driver=hook
spring.bigcache.instances.evict.shards=2
spring.bigcache.instances.evict.life-window=1m
spring.bigcache.instances.evict.clean-window=0
spring.bigcache.instances.evict.max-entries-in-window=4096
spring.bigcache.instances.evict.max-entry-size=1024
spring.bigcache.instances.evict.hard-max-cache-size=1
```

Every key not set for an instance falls back to the family-wide
`spring.bigcache.default.<key>` bucket ([starter.go:38](starter.go#L38)), so the common part
of several instances is written once.

**⚠ An instance must have at least one key of its own.** Activation is the presence of a
`spring.bigcache.instances.*` key, and each instance is one child of that map — a name with
everything inherited has no child, so no bean is created for it. Inheriting is for the *values*,
not for declaring that an instance exists.

### 1.3 Code

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/cache"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
	_ "go-spring.org/starter-bigcache"
)

// Inject the wrapper, one field per configured instance, keyed by bean name.
// The raw *bigcache.BigCache is not injectable: it lives in an unexported field
// of the wrapper (see README "Design Notes").
type Service struct {
	Hot   *StarterBigCache.Cache `autowire:"hot"`
	Cold  *StarterBigCache.Cache `autowire:"cold"`
	Evict *StarterBigCache.Cache `autowire:"evict"`

	// The same `hot` instance through the cache abstraction, as the bean
	// "bigcache:hot" (§3.1) — a second face on one cache, not a second cache.
	Cache *cache.Cache `autowire:"bigcache:hot"`
}

func main() {
	// Service is not referenced by any other bean, so it must be exported as a
	// root object or the container would not instantiate it.
	svrBean := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		v, err := s.Hot.Get(context.Background(), "key")
		if err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write(v)
	})
	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		s := svrBean.Interface().(*Service)
		if err := s.Hot.Set(context.Background(), "key", []byte("value")); err != nil {
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write([]byte("OK"))
	})

	go func() { runTest(svrBean.Interface().(*Service)) }()
	gs.Run()
}

func runTest(s *Service) {
	ctx := context.Background()

	// SET/GET round-trip.
	_ = s.Hot.Set(context.Background(), "key", []byte("value"))
	v, _ := s.Hot.Get(context.Background(), "key")
	fmt.Println("get:", string(v))

	// DELETE, then a miss: bigcache reports absence as ErrEntryNotFound.
	_ = s.Hot.Delete(context.Background(), "key")
	_, err := s.Hot.Get(context.Background(), "key")
	fmt.Println("miss is ErrEntryNotFound:", errors.Is(err, bigcache.ErrEntryNotFound))

	// Instances are independent: a key in `cold` is invisible through `hot`.
	_ = s.Cold.Set(context.Background(), "only-cold", []byte("cold-value"))
	_, err = s.Hot.Get(context.Background(), "only-cold")
	fmt.Println("hot does not see cold:", errors.Is(err, bigcache.ErrEntryNotFound))

	// stats-enabled (on by default) makes the hit/miss counters readable here —
	// and exported as OTel gauges (§4.1).
	st := s.Hot.Stats()
	fmt.Println("hot stats:", st.Hits, st.Misses)

	// Through the cache abstraction: JSON-encoded, misses as cache.ErrMiss. The
	// per-call TTL is ignored (§3), so the value outlives the 1s it was given.
	_ = s.Cache.Set(ctx, "abstraction", "value", 1)
	time.Sleep(1100 * time.Millisecond)
	var av string
	fmt.Println("still there:", s.Cache.Get(ctx, "abstraction", &av) == nil, av)

	// Overflow the hard-capped `evict` cache (§4.2).
	big := make([]byte, 900)
	for i := range 4000 {
		_ = s.Evict.Set(fmt.Sprintf("k-%d", i), big)
	}
	log.Infof(ctx, log.TagAppDef, "done")
}
```

The full file, including the manual-verification flag, the HTTP round-trip assertion and the
graceful shutdown, is [example/main.go](example/main.go); the custom `Driver` it registers lives
in [example/driver.go](example/driver.go) (§3.2).

### 1.4 Run and verify

```bash
cd example && ./check.sh      # runs the example; exits non-zero on any failed assertion
```

To poke it by hand, run with `-manual` and use the two handlers:

```bash
go run . -manual
curl http://127.0.0.1:9090/set    # OK
curl http://127.0.0.1:9090/get    # value
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import _ "go-spring.org/starter-bigcache"
  └─ gs.Module(OnProperty("spring.bigcache.instances"))        [starter.go:34]
     fires only when at least one spring.bigcache.instances.* key exists
        └─ flatten.WithFallback(instances, default)            [starter.go:38]
              per-instance keys fall back to spring.bigcache.default.<k>
              └─ conf.BindEach("${spring.bigcache.instances}") [starter.go:39]
                 one Config bound per <name>, driven by the map keys
                    └─ r.Provide(newClient, name@1, c@2, driver@3).Name(name)
                       .Destroy((*Cache).Destroy)              [starter.go:48]
                       index 0 (*gs.ContextProvider) is autowired; the Driver
                       param is selected by ${...<name>.driver}

gs.Run()
  ├─ ctor newClient                                            [starter.go:72]
  │    d == nil → DefaultDriver{} (no company Driver bean registered)
  │    → DefaultDriver.CreateClient(ctx, name, c)              [driver.go:81]
  │        bigcache.DefaultConfig(LifeWindow) + the six other knobs
  │        → bigcache.New(ctx, conf)          ← the only step that can fail on
  │          the raw cache (e.g. shards not a power of two)
  │        → NewCache(client, name)                          [client.go:78]
  │             → newStatObserver(client, name)              [observe.go:229]
  │                  → buildInstruments()                    [observe.go:170]
  │                    process-wide, resolved ONCE per process via
  │                    singleton.Singleton — the first cache built binds the
  │                    instrument set for every later one
  │                  → statObserver.observeGauges(client)    [observe.go:251]
  │                    registers THIS cache's statistics against the shared
  │                    gauges, labelled instance=<name>
  │    The bean is complete when the ctor returns: no Init step, nothing
  │    patches the cache afterwards.
  ├─ (optional) *cache.Cache bean "bigcache:<name>"           [starter.go:61]
  │    lazy — un-injected, it never instantiates
  ├─ readiness: the app is up once every root-reachable bean is built
  └─ shutdown: (*Cache).Destroy                              [client.go:89]
       obs.close() → reg.Unregister()  (gauges stop reporting this cache)
       client.Close() → stops the background eviction goroutine
```

Two ordering facts worth knowing:

- **The destroy order is load-bearing.** `Unregister` comes first: once the cache is closed
  its statistics are meaningless, and a registration left behind would report a dead cache
  *and* pin it in memory ([client.go:89](client.go#L89)).
- **⚠ `Close()` is not idempotent.** bigcache closes a channel internally, so calling `Close()`
  and then `Destroy()` — or either one twice — panics. `Destroy` is the normal path; use
  `Close()` only when you own the cache outside the container.

### 2.2 The command surface — hand-written, because bigcache has no hook point

bigcache (unlike go-redis or gorm) exposes no plugin or interceptor seam, so per-operation
observability can only come from holding the wrapper
([client.go:48-50](client.go#L48-L50)). That decides the shape of the public type:

- `Get`/`Set`/`Delete` are re-implemented and observed — the three operations that carry
  business traffic ([client.go:111-135](client.go#L111-L135)).
- Every other raw method (`Stats`, `Len`, `Capacity`, `Reset`, `Close`, `KeyMetadata`,
  `Iterator`) is re-exposed as a plain pass-through delegation, deliberately unobserved.
- The raw `*bigcache.BigCache` is an unexported field with **no accessor**, so a caller cannot
  bypass the observation layer.

No executor sits under these calls: nothing here reaches out of the process, so there is no
protection to apply and no access log worth writing. The starter consequently takes no
governance bean and no `cloud.ClientParams` — a seam with nothing behind it.

### 2.3 One `Get("key")` on a miss, layer by layer

```
c.Get(ctx, "key")                                                     [client.go:115]
      → obs.observe(ctx, "get", fn)            [observe.go:267]
          run fn → c.client.Get("key") → bigcache.ErrEntryNotFound
          statusOf(err) → "ok"                 [observe.go:259]
              a miss is NOT a failure: the cache answered, the key was absent
          counter  bigcache.operation.total   {operation="get",status="ok",instance="hot"} += 1
  returns bigcache.ErrEntryNotFound verbatim to the caller
```

There is no span: trace topology is made of edges, and a microsecond in-process call adds none.
The key never enters the metric either — cache keys come from an open set, and one as a label
would multiply the series without bound
([observe.go:267-282](observe.go#L267-L282)).

---

## 3. Per-key behavior reference

All keys live under `spring.bigcache.instances.<name>.` ([config.go:26](config.go#L26)).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `shards` | int | 1024 | Shard count; bigcache requires a power of two. | Non-power-of-two → `bigcache.New` fails at startup, naming the instance. |
| `life-window` | duration | 10m | How long an entry may live before it is **stale**. One value for the whole instance — not per entry. Truncated to whole seconds internally, and staleness is a strict `>`: an entry goes stale after more than `life-window` whole seconds. **A stale entry is still served** — see the ⚠ below. | Short → churn; long → stale reads. |
| `clean-window` | duration | 1m | How often the background cleaner removes stale entries. This — not `life-window` — decides when a value stops being readable. 0 disables the cleaner goroutine. | 0 → stale entries are served until capacity eviction removes them. |
| `max-entries-in-window` | int | 600000 | Pre-allocation hint only — no runtime cap. | Under-guessed → realloc churn at startup. |
| `max-entry-size` | int | 500 | Pre-allocation hint for one entry, in bytes. | Under-guessed → realloc churn. |
| `hard-max-cache-size` | int | 0 | Hard memory cap in MB; 0 = unlimited. | Set without need → early eviction (oldest entries dropped). |
| `stats-enabled` | bool | **true** | Records per-key hit/miss/collision counters, readable via `Stats()`/`KeyMetadata()` and exported as the five `bigcache.hits`… gauges. **On by default, unlike bigcache's own `DefaultConfig`** — the starter's headline gauges read exactly this ([config.go:48-54](config.go#L48-L54)). | Off → those gauges are exported as a constant **zero**, not absent: a dashboard showing no hits would then mean the counter is off, not that the cache is cold. Off also drops the per-key bookkeeping bigcache keeps while it is on, which is the only reason to turn it off. |

**Driver selection** — `spring.bigcache.instances.<name>.driver`
([starter.go:51](starter.go#L51)): empty = the single `Driver` bean by type, or the bundled
`DefaultDriver` when none is registered; set = that bean name, and naming a missing bean fails
startup. The family-wide fallback is `spring.bigcache.default.driver`.

**⚠ `life-window` is not a read-side TTL.** It marks an entry stale; it does not hide it. `Get`
returns an entry whatever its age, and removal is the cleaner's job — every `clean-window`. With
`clean-window=0` a value therefore outlives its `life-window` indefinitely, until capacity
eviction takes it. `example/` asserts both halves of this: the same 1s `life-window` under two
cleaner settings, one entry gone and one still served.

**⚠ One TTL for the whole instance.** BigCache expires by the single global `life-window`;
there is no per-entry TTL. Reached through the cache façade, `SetBytes`' `ttlSeconds` argument
is **ignored** on purpose ([bytecache.go:55-60](bytecache.go#L55-L60)) — the signature is the
shared `cache.ByteCache` contract, and BigCache has nothing to honour it with. Need several TTL
classes → define several named instances.

### 3.1 Beans

| Bean | Type | Name | How to inject |
|------|------|------|---------------|
| client | `*StarterBigCache.Cache` | `<name>` | `autowire:"<name>"` |
| cache abstraction | `*cache.Cache` | `bigcache:<name>` | `autowire:"bigcache:<name>"` |

The `*cache.Cache` bean is lazy: un-injected it never instantiates, so there is no config
switch for it ([starter.go:61-65](starter.go#L61-L65)). At that boundary
`bigcache.ErrEntryNotFound` maps to `cache.ErrMiss`, and `Delete` of an absent key is not an
error ([bytecache.go:44-69](bytecache.go#L44-L69)).

### 3.2 Extension point

Cache assembly is owned by the `Driver` interface
([driver.go:68](driver.go#L68)):

```go
type Driver interface {
	CreateClient(ctx context.Context, name string, c Config) (*Cache, error)
}
```

Provide your own as an **optional container bean** — its constructor may inject config bound
from the properties file at wiring time:

```go
gs.Provide(func(c *MyConf) StarterBigCache.Driver { return myDriver{c} })
```

This is the only way to reach `bigcache.Config` fields the starter does not bind, such as
`OnRemove`. [example/driver.go](example/driver.go) registers one such Driver bean, named `hook`
and selected by `spring.bigcache.default.driver=hook`; it owns the whole assembly and wires
`OnRemove`, which is what lets `example/` assert the hook actually fires.

A driver **attributes its own construction failures**: the error it returns reaches the
container unwrapped, so it should name the stage and the instance, e.g.
`errutil.Explain(err, "bigcache: create instance %q", name)` — see
[DefaultDriver](driver.go#L81) for the pattern.

---

## 4. Verification & fault drills

### 4.1 Observe the per-operation signals and the statistics gauges

[example-otel/](example-otel/) wires starter-otel with the in-process Prometheus exporter
(and disables gs's default `:9090` HTTP server so the exporter owns the port):

```properties
spring.http.server.enabled=false
spring.observability.enable=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
spring.observability.trace.exporter=none
```

```bash
cd example-otel && go run . -manual
# generate traffic, then:
curl -s :9090/metrics | grep 'bigcache_'
# bigcache_operation_total{operation="get",status="ok",instance="hot"} counts the calls;
# bigcache_hits{instance="hot"} grows only with stats-enabled=true
```

Reading the signals: the counter carries `operation=<get|set|delete>`,
`status=<ok|error>` and `instance=<name>`. A miss counts as `status="ok"` — hit rate is the
gauges' job, not the status axis'. Gauges are pulled on scrape (no per-call cost) and are
**per process**, not per call. The meter scope is `go-spring.org/starter-bigcache`
([observe.go:49](observe.go#L49)).

Note that `bigcache_operation_total` appears only **after** the first
operation of that kind: the gauges are observable (always reported), the counter
is an event (reported once something happened). An empty `grep` right after boot is not a
wiring fault — drive one `Get` first. `example-otel` asserts these series too, including that a
miss counts as `status="ok"` and that no cache key ever becomes a label.

### 4.2 Eviction drill (the example's `evict` instance)

With `hard-max-cache-size=1` (1 MB) and `max-entry-size=1024`, writing past the cap evicts the
oldest entries. Observable shape: `bigcache_entries{instance="evict"}` plateaus at capacity
while `bigcache_misses` climbs for the evicted keys. `example/` asserts it from the cache side:
resident entries below what was written, and the `OnRemove` hook having fired.

### 4.3 Cache-abstraction wiring, and the ignored TTL

```go
_ = s.Cache.Set(ctx, "k", "v", 1)       // through the *cache.Cache façade; 1 = ttlSeconds
var v string
err := s.Cache.Get(ctx, "k", &v)        // JSON-decoded into a pointer
if errors.Is(err, cache.ErrMiss) { /* absent */ }
```

The façade serializes with JSON and reports a miss as `cache.ErrMiss`; the wrapper reports
`bigcache.ErrEntryNotFound`. Note the TTL argument is a whole number of **seconds**
(`ttlSeconds int`), not a `time.Duration`.

To see the ignored TTL: set with `ttlSeconds=1`, sleep past a second, read again — the value is
still there, because expiry follows the instance's `life-window` and nothing else
([bytecache.go:58](bytecache.go#L58)). This is asserted by `example/main.go` Feature 5, and
by `bytecache_test.go` at the adapter level.

### 4.4 Startup-failure drill: a refusing meter

The instrument set is built once per process, on the first cache construction. If a custom
`MeterProvider` refuses an instrument, startup fails with the stage named rather than
panicking out of the library:

```
wire bean hot(<registration file:line>), err constructor returned error: bigcache: create gauge "bigcache.hits": <sdk error>
```

The raw cache built a moment earlier is closed on the way out, so a failed construction never
leaves the eviction goroutine behind ([driver.go:96-102](driver.go#L96-L102)). Note this is
the *only* way `NewCache` fails — the raw cache is already open and healthy when it is called.

### 4.5 Spans: none, deliberately

The wrapper opens no span. A trace is worth what its edges say, and an in-process microsecond
call has no edge — the counter (§2.3) and the gauges (§4.2) are the whole story. If a cache call
ever needs to appear in a trace, the call site — which knows the business context — is the place
to record it.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup fails: `bigcache: create instance "<name>": ...` | The raw cache could not be built — most often `shards` not a power of two | Fix the value for that instance. |
| Startup fails: `bigcache: create gauge "<metric>": ...` | A custom `MeterProvider` refused an instrument (process-wide) | Check the OTel setup; the named instrument is the one refused. |
| Gauges all zero while `Len()` works | `stats-enabled` was set to `false` | Turn it back on for instances you watch. |
| Gauges vanished after a shutdown/restart cycle | `Destroy` unregistered them and closed the cache | Expected — the registration's lifetime is the cache's. |
| Panic: "close of closed channel" | `Close()` called, then `Destroy()` — or either twice | Call `Destroy()` only; it is the container's path. |
| Values truncated / realloc churn | `max-entry-size` under-guessed | It is a pre-allocation hint — size it to real entries. |
| Entries disappear early | `hard-max-cache-size` cap evicting the oldest | Raise or remove the cap. |
| A value past its `life-window` is still returned | `life-window` does not hide entries, and nothing removed it — with `clean-window=0` nothing ever does | Set a non-zero `clean-window`, or treat the instance as having no read-side TTL. |
| Stale values served past the TTL you passed to `Set` | `life-window` is per instance; a per-call TTL is ignored | Size by `life-window`, or use separate instances. |
| No per-call line in the log | There is none — bigcache writes no access log and the starter emits none | Read the metrics instead. |
| Two replicas disagree | Entries are process-local heap; nothing is invalidated across replicas | Expected — use a networked backend for coherence. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 7 instance keys (+7 inherited from `spring.bigcache.default.*`) + 1 driver key |
| Required | 0 |
| Quickstart external deps | 0 |
| "Watch out" entries | 4 |

One deliberate deviation from the component's defaults: `stats-enabled` ships **on**, where
bigcache's own `DefaultConfig` leaves it off. §3 gives the reason — without it the starter's
headline gauges are silent zeros, and a default that blanks your own metrics is worse than one
that spends a per-key map. Turn it off where that bookkeeping outweighs the counters.

Design suspects (audit ledger):

- The cache façade's `ttlSeconds` parameter is accepted and discarded
  ([bytecache.go:58](bytecache.go#L58)); the shared interface cannot express BigCache's
  instance-wide TTL. It is now stated in three places (the adapter, the README, §3/§5 here) —
  the risk is not that a reader misses it, but that the three drift apart.

Removed: the per-instance `health` switch and its constant-UP indicator — an in-process heap
cache has no reachability to probe, so the indicator could never fail, and a constant-UP term
in an AND-aggregated readiness carries no signal. bigcache is the recorded exception to the
"health on by default" rule for client starters; it contributes no `health.Indicator` and no
`ping` probe.

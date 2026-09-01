# starter-cache Usage — Reference

Detailed usage reference. For the module map see [starter/README](../README.md). All behavior claims are
verified against the starter source (`starter.go`, `starter_test.go`), the cache abstraction
(`cloud/cache/cache.go`, `codec.go`), and the four backend driver registrations
(`starter-go-redis/starter.go`, `starter-redigo/starter.go`, `starter-bigcache/starter.go`,
`starter-memcached/starter.go`). **Cache semantics themselves (miss vs error, codec, TTL) are
documented in `cloud/cache`** — everything below is the go-spring wiring increment: the driver
registry, the `${spring.cache}` module, and the bean it exposes.

**Activation**: any `spring.cache.*` key — the module is `gs.OnProperty("spring.cache")`, a prefix
check (`starter.go:42`). The starter itself is inert: importing only `starter-cache` registers no
drivers; importing a backend starter (go-redis / redigo / bigcache / memcached) is what registers a
driver via `init()` (e.g. `starter-go-redis/starter.go:81`). A `driver` value naming an unregistered
backend fails at startup with the sorted list of registered names (`starter.go:101-113`).

**No standalone example exists yet** — the worked project below mirrors the verified backend
examples (`starter-go-redis/example`, `starter-bigcache/example`, both run by their `check.sh`) plus
this module's `starter_test.go`; it has not been smoke-run as a standalone `spring.cache` example
(suspect #1 in §6).

---

## 1. Complete worked project

A service exposing two caches through the façade: a Redis-backed shared cache and an in-process
bigcache hot tier. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**Prerequisite** (single external dependency, Redis; bigcache is in-process):

```bash
docker run -d --name redis -p 127.0.0.1:6379:6379 redis:7
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring           v1.3.x
    go-spring.org/starter-cache    latest
    go-spring.org/starter-go-redis latest   // registers the "go-redis" driver
    go-spring.org/starter-bigcache latest   // registers the "bigcache" driver
)
```

**main.go**:

```go
package main

import (
    _ "demo/conf"
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-bigcache"
    _ "go-spring.org/starter-cache"
    _ "go-spring.org/starter-go-redis"
)

func main() { gs.Run() }
```

**service.go** — inject the façade beans and serve a Set/Get pair per cache:

```go
package conf

import (
    "errors"
    "net/http"
    "time"

    "go-spring.org/cloud/cache"
    "go-spring.org/spring/gs"
)

type Service struct {
    // ⚠ The bean name is the BACKEND instance name (beanID), not the spring.cache
    // entry name — see §2.2. Here entry "shared" uses driver go-redis:shared, so
    // the *cache.Cache bean is named "shared" only because we chose to reuse the
    // same word; with driver go-redis:main the bean would be "main".
    Shared *cache.Cache `autowire:"shared"`
    Hot    *cache.Cache `autowire:"hot"`
}

func init() {
    b := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()) // root: nothing else injects it

    http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
        s := b.Interface().(*Service)
        key, val := r.URL.Query().Get("k"), r.URL.Query().Get("v")
        // Typed Set: JSON codec, 60s TTL.
        if err := s.Shared.Set(r.Context(), key, val, 60*time.Second); err != nil {
            http.Error(w, err.Error(), 500)
            return
        }
        _ = s.Hot.Set(r.Context(), key, val, 30*time.Second)
        _, _ = w.Write([]byte("ok"))
    })
    http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
        s := b.Interface().(*Service)
        var v string
        if err := s.Shared.Get(r.Context(), r.URL.Query().Get("k"), &v); err != nil {
            if errors.Is(err, cache.ErrMiss) {
                http.Error(w, "miss", 404)
                return
            }
            http.Error(w, err.Error(), 500)
            return
        }
        _, _ = w.Write([]byte(v))
    })
}
```

**conf/app.properties** (backend blocks verbatim from `starter-go-redis/example/conf/app.properties`
and `starter-bigcache/example/conf/app.properties`, trimmed to one instance each):

```properties
# --- backend: go-redis instance "shared" -----------------------------------
spring.go-redis.shared.addr=127.0.0.1:6379

# --- backend: bigcache instance "hot" --------------------------------------
spring.bigcache.hot.life-window=1m
spring.bigcache.hot.shards=256
spring.bigcache.hot.stats-enabled=true

# --- cache façade entries ---------------------------------------------------
# driver format "<driver>:<beanID>" — beanID names the BACKEND bean to wrap.
# The exposed *cache.Cache bean is named "shared"/"hot" (the beanID), NOT the
# entry name; here entry name == beanID by choice.
spring.cache.shared.driver=go-redis:shared
spring.cache.hot.driver=bigcache:hot
```

**Verify** (mirrors the backend examples' manual mode):

```bash
curl -i 'localhost:8080/set?k=user:1&v=alice'          # ok
curl -i 'localhost:8080/get?k=user:1'                  # alice
curl -i 'localhost:8080/get?k=user:2'                  # 404 miss
redis-cli -h 127.0.0.1 GET 'user:1'                    # "\"alice\"" — JSON codec, shared storage
sleep 61 && curl -i 'localhost:8080/get?k=user:1'      # 404 (60s TTL elapsed)
```

The `redis-cli` round-trip proves the value went through the app into the backing Redis under the
plain key (no prefixing is added by the façade — key namespacing is the caller's job).

---

## 2. Assembly & timing

### 2.1 Lifecycle timeline

```
import starter-go-redis / starter-bigcache / ...
  └─ backend init(): StarterCache.RegisterDriver("<name>", driver)     [init panics on duplicate]
import starter-cache
  └─ init(): gs.Module(gs.OnProperty("spring.cache"), ...) registered  (starter.go:41-64)
gs.Run()
  ├─ module fires (any spring.cache.* key present):
  │    conf.Bind(p, &map[string]{Driver}, "${spring.cache}")           (starter.go:46)
  │    for each entry (map order — nondeterministic, order is irrelevant):
  │      strings.Cut(driver, ":") → (driverName, beanID)               (starter.go:50)
  │      GetDriver(driverName) — unknown → error listing registered    (starter.go:54,101-113)
  │      d(beanID)(r, p) → backend's ModuleFunc:
  │        r.Provide(func(c *Client) *cache.Cache {
  │            return cache.New(bytecache.NewByteCache(...))
  │        }, gs.TagArg(beanID)).Name(beanID)                          (starter-go-redis/starter.go:82-85)
  ├─ bean wiring: the cache ctor autowires the backend wrapper by name (TagArg);
  │    backend bean's Init has armed resilience/observe before use
  ├─ Run: nothing — no server, no readiness of its own
  └─ shutdown: nothing — cache.Cache has no Close; the BACKEND bean's
       Destroy (go-redis Client.Destroy, bigcache Cache.Destroy) releases resources
```

Multiple cache entries each produce one `*cache.Cache` bean. Validation: malformed `driver` (no
`:`, empty half) fails the module with `cache: invalid driver %q (want "<driver>:<beanID>")`
(`starter.go:51-53`); an unknown driver name fails with the registered list (`starter.go:112`).

### 2.2 Driver resolution — the `<driver>:<beanID>` mechanics

`spring.cache.users.driver=go-redis:main` resolves in three steps:

1. **Prefix**: `"go-redis"` selects the registry entry — i.e. which backend starter's adapter runs.
   The registry is populated at import time by each backend starter's `init()`
   (`starter-go-redis/starter.go:81`, `starter-redigo/starter.go:85`,
   `starter-bigcache/starter.go:69`, `starter-memcached/starter.go:67`).
2. **beanID**: `"main"` is passed to the driver and used twice inside its ModuleFunc — as the
   `gs.TagArg(beanID)` (inject the backend wrapper bean named `main`, e.g. the
   `spring.go-redis.main.*` instance) and as the `.Name(beanID)` of the new `*cache.Cache` bean.
3. **Entry name is discarded**: the map key `users` is only the grouping slot in the config; it
   never reaches the bean name (the bind target is `map[string]struct{Driver}`, `starter.go:43-45`).

⚠ **Consequence (suspect #2, §6)**: the exposed bean is named after the *backend bean*, not the
cache entry. Two entries wrapping the same backend — `users.driver=go-redis:main` and
`sessions.driver=go-redis:main` — both try to provide a `*cache.Cache` named `main`: duplicate bean
error at wiring. Conversely `users.driver=go-redis:users` produces a bean named `users` that
happens to collide with the backend's own wrapper bean name only by (Name,Type) key — types differ
(`*Client` vs `*cache.Cache`), so they coexist. Always inject `*cache.Cache` by beanID.

### 2.3 One Set/Get call path

`Cache.Set(ctx, "user:1", "alice", 60s)` (typed) → `codecOr().Marshal(val)` (JSON by default,
`cache.go:123-129`) → `SetBytes` on the embedded `ByteCache` → the backend adapter (go-redis:
`bytecache.NewByteCache(c.UniversalClient)`, `starter-go-redis/starter.go:83`) issues the native
`SET key val PX 60000`. `Get` is the mirror: `GetBytes` → `(nil, cache.ErrMiss)` on a redis Nil
reply (`starter-go-redis/bytecache/bytecache.go:40-45`) → `codecOr().Unmarshal` into the pointer
(`cache.go:113-119`). Non-positive TTL means no expiry (`cache.go:57-59`).

### 2.4 The four drivers

| Driver name | Registered by | Wraps bean type | Backend prefix | Example entry |
|---|---|---|---|---|
| `go-redis` | `starter-go-redis` (`starter.go:81`) | `*StarterGoRedis.Client` (all modes: single/sentinel/cluster) | `spring.go-redis.<id>` | `spring.cache.c.driver=go-redis:c` + `spring.go-redis.c.addr=...` |
| `redigo` | `starter-redigo` (`starter.go:85`) | `*StarterRedigo.Pool` | `spring.redigo.<id>` | `spring.cache.c.driver=redigo:c` + `spring.redigo.c.addr=...` |
| `bigcache` | `starter-bigcache` (`starter.go:69`) | `*StarterBigCache.Cache` (embeds `*bigcache.BigCache`) | `spring.bigcache.<id>` | `spring.cache.hot.driver=bigcache:hot` + `spring.bigcache.hot.life-window=1m` |
| `memcached` | `starter-memcached` (`starter.go:67`) | `*StarterMemcached.Client` (embeds `*memcache.Client`) | `spring.memcached.<id>` | `spring.cache.c.driver=memcached:c` + `spring.memcached.c.servers=127.0.0.1:11211` |

All four adapters construct `cache.New(bytecache.NewByteCache(<client>))` with no `WithCodec` — the
façade codec is therefore always the default JSON; `WithCodec` is reachable only by constructing a
`cache.Cache` programmatically, not through config.

---

## 3. Per-key behavior reference

The façade itself has exactly one key per cache entry; the worked example's backend instances
contribute the rest. Verified against
`grep -rhoE 'value:"[^"]+"' <starter dirs> --include='*.go' | sort -u`.

### 3.1 Façade (`spring.cache.<name>.*`, `starter.go:43-45`)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.cache.<name>.driver` | string | — (required) | Format `"<driver>:<beanID>"`. Driver prefix selects the backend adapter; beanID selects the backend instance AND becomes the `*cache.Cache` bean name (§2.2). Any `spring.cache.*` key activates the module. | No `:` / empty half → startup error `cache: invalid driver` (`starter.go:51-53`). Unknown driver → error listing registered names (`starter.go:112`). beanID with no matching backend bean → wiring failure (unresolvable autowire). |

⚠ The `<name>` part is a pure grouping slot: it validates nothing and names nothing.

### 3.2 Backend instance keys referenced by the worked example

**`spring.go-redis.shared.*`** (all keys from `starter-go-redis/config.go:35-139`; full semantics in
starter-go-redis's USAGE):

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `addr` | string | `` | `host:port`; required unless `service-name` set |
| `mode` | string | `single` | `single`/`sentinel`/`cluster`; sentinel needs `master-name`+`sentinel-addrs`, cluster needs `addrs` |
| `password` / `username` / `db` | string/string/int | ``/``/0 | auth + logical db |
| `pool-size` / `max-idle` / `max-retries` | int | 10/5/0 | pool and retry tuning |
| `dial-timeout` / `read-timeout` / `write-timeout` / `conn-max-lifetime` | duration | 5s/3s/3s/2m | time budgets |
| `driver` | string | `DefaultDriver` | backend-internal client factory, unrelated to the cache driver |
| `tracing.enabled` / `metrics.enabled` | bool | true/true | redisotel hooks (no-op without starter-otel) |

**`spring.bigcache.hot.*`** (`starter-bigcache/config.go:28-52`; full list):

| Key | Type | Default | Notes |
|-----|------|---------|-------|
| `shards` | int | 1024 | shard count |
| `life-window` | duration | 10m | per-entry lifetime (works with §1's TTL test) |
| `clean-window` | duration | 1m | eviction sweep interval |
| `max-entries-in-window` / `max-entry-size` / `hard-max-cache-size` | int | 600000/500/0 | capacity shaping |
| `stats-enabled` | bool | false | exposes stats for the health/metrics path |
| `driver` | string | `DefaultDriver` | backend-internal factory |

(The redigo / memcached instance keys — `spring.redigo.<id>.*`, `spring.memcached.<id>.*` — follow
the same pattern; see `starter-redigo/config.go:29-106`, `starter-memcached/config.go:28-61`. Not
used by the worked example, so not expanded here.)

---

## 4. Verification & fault drills

1. **Round-trip through the app**: §1's curl SET/GET plus `redis-cli GET user:1` shows the JSON
   payload in the backing store under the bare key.
2. **Miss vs backend-down**: `GET` a missing key → HTTP 404 via `errors.Is(err, cache.ErrMiss)`
   (`cache.go:134`); stop redis (`docker stop redis`) and GET an *existing* key → HTTP 500 (backend
   error path, not a miss). This is the distinction the abstraction exists for.
3. **Bean-name mismatch drill**: change the entry to `spring.cache.users.driver=go-redis:main` and
   autowire `autowire:"users"` → container fails: no `*cache.Cache` bean named `users` (the bean is
   `main`, §2.2). Fix the tag to `main` or the beanID to `users`.
4. **Duplicate wrap drill**: add `spring.cache.sessions.driver=go-redis:main` alongside
   `users.driver=go-redis:main` → duplicate `*cache.Cache` bean `main` fails wiring. One façade bean
   per backend instance.
5. **Unknown driver drill**: `spring.cache.x.driver=redis:main` (typo, missing `go-`) → startup
   error `cache: no driver registered as "redis" (registered: [bigcache go-redis ...])`
   (`starter.go:112`) — the fix (import the starter or fix the prefix) is named in the error.
6. **Malformed driver drill**: `spring.cache.x.driver=go-redis` (no `:beanID`) → `cache: invalid
   driver "go-redis" (want "<driver>:<beanID>", e.g. "go-redis:main")` (`starter.go:52`).
7. **Multi-driver coexistence**: the worked project itself — go-redis and bigcache entries in one
   file, both beans injectable simultaneously; per-backend observability stays independent
   (`redis:shared` / `bigcache:hot` health indicators, `starter-go-redis/starter.go:58`,
   `starter-bigcache/starter.go:60`).
8. **TTL semantics**: the `sleep 61` step in §1 — a Set with positive ttl expires; `SetBytes` with
   ttl `<= 0` never expires (`cache.go:57-59`).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `cache: no driver registered as "x" (registered: [...])` | backend starter not imported, or driver prefix typo | import `starter-go-redis`/`starter-redigo`/`starter-bigcache`/`starter-memcached`, or correct the prefix (`starter.go:101-113`) |
| `cache: invalid driver ... want "<driver>:<beanID>"` | missing `:` or empty half | write `go-redis:main` form (`starter.go:51-53`) |
| Container fails: no bean of type `*cache.Cache` named `users` | bean is named after the beanID, not the entry (§2.2) | align `driver`'s beanID and the autowire tag |
| Container fails: duplicate bean | two cache entries wrap the same backend instance | one entry per backend instance, or add another backend instance |
| Container fails: unresolvable arg in cache ctor | beanID names no backend instance (`spring.go-redis.<beanID>` absent) | configure the backend entry or fix the beanID |
| Everything wired, `Get` returns 500 | backend down / auth wrong — surfaced as backend error, not miss | check backend starter's health indicator and its own troubleshooting table |
| Values come back JSON-quoted (`"alice"` not `alice`) | façade codec is fixed JSON (`cache.go:82-88`) | unmarshal into typed values, or use `GetBytes/SetBytes` with your own codec at the call site |
| Cache behaves as no-expiry though ttl was set | ttl passed as non-positive (`<=0` means no expiry, `cache.go:57-59`) | pass a positive duration |

---

## 6. Design health

| Metric | Value |
|--------|-------|
| Config keys (façade) | 1 per entry (`driver`) |
| Required | 1 |
| Quickstart external deps | the backing backend's (worked example: 1 — Redis; bigcache: 0) |
| "Watch out" entries | 3 |

Suspect ledger (kept from previous audit + this pass):

1. **No standalone example** exercising `spring.cache` — this doc's worked project is assembled
   from verified backend examples but not smoke-run as one; add a `spring.cache.*` block to an
   existing backend example. (pre-existing)
2. **Cache bean named by backend beanID, not the cache entry key** — `starter.go:58` passes only
   `beanID` into the driver, and every backend does `.Name(beanID)`
   (`starter-go-redis/starter.go:85` et al.); the entry name is validated into existence then
   discarded. Surprising when the two differ, makes "one façade alias per concern over one backend"
   impossible (duplicate beans), and the `Driver` type signature `func(beanID string) gs.ModuleFunc`
   cannot carry the entry name without a breaking change. Candidate: name the bean after the entry,
   fall back to beanID for the lookup. (pre-existing, mechanics now pinned to file:line)
3. **Module has no README** (only USAGE); the driver-registry contract lives in code comments.
   (pre-existing)
4. New this pass: façade codec is hardwired to JSON through the starter path — `WithCodec` exists
   in `cloud/cache` but no driver passes options; no config surface either.
5. New this pass: the module iterates the cache-entry map with no dedup guard against the same
   `(driver, beanID)` appearing twice — the failure is a late duplicate-bean wiring error rather
   than an early `invalid driver`-style message naming the entry.

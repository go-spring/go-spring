# starter-memcached Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `client.go`, `command.go`, `driver.go`, `config.go`,
`health/health.go`, `bytecache/bytecache.go`) and the runnable [example/](example/)
(`check.sh` brings up a docker memcached and self-asserts a SET/GET/INCR round-trip).
**gomemcache's own semantics (sharding, text protocol, Item fields) are
[gomemcache documentation](https://github.com/bradfitz/gomemcache)** — everything below is
go-spring's increment.

**Activation**: beans exist only when any `spring.memcached.*` key is present — `gs.OnProperty("spring.memcached")`
is a prefix check (`starter.go:42`). Multi-instance only: each `spring.memcached.<name>` map entry
becomes one named client bean plus one health indicator; there is no `__default__` singleton.

---

## 1. Complete worked project

File tree (mirrors `example/`):

```
demo/
├── go.mod
├── main.go
├── service.go
├── discovery.go        # registers a discovery backend (only if using service-name)
└── conf/
    └── app.properties
```

**Prerequisite** (single external dependency):

```bash
docker run -d --name demo-memcached -p 127.0.0.1:11211:11211 memcached:1.6
```

**go.mod**:

```
require (
    github.com/bradfitz/gomemcache/memcache latest
    go-spring.org/spring                    v1.3.x
    go-spring.org/starter-memcached         latest
    go-spring.org/starter-actuator          latest   // optional: /readiness folds in memcache health
    go-spring.org/starter-governance        latest   // optional: resilience/fault for memcached ops
)
```

**main.go**:

```go
package main

import "go-spring.org/spring/gs"

func main() { gs.Run() }
```

**service.go** — inject the wrapper, not the raw client (see §2.3):

```go
package main

import (
    "github.com/bradfitz/gomemcache/memcache"
    "go-spring.org/spring/gs"
    StarterMemcached "go-spring.org/starter-memcached"
)

type Service struct {
    // autowire tag = the spring.memcached.<name> map key
    Memcached    *StarterMemcached.Client `autowire:"cache"`
    SessionCache *StarterMemcached.Client `autowire:"session"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}
```

**discovery.go** — only needed for the `service-name` addressing style (static backend here; a
real one talks to Consul/Nacos and pushes fresh snapshots):

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    discovery.RegisterDiscovery("default",
        discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:11211", Healthy: true}))
}
```

**conf/app.properties** (verbatim surface from `example/conf/app.properties`):

```properties
spring.memcached.cache.servers=127.0.0.1:11211

spring.memcached.session.servers=127.0.0.1:11211
spring.memcached.session.timeout=100ms
spring.memcached.session.max-idle-conns=4

# Discovery addressing: no `servers`; the list comes from the registered backend.
spring.memcached.discovery.service-name=memcached-cluster
```

**Verify** (the example exposes the same handlers on :9090; run `go run . -manual`):

```bash
curl http://127.0.0.1:9090/set     # -> OK
curl http://127.0.0.1:9090/get     # -> value
curl http://127.0.0.1:9090/incr    # -> 1, 2, 3 ...
curl http://127.0.0.1:9090/get     # after a delete: "memcache: cache miss"
```

Or headless: `cd example && ./check.sh` — it self-asserts SET/GET/INCR, delete-then-miss,
and a discovery-client round-trip, exiting non-zero on any failure.

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-memcached
  ├─ init: RegisterDriver("DefaultDriver", ...)                     driver.go:35
  └─ init: gs.Module(OnProperty("spring.memcached"), BindEach)       starter.go:42-43
        per spring.memcached.<name> entry:
          r.Provide(newClient, IndexArg(name,c))                     starter.go:46-50
              .Name(name).Init((*Client).Init).Destroy((*Client).Destroy)
          r.Provide(health indicator "memcache:"+name)               starter.go:53
gs.Run()
  ├─ config bind: ${spring.memcached.<name>} → Config (value tags)   config.go:24-61
  ├─ newClient: validate, CreateClient, STARTUP PING (fail-fast)    starter.go:79-101
  ├─ gs field-injects Observability onto the wrapper                 client.go:50
  ├─ Client.Init: observer + resilience/fault executor               client.go:71-76
  ├─ readiness: health indicator folds Ping into /readiness          health/health.go:33-36
  └─ shutdown: Client.Destroy — release executor, stop discovery watch  client.go:83-90
```

Startup ping timing: it runs **inside the constructor**, before the bean exists — a dead server
aborts container assembly with `memcached: startup ping failed` (`starter.go:98-100`); it is not
lazy and not retryable. gomemcache's `Ping` probes every configured server, so one dead node in
`servers` fails the whole instance.

### 2.2 Discovery addressing flow

With `service-name` set (and mesh mode off), `DefaultDriver.CreateClient` builds a
`discovery.Resolver` against the backend named by `discovery` (default `"default"`), filtered by
`scheme` (`driver.go:70`, `driver.go:111-112`). The **initial snapshot only** becomes the client's
server list: empty snapshot fails boot with
`memcached: discovery returned no endpoints for %q` (`driver.go:75-79`). The Resolver's background
Watch is retained solely to own the watch lifecycle — **live membership updates are NOT re-applied**,
because gomemcache shards keys onto a fixed server set chosen at creation (`driver.go:60-66`,
`driver.go:90-96`). Cluster membership changes require a restart; there is no dynamic membership —
for a topology that scales dynamically, point `servers` at a serverless/proxy-style endpoint (one
stable address) and let the proxy own membership. In mesh mode discovery is skipped
entirely and `servers` is used as-is (sidecar owns discovery+LB, `driver.go:69-70` comment).

### 2.3 One Set call, layer by layer

`Client.Set(item)` (`command.go:70-75`):

1. `instrument("set", item.Key)` starts an observe span. gomemcache's API carries no context, so the
   span is a **root span** using `context.Background()` — not linked to the caller's request trace
   (`command.go:38-40`, limitation documented at `client.go:37-41`).
2. `guardErr` runs the op under the resilience executor via `resilience.Run`
   (`command.go:174-181`): limiter/breaker scoped to resource `memcached:<instance-name>`
   (`client.go:73`); `memcache.ErrCacheMiss` counts as success so misses never trip the breaker
   (`command.go:168`); the executor is fault-wrapped (`fault.WrapExecutor`, `client.go:74`) and
   observe-wrapped (`resilobserve.WrapExecutor`, `client.go:75`). With governance off it is a
   transparent no-op.
3. The embedded `*memcache.Client` performs the actual write; the end callback closes the span with
   the error.

All 17 operations (get/get_and_touch/get_multi/touch/set/add/replace/append/prepend/cas/delete/
delete_all/increment/decrement/ping/flush_all) follow this same shape (`command.go:45-162`). Methods
not overridden (only `Close` among lifecycle ones) are promoted unchanged from the embedded client.

### 2.4 The starter-cache bridge

A second `init` registers the `"memcached"` driver with starter-cache (`starter.go:66-73`):
`spring.cache.<name>.driver = memcached:<memcached-instance-name>` exposes a typed
`cache.Cache` over `bytecache.NewByteCache` (`bytecache/bytecache.go:33-36`). TTL conversion:
`toExp` maps ttl to int32 seconds — **0/negative means never expire**, sub-second rounds up to 1s
so it is not silently "forever" (`bytecache/bytecache.go:42-50`). `GetBytes` maps
`ErrCacheMiss` to `cache.ErrMiss`; `Delete` of an absent key is not an error
(`bytecache/bytecache.go:55-79`).

---

## 3. Per-key behavior reference

Nine value tags exist in the starter (verified with the grep audit; `demo.label` in the output
belongs to the example app, not the starter).

| Key (under `spring.memcached.<name>`) | Type | Default | Behavior / interactions | Misconfiguration consequence |
|---|---|---|---|---|
| `servers` | []string | empty | Static server list; requests sharded across it (config.go:28). XOR with `service-name`. | Both empty → ctor error `one of servers or service-name must be set` (starter.go:83); dead address → startup ping fail-fast |
| `service-name` | string | empty | Discovery addressing: resolves the server list through the backend named by `discovery` (config.go:39). When set (non-mesh), `servers` is ignored. | Backend missing → boot error `discovery resolve %q failed`; empty snapshot → boot error (driver.go:76-79) |
| `scheme` | string | empty | Narrows discovery to endpoints of one transport scheme; only consulted with `service-name` (config.go:45). | Over-filtering → "no endpoints" boot error |
| `discovery` | string | `default` | Which registered `discovery.Discovery` resolves `service-name` (config.go:50). | Unknown backend name → boot error |
| `timeout` | duration | 0 | Socket read/write timeout per request; 0 = gomemcache default 100ms (config.go:54). | Too low → spurious timeouts under load |
| `max-idle-conns` | int | 0 | Idle connections kept per server; 0 = driver default 2 (config.go:58). | Too low → reconnect churn |
| `driver` | string | `DefaultDriver` | Selects a registered `Driver` (config.go:61). Custom drivers register via `RegisterDriver`; duplicate names panic (driver.go:45-49). | Unknown name → ctor error `memcached driver not found` (starter.go:87-88) |
| `observability` (on the wrapper bean, top-level) | ObserveConfig | empty | Shared observe-kit config for span/metric/log, field-injected by gs (client.go:50). ⚠ It resolves as a **top-level shared key** — every memcached instance (and other starters reading the same expression) shares one config value, not per-instance. | Absent → default observe behavior |

No `resilience` key: resilience/fault come from the governance center (`govern.*` config of
starter-governance), keyed by resource `memcached:<instance-name>`.

---

## 4. Verification & fault drills

1. **SET/GET round-trip**: `curl :9090/set` → `OK`; `curl :9090/get` → `value`
   (handlers in `example/example.go`; check.sh asserts them headlessly).
2. **Cache-miss semantics**: delete the key, `curl :9090/get` → `memcache: cache miss`; with
   governance on, repeated misses do NOT open the breaker (ErrCacheMiss is success,
   `command.go:168`).
3. **Server-down fail-fast**: stop memcached (`docker stop demo-memcached`), boot the app →
   container assembly aborts with `memcached: startup ping failed` (`starter.go:98-100`). Bad
   `servers` address behaves identically — this is the fail-fast posture, there is no lazy mode.
4. **Discovery bad-address drill**: set `service-name` with no backend registered → boot fails with
   `memcached: discovery resolve %q failed` (`driver.go:71-72`). Register a backend that returns an
   empty endpoint set → boot fails with `discovery returned no endpoints` (`driver.go:78`).
5. **Membership-change limitation drill**: with a discovery instance running, remove the endpoint
   from the backend snapshot — the client keeps dialing the old address until restarted
   (§2.2). This is a documented gomemcache constraint, not a wiring bug.
6. **Health / readiness**: import starter-actuator; each instance contributes an indicator named
   `memcache:<name>` whose probe is a live `Ping` (`starter.go:53`, `health/health.go:34-36`).
   Kill the server, then `curl :9370/readiness` flips DOWN. Note the probe carries no deadline —
   gomemcache's `Ping` has no context; the client timeout bounds it (`health/health.go:30-32`).
7. **Multi-instance**: `cache` and `session` instances coexist (distinct bean names = the map
   keys, `starter.go:47-50`); two entries pointing at the same server are independent beans with
   independent executors (resource labels `memcached:cache` vs `memcached:session`).
8. **Observability**: with starter-otel imported, spans named `memcached.get`/`memcached.set`/...
   appear per operation (root spans, §2.3); duration/in-flight metrics and access log come from the
   shared observe kit (`observe.NewDB("memcached", ...)`, `client.go:72`).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Boot fails `one of servers or service-name must be set` | instance block has neither key | set one of them (`starter.go:83`) |
| Boot fails `memcached: startup ping failed` | server down / wrong address at boot | start memcached, fix `servers`; ping is fail-fast (`starter.go:98-100`) |
| Boot fails `memcached driver not found: X` | `driver` names an unregistered driver | register via `RegisterDriver(X, ...)` in an `init`, or drop the key (`starter.go:87-88`) |
| Boot fails `discovery resolve "..." failed` | `service-name` set but no backend under the `discovery` name | register the backend (`discovery.RegisterDiscovery`) before boot |
| Boot fails `discovery returned no endpoints` | backend healthy but the service has no instances (or `scheme` over-filters) | start instances / clear `scheme` (`driver.go:75-79`) |
| Stale server list after cluster scale-out/scale-in | gomemcache fixes the server set at creation; watch is lifecycle-only | restart the process to re-resolve (`driver.go:60-66`) |
| Traces show memcached spans disconnected from request traces | gomemcache API has no context; spans are root spans | known limitation (`client.go:37-41`); correlate by key/time |
| Breaker never trips on cache misses | by design: ErrCacheMiss counts as success | trip drills must use real failures, not misses (`command.go:168`) |
| `RegisterDriver` panic `already registered` | two inits register the same name (e.g. custom driver named `DefaultDriver`) | rename your driver (`driver.go:45-49`) |
| Readiness stays UP while ops fail | indicator probes `Ping` only; a slow-but-alive server still passes | watch observe metrics for real latency/errors |

---

## 6. Design health

| Metric | Value |
|---|---|
| Config keys | 9 (7 connection + 1 shared observability + 1 via cache-bridge naming) |
| Required | 1 (`servers` xor `service-name`) |
| Quickstart external deps | 1 (memcached, docker) |
| "Watch out" entries | 4 |

Suspect ledger (carried from the previous edition, updated):

- ~~No health indicator~~ — **resolved**: each instance now registers `memcache:<name>` as an
  exported `health.Indicator` (`starter.go:53`, `health/health.go:33-36`); with starter-actuator it
  folds into `/readiness` with no extra wiring.
- `observability` resolves as a top-level shared key — all instances share one config; per-instance
  observability tuning is impossible (`client.go:50`). Cross-family suspect (kept from previous
  edition).
- Discovery watch is lifecycle-only: membership changes need a restart (structural gomemcache
  constraint, `driver.go:60-66`) — consider documenting a rebuild seam or a client-swap pattern
  (cf. the dubbo dynamic-timeout atomic-swap approach).
- Spans are root spans (no context in gomemcache API) — trace correlation is weaker than the
  redis/gorm starters (`client.go:37-41`).

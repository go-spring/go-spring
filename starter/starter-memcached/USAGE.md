# starter-memcached Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `client.go`, `driver.go`, `config.go`,
`health.go`, `bytecache.go`) and the runnable [example/](example/)
(`check.sh` brings up a docker memcached and self-asserts a SET/GET/INCR round-trip).
**gomemcache's own semantics (sharding, text protocol, Item fields) are
[gomemcache documentation](https://github.com/bradfitz/gomemcache)** — everything below is
go-spring's increment.

**Activation**: beans exist only when any `spring.memcached.instances.*` key is present — `gs.OnProperty("spring.memcached")`
is a prefix check (`starter.go:42`). Multi-instance only: each `spring.memcached.instances.<name>` map entry
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
    go-spring.org/starter-governance-file        latest   // optional: resilience/fault for memcached ops
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
    // autowire tag = the spring.memcached.instances.<name> map key
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
    gs.Provide(func() (discovery.Discovery, error) {
        return discovery.NewStaticDiscovery(    discovery.Endpoint{Addr: "127.0.0.1:11211", Healthy: true}), nil
    }).Name("default")
}
```

**conf/app.properties** (verbatim surface from `example/conf/app.properties`):

```properties
spring.memcached.instances.cache.servers=127.0.0.1:11211

spring.memcached.instances.session.servers=127.0.0.1:11211
spring.memcached.instances.session.timeout=100ms
spring.memcached.instances.session.max-idle-conns=4

# Discovery addressing: no `servers`; the list comes from the registered backend.
spring.memcached.instances.discovery.service-name=memcached-cluster
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
  └─ init: gs.Module(OnProperty("spring.memcached"), BindEach)       starter.go:42-43
        per spring.memcached.instances.<name> entry:
          r.Provide(newClient, IndexArg(name,c), IndexArg(3,?Driver))  starter.go:47-51
              .Name(name).Destroy((*Client).Destroy)     # no InitMethod — see below
          r.Provide(health indicator "memcache:"+name)               starter.go:65
gs.Run()
  ├─ config bind: ${spring.memcached.instances.<name>} → Config (value tags)   config.go:24-58
  ├─ ctor newClient [starter.go:87]: validate → optional Driver bean
  │     (none → bundled DefaultDriver) → d.CreateClient(c, backend)
  │         └─ NewClient: identity + observe layer                   client.go
  ├─ ctor: Client.applyGovernance(mgr, inj): resilience/fault executor
  ├─ ctor: HealthCheck (raw client) — assembly is complete before the probe
  ├─ readiness: health indicator folds HealthCheck into /readiness   `health.go`
  └─ shutdown: Client.Destroy — close pool, release executor         client.go
```

**Assembly extension point**: client assembly is owned by a `Driver` (interface,
`driver.go:31-52`). A company/umbrella starter may provide its own `Driver` as an **optional
container bean** (a `gs.Provide(func() StarterMemcached.Driver{...})`, so it can inject config
bound from the properties file at wiring time); every instance under `spring.memcached` is then
built through it. When no such bean exists the starter falls back to the bundled `DefaultDriver`
(`driver.go:55-79`) inside assembly. When several Driver beans coexist, an
entry selects one by name: `spring.memcached.instances.<name>.driver = <bean-name>` (empty = inject the
single Driver bean by type; naming a missing bean fails startup).

`CreateClient` returns the exported `*Client` — the same type apps inject — not the raw
`*memcache.Client`, so a custom driver takes part in the type the ecosystem sees. It returns the
client **identity-complete**: `NewClient` is the only constructor, and the driver passes it the
entry's name and `c.ServiceName`, so a Client can never exist without its identity. The observe
layer is installed there too — it needs no external input, so it is part of what the client *is*,
not a later assembly step. The one thing a driver does not assemble is governance: the resilience
executor is applied by `Client.applyGovernance`, called by the ctor right after, because the
authorities come from the container and a driver must not have to depend on `cloud/governance`.

There is deliberately **no `Init` hook**. Initialization is "what happens right after the object
exists", and the constructor does it in one place: `NewClient` fixes identity and observe,
`applyGovernance` installs governance. Splitting either into a separate `gs.InitMethod` would buy
nothing and give the container a way to hand out a half-built client.

Startup ping timing (opt-in, `ping=true`): it runs **inside the constructor**, before the bean
exists — a dead server aborts container assembly with `memcached: startup ping failed`
(`starter.go`); it is not lazy and not retryable. Off by default, the probe is skipped so a dead
server is not caught at boot and only surfaces on the first operation. gomemcache's `Ping` probes every configured server, so one dead node in
`servers` fails the whole instance. The probe goes straight to the raw client: it is a boot check
on the connection, not business traffic, so it opens no span and spends no limiter/breaker budget.
A failed probe abandons the client, so the ctor releases the governance it had just applied. The
boot probe and the readiness indicator both delegate to `HealthCheck` (`health.go`), the module's
single health implementation.

### 2.2 Discovery addressing flow

With `service-name` set (and mesh mode off), the starter resolves the `discovery` label (default
`"default"`) to a backend bean and passes it to `DefaultDriver.CreateClient` as the `backend`
argument; the driver builds a discovery resolver against it, filtered by
`scheme` (`driver.go:84`, `driver.go:100-103`). The initial snapshot is read at build time as a
fail-fast gate — an empty one fails boot with
`memcached: discovery returned no endpoints for %q` (`driver.go:90-99`) — and the client is then
built over a **live `ServerSelector`** (`selector.go`) that re-reads the snapshot on every key
lookup. So an instance joining or leaving is visible on the next operation; no restart, no
proxy needed. Resolver freshness lives inside the backend, so there is nothing to release.

Two properties are deliberate, because for memcached the selector *is* the cache semantics:

- **Key affinity is preserved.** The selector hashes with the same CRC32-of-key scheme
  gomemcache's own `ServerList` uses, over an **address-sorted** snapshot, so an unchanged
  cluster keeps an unchanged key→server mapping no matter what order the naming service reports.
  A key therefore keeps landing on the instance that holds it.
- **An unusable cluster is reported, not papered over.** An empty snapshot or a failed read
  surfaces as `memcache.ErrNoServers` / the read error on that operation, rather than serving a
  stale set the key may no longer live on. (Weighted distribution is available the way the
  library expresses it — list an address more than once.)

In mesh mode discovery is skipped entirely and `servers` is used as-is (sidecar owns
discovery+LB, `driver.go:76-78` comment).

### 2.3 One Set call, layer by layer

`Client.Set(ctx, item)` (`client.go`):

1. `run`/`runErr` start a client span from the module-local observer (`observe.go`) **with the
   caller's ctx**, so the span joins the caller's request trace. gomemcache's wire call itself
   cannot honor ctx (the socket wait is bounded by `timeout`), but ctx still governs the
   resilience layer's cancellation (rate-limit wait, retry backoff, breaker checks).
2. `run` runs the op under the resilience executor via `resilience.Run`:
   limiter/breaker scoped to service `memcached:<service-name or instance-name>` (`client.go`);
   `memcache.ErrCacheMiss` counts as success so misses never trip the breaker
   (resilience.Tolerate); the executor is applied by `Client.applyGovernance` —
   `fault.WrapClientExecutor(mgr.ClientExecutorFor("memcached", service), service, inj)`, where
   `mgr`/`inj` are the beans the container injects into `newClient` and the ctor passes on, with
   the observe layer applied inside resolve. When no governance is applied it is a transparent
   no-op.
3. The raw client performs the actual write; the span closes with the error.

All 17 operations (get/get_and_touch/get_multi/touch/set/add/replace/append/prepend/cas/delete/
delete_all/increment/decrement/ping/flush_all) follow this same shape (`client.go`). Together with
`Close` they are the whole `*Client` surface: the raw client is a private field, so nothing is
promoted and there is no exported way to reach it — a caller cannot bypass the observe and
governance layers by accident.

### 2.4 The cache abstraction bean

Alongside the wrapper, each instance is provided as a typed `cache.Cache` bean named
`memcached:<service-name or instance-name>` (`starter.go:61-67`) over `NewByteCache`
(`bytecache.go`) — inject `*cache.Cache` with the autowire tag
`memcached:<service-name or instance-name>`. Un-injected, the bean never instantiates, so there is no config
switch to set. TTL conversion:
`toExp` maps ttl to int32 seconds — **0/negative means never expire**, sub-second rounds up to 1s
so it is not silently "forever" (`bytecache.go`). `GetBytes` maps
`ErrCacheMiss` to `cache.ErrMiss`; `Delete` of an absent key is not an error
(`bytecache.go`).

---

## 3. Per-key behavior reference

Eight value tags exist in the per-instance Config (verified with the grep audit; `demo.label` in
the output belongs to the example app, not the starter).

| Key (under `spring.memcached.instances.<name>`) | Type | Default | Behavior / interactions | Misconfiguration consequence |
|---|---|---|---|---|
| `servers` | []string | empty | Static server list; requests sharded across it (config.go:28). XOR with `service-name`. | Both empty → ctor error `one of servers or service-name must be set` (starter.go:92); dead address → startup ping fail-fast (when `ping=true`) |
| `service-name` | string | empty | Discovery addressing: the server set follows the backend named by `discovery` (config.go:39), re-read per key lookup (selector.go). When set (non-mesh), `servers` is ignored. | Backend missing → boot error `discovery resolve %q failed`; empty snapshot at boot → boot error (driver.go:93-99); empty at runtime → `memcache.ErrNoServers` on that operation |
| `scheme` | string | empty | Narrows discovery to endpoints of one transport scheme; only consulted with `service-name` (config.go:45). | Over-filtering → "no endpoints" boot error |
| `discovery` | string | — | Which registered `discovery.Discovery` resolves `service-name` (config.go:50). The wiring resolves this label to a bean and passes it to the driver as the `backend` argument of `CreateClient`. Falls back to `${spring.memcached.default.discovery}` when unset. | Both unset or an unregistered name while service-name is set → boot error. |
| `timeout` | duration | 0 | Socket read/write timeout per request; 0 = gomemcache default 100ms (config.go:54). | Too low → spurious timeouts under load |
| `max-idle-conns` | int | 0 | Idle connections kept per server; 0 = driver default 2 (config.go:58). A governance rule setting `max-conns` overrides it (gomemcache has no open-connection cap, so the rule sizes this idle cap). | Too low → reconnect churn |
| `ping` | bool | false | Opt-in startup probe: `HealthCheck` pings every configured server once and fails the boot on an unreachable one; off by default so a server that is not up yet only surfaces on first use. | Expecting fail-fast without setting it → boot "succeeds", first request fails. |
| `health` | bool | true | Contributes the `memcache:<name>` health.Indicator for the instance. | false → no indicator bean; readiness of that memcached is no longer reported. |

The `driver` key names the Driver bean: empty = fall back to the family-wide `spring.<family>.default.driver`, then to the single Driver bean by type (see
§2.1), set to a bean name to select one explicitly; no `resilience` key: resilience/fault come from the governance center
(`spring.governance.*` config of
starter-governance-file), keyed by service `memcached:<service-name or instance-name>`.: resilience/fault come from the governance center (`spring.governance.*` config of
starter-governance-file), keyed by service `memcached:<service-name or instance-name>`.

---

## 4. Verification & fault drills

1. **SET/GET round-trip**: `curl :9090/set` → `OK`; `curl :9090/get` → `value`
   (handlers in `example/example.go`; check.sh asserts them headlessly).
2. **Cache-miss semantics**: delete the key, `curl :9090/get` → `memcache: cache miss`; with
   governance on, repeated misses do NOT open the breaker (ErrCacheMiss is success,
   `client.go`).
3. **Server-down fail-fast**: set `ping=true`, stop memcached (`docker stop demo-memcached`), boot
   the app → container assembly aborts with `memcached: startup ping failed` (`starter.go:123`).
   Bad `servers` address behaves identically. With the default `false` no boot probe runs, so the
   failure only surfaces on the first operation.
4. **Discovery bad-address drill**: set `service-name` with no backend registered → boot fails with
   `memcached: discovery resolve %q failed` (`driver.go:92`). Register a backend that returns an
   empty endpoint set → boot fails with `discovery returned no endpoints` (`driver.go:104`).
5. **Membership-change limitation drill**: with a discovery instance running, remove the endpoint
   from the backend snapshot — the client keeps dialing the old address until restarted
   (§2.2). This is a documented gomemcache constraint, not a wiring bug.
6. **Health / readiness**: import starter-actuator; each instance contributes an indicator named
   `memcache:<name>` whose probe is a live `HealthCheck` (`starter.go:65`, `health.go`).
   Kill the server, then `curl :9370/readiness` flips DOWN. Note the probe carries no deadline —
   gomemcache's `Ping` has no context; the client timeout bounds it (`health.go`).
7. **Multi-instance**: `cache` and `session` instances coexist (distinct bean names = the map
   keys, `starter.go:47-50`); two entries pointing at the same server are independent beans with
   independent executors (service labels `memcached:cache` vs `memcached:session`).
8. **Observability**: with starter-otel imported, a client span named after the operation
   (`get`/`set`/...) appears per call, joined to the caller's trace via the passed ctx (§2.3). The
   starter declares the operation's identity (`observe.go`); the resilience layer emits from it, so
   the call-level `db.client.operation.duration`, the attempt-level `db.client.attempt.duration`
   histograms and the `_app_memcached_access` access log all come from there.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Boot fails `one of servers or service-name must be set` | instance block has neither key | set one of them (`starter.go:92`) |
| Boot fails `memcached: startup ping failed` | server down / wrong address at boot | emitted only when `ping=true`; start memcached, fix `servers` (`starter.go:123`). With `ping` off the failure surfaces on the first operation. |
| Boot fails `discovery resolve "..." failed` | `service-name` set but no backend under the `discovery` name | register the backend bean (named discovery.Discovery) before boot |
| Boot fails `discovery returned no endpoints` | backend healthy but the service has no instances (or `scheme` over-filters) | start instances / clear `scheme` (`driver.go:97-98`) |
| Stale server list after cluster scale-out/scale-in | gomemcache fixes the server set at creation; watch is lifecycle-only | restart the process to re-resolve (`driver.go:61-68`) |
| Traces show memcached spans disconnected from request traces | a `context.Background()`-style ctx (no trace) was passed; spans follow the caller's ctx | pass the request's ctx so the span joins the trace; the wire call itself still ignores ctx (bounded by `timeout`) |
| Breaker never trips on cache misses | by design: ErrCacheMiss counts as success | trip drills must use real failures, not misses (`client.go`) |
| Readiness stays UP while ops fail | indicator probes `Ping` only; a slow-but-alive server still passes | watch observe metrics for real latency/errors |

---

## 6. Design health

| Metric | Value |
|---|---|
| Config keys | 9 (8 connection + 1 via cache-bridge naming) |
| Required | 1 (`servers` xor `service-name`) |
| Quickstart external deps | 1 (memcached, docker) |
| "Watch out" entries | 4 |

Suspect ledger (carried from the previous edition, updated):

- ~~No health indicator~~ — **resolved**: each instance now registers `memcache:<name>` as an
  exported `health.Indicator` (`starter.go:65`, `health.go`); with starter-actuator it
  folds into `/readiness` with no extra wiring.
- Discovery watch is lifecycle-only: membership changes need a restart (structural gomemcache
  constraint, `driver.go:61-68`) — consider documenting a rebuild seam or a client-swap pattern
  (cf. the dubbo dynamic-timeout atomic-swap approach).
- The wire call cannot be cancelled via ctx (no context in gomemcache API; bounded by `timeout`) —
  only the resilience layer honors cancellation (`client.go`).

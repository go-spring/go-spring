# starter-registry-nacos Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `center.go`, `registrar.go`,
`discovery_nacos.go`), the registration core (`../starter-registry/starter.go`,
`../starter-registry/config.go`) and the runnable [example/](example/) (docker-compose +
`check.sh`). **Nacos's own semantics (services, groups, namespaces, clusters, ephemeral
instances, heartbeat) are
[Nacos's documentation](https://nacos.io/en-us/docs/what-is-nacos.html)** — everything below is
go-spring's increment: registration lifecycle, discovery adapters, and weight/drain wiring.

**Scope**: this starter ships BOTH halves of Nacos service discovery, both carried by ONE bean
per configured center —
- **Provider half** (`spring.registry.nacos.<name>.*` + `spring.registry.*`): registers this
  process into every configured Nacos center once the app is ready. For VM / bare-metal /
  hybrid deployments; in pure Kubernetes the platform registers Pods for you.
- **Consumer half** (no config of its own): the same bean is a `cloud/discovery` backend which
  client starters (gateway `lb://` routes, `spring.http-client.backends.<n>.discovery`, etc.)
  resolve through by citing the bean name `nacos.<name>`.

**Activation**: each `spring.registry.nacos.<name>` block is one registry center — one shared
naming client, one startup probe, one lifecycle (`center.go`). The bean named `nacos.<name>`
exports BOTH `discovery.Registrar` (collected by the starter-registry core when
`spring.registry.service-name` is set — a pure consumer app registers nothing) and
`discovery.Discovery` (cited by bean name, so a pure provider never pays for the read half).

No `enabled` key anywhere. No default/unnamed block: the block name is mandatory and becomes
part of the bean name.

---

## 1. Complete worked project

A provider (`orders`) registered into Nacos AND a consumer (gateway) resolving it via an
`lb://` route — the full register → resolve → drain loop. File tree:

```
demo/
├── go.mod
├── provider/
│   ├── main.go
│   └── conf/app.properties
└── gateway/
    ├── main.go
    └── conf/app.properties
```

**go.mod** (deps that matter):

```
require (
    go-spring.org/spring                     v1.3.x
    go-spring.org/starter-registry-nacos     latest
    go-spring.org/starter-gateway            latest   // consumer: lb:// routes via cloud/discovery
    go-spring.org/starter-echo               latest   // the served HTTP surface of the provider
)
```

**provider/main.go** — one import line is the whole integration (the registration core
`starter-registry` comes in transitively):

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-echo"              // serves :8080
    _ "go-spring.org/starter-registry-nacos"    // registers :8080 into nacos on ready
)

func main() { gs.Run() }
```

**provider/conf/app.properties** — the complete commented surface:

```properties
# --- nacos naming connection: one NAMED block per server (setting server
#     ACTIVATES that block; the bean name is "nacos." + the block name) ------
spring.registry.nacos.main.server=127.0.0.1:8848
spring.registry.nacos.main.namespace=            # empty = "public"
spring.registry.nacos.main.group=DEFAULT_GROUP   # shared by registration AND discovery
spring.registry.nacos.main.cluster=DEFAULT       # shared by registration AND discovery
# spring.registry.nacos.main.username= / password=   # when nacos auth is enabled
# a second center is a second block: registration fans out to both (dual write)
# spring.registry.nacos.dr.server=10.0.2.1:8848

# --- the instance to advertise (backend-agnostic keys, shared by ALL centers)
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080         # required when registering; never guessed for you
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1

# --- the served surface ------------------------------------------------------
spring.http.server.enabled=false
spring.echo.server.addr=:8080
```

**gateway/main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-gateway"
    _ "go-spring.org/starter-registry-nacos" // same starter provides discovery backends
)

func main() { gs.Run() }
```

**gateway/conf/app.properties** — only the connection block; discovery cites the backend
bean's name and needs no config of its own:

```properties
# The center: same block a pure provider would set (no service-name → registers nothing).
spring.registry.nacos.main.server=127.0.0.1:8848
spring.registry.nacos.main.group=DEFAULT_GROUP

spring.gateway.server.addr=:9440
spring.gateway.discovery=nacos.main                 # the backend bean's name

spring.gateway.routes.orders.path=/api/**
spring.gateway.routes.orders.upstream.target=lb://orders
spring.gateway.routes.orders.upstream.balancer=weighted
```

**Verify** (nacos from `example/docker-compose.yml` — `docker compose up -d`, then wait for
`curl -fsS :8848/nacos/v1/console/health/readiness`, mirroring `example/check.sh`):

```bash
go run ./provider &      # log: tag _app_registry_nacos "registered \"orders\" at 127.0.0.1:8080"
go run ./gateway &
curl -s :9440/api/hello  # 200 via lb://orders → 127.0.0.1:8080
# resolve directly (mirrors example/example.go verifyOnce):
curl -s '127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=orders' | jq '.hosts[] | {ip,port,weight,enabled,healthy}'
```

The example's one-shot self-verification (`go run ./example -manual` to keep it up) prints
`registered addr=... meta=...` after readiness and self-SIGTERMs to exercise deregistration.

---

## 2. Assembly & timing

### 2.1 Registration lifecycle (provider half)

Registration is owned by the **starter-registry core** (`../starter-registry`), imported
transitively by this starter; the nacos starter only contributes one registrar bean per block:

```
import starter-registry-nacos
  ├─ per block ${spring.registry.nacos.<name>}: Provide(newNacosBackend)
  │    Name("nacos."+name).Export(As[discovery.Discovery], As[discovery.Registrar])
  │    condition: gs.OnProperty("spring.registry.nacos")              [center.go]
  └─ import starter-registry (core, exactly once per process)
       └─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
            condition: gs.OnProperty("spring.registry.service-name")
gs.Run()
  ├─ conf.BindEach over spring.registry.nacos.* → one NacosConfig per block
  ├─ newNacosBackend per block: build nacos naming client + FAIL-FAST PROBE
  │    (GetAllServicesInfo page-1 — unreachable/auth-bad server aborts startup)  [center.go]
  ├─ registryServer collects every backend's Registrar via []discovery.Registrar
  │    slice injection (across ALL backends — nacos, zookeeper, ...)
  ├─ Run: validate service-name/addr non-empty AND ≥1 registrar BEFORE readiness
  ├─ <-sig.TriggerAndWait()   ← readiness gate: register only when the whole app is up
  ├─ for each registrar: RegisterInstance(ephemeral=true, weight normalized <=0→1)
  │    SDK background heartbeat keeps the entry alive; Nacos drops it ~15s after the
  │    process dies without Deregister — correctness never depends on Deregister;
  │    ANY center's failure aborts startup (consumers' views must not split)
  ├─ <-ctx.Done()            ← blocks until shutdown
  └─ SIGTERM: PreStop → Deregister from EVERY center FIRST (before pre-stop delay,
       before any server stops serving) → in-flight requests drain losslessly
       Stop deregisters again as fallback; deregister is idempotent
```

Why register-on-ready and not at boot: consumers must never resolve an address whose HTTP
server is not yet listening. Why deregister in PreStop: discovery stops handing the address
out while the server is still serving in-flight requests — that ordering is what makes a
rolling restart lossless.

### 2.2 The freshness path (consumer half)

Freshness lives entirely inside the backend; there is no client-side watch loop
(`discovery_nacos.go`):

1. The FIRST `Resolve(service)` pays a seed query (`SelectInstances` with `HealthyOnly=true`)
   within the block's group/cluster, then opens a Nacos subscription
   (`client.Subscribe(ServiceName, GroupName, cb)`). Subscribe failure fails that Resolve.
2. Every Nacos push (register/deregister/weight/health change) invokes the callback, which
   maps instances → endpoints (`Enable` inverted into `Disabled`, `Metadata["scheme"]`
   carried, sorted by addr) and stores the full snapshot under the entry's mutex.
3. Later `Resolve` calls are in-memory reads of the cached snapshot (optionally narrowed by
   a `scheme` query option); a discovery `Loader` just reads this backend's current snapshot,
   and a `loadbalance.Pool.Pick` reads the live snapshot on every pick.

Failure modes: a push error keeps the last snapshot (stale addresses beat none — Warn log);
a failed seed query fails that first Resolve, and the subscription still delivers the first
push afterwards.

### 2.3 The DRAIN path (UpdateWeight(0))

1. Operator/`preStop` hook calls `Server.UpdateWeight(ctx, 0)` on the `registryServer` bean
   (autowire by name, or keep a reference from the ctor). Precondition: registration has
   happened (errors "instance not registered yet" otherwise).
2. The core broadcasts to EVERY registrar; the nacos side (`registrar.go`) issues one
   `UpdateInstance` per center — **0 passes through** (Nacos natively treats 0 as
   receive-no-traffic); only negatives normalize to 1. The instance stays ephemeral, so its
   heartbeat keeps running and subscribers get the new weight on the next push — no
   re-register.
3. Watchers see 0: the push re-renders endpoints with `Weight: 0`; the snapshot is replaced;
   a `loadbalance.Pool.Pick` runs `excludeDrained` after eligibility/ejection filtering:
   every `Weight == 0` endpoint is dropped; **fallback**: if ALL endpoints are zero-weighted
   the filter yields to its input (degrades to an even split) so unnormalized snapshots never
   blackhole the pool. Negative weights are kept (misconfiguration must not silently remove
   an instance).
4. Deregister-on-shutdown then removes the entry entirely.

---

## 3. Per-key behavior reference

### 3.1 `spring.registry.nacos.<name>.*` (nacos connection — 7 keys per block, `config.go`)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `server` | string | — | **Block activation key**. `host:port`; split/parsed at ctor. | Missing in a block → binding fails for that block. Malformed → startup error "must be host:port". |
| `namespace` | string | "" (public) | Namespace id shared by registration and discovery (the block owns it). | Mismatch with the providers you consume → they are in a different namespace. |
| `group` | string | DEFAULT_GROUP | Group shared by registration and discovery — one value, both halves. | Mismatch with a provider registered elsewhere → resolution silently returns empty. |
| `cluster` | string | DEFAULT | Nacos cluster name shared by both halves. | Provider in cluster X + this block pinned elsewhere → you miss it. |
| `username` / `password` | string | "" | Nacos auth; validated by the startup probe. | Bad credentials → startup fails at the probe, not at first Register. |
| `timeout-ms` | uint64 | 5000 | Bounds each nacos API call incl. the probe. | Too low → flaky probe/register on slow links. |

Two blocks with the same `<name>` fail loudly in the container (duplicate bean name); block
names across backends never collide (the bean name carries the backend type, e.g. `nacos.main`
vs `zookeeper.main`).

### 3.2 `spring.registry.*` (the instance — 5 keys, `../starter-registry/config.go`)

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `service-name` | string | "" | Logical name clients resolve. **Registration intent signal**: set means this process publishes itself into every configured center; unset means pure consumer. | Unset with blocks configured → registers nothing (a valid pure-consumer app). |
| `addr` | string | "" | Advertised `host:port`. Required when registering; never guessed. | Empty with service-name set → startup error listing both required keys. |
| `id` | string | "" | Bound by the core; the nacos backend ignores it (Nacos identifies instances by ip:port). | — |
| `weight` | int | 100 | Write-side normalization: `<=0` is stored as **1**. Only the runtime API `UpdateWeight(0)` can store 0 (drain). ⚠ same field, opposite semantics by call path. | Config `weight=0` does NOT drain — instance is reachable at weight 1. |
| `metadata` | map | empty | Stored with the instance; `scheme` key is the transport convention (`tls`/`https`/...), others are free-form (zone, version). | Consumers using scheme-filtering and no `scheme` key treat the instance as plain TCP. |

### 3.3 Discovery (no keys — cite the bean name)

There is no per-backend discovery config path. Each block's bean IS the discovery backend,
named `nacos.<name>` (`center.go`), sharing the block's naming client, namespace, group and
cluster — read and write cannot diverge because they are the same values. Clients cite the
bean name: gateway `spring.gateway.discovery=nacos.main` / per-route
`upstream.discovery=nacos.main`, `spring.http-client.backends.<n>.discovery=nacos.main`,
etc. The read half costs nothing until first cited: a pure provider never pays for it. To
resolve from several centers, cite each center's bean where you need it.

---

## 4. Verification & fault drills

All drills assume the §1 project + `example/docker-compose.yml` nacos.

### 4.1 Register / resolve / deregister

```bash
go run ./provider &           # wait for log: _app_registry_nacos "registered \"orders\" ..."
curl -s '127.0.0.1:8848/nacos/v1/ns/instance/list?serviceName=orders&groupName=DEFAULT_GROUP' \
  | jq '.hosts[] | {ip,port,weight,ephemeral,enabled,healthy}'
kill -TERM <provider-pid>     # PreStop deregisters: same query now returns hosts: []
```

The example self-runs this loop (`example/check.sh` asserts `registered addr=` in the output,
then the SIGTERM exercises deregister).

### 4.2 Dual write (two blocks) and drain

Add a second block (`spring.registry.nacos.dr.server=...`) to the provider and repeat the
§4.1 query against BOTH servers: the instance appears in each (one publication fanned out),
and the log line says `in 2 registry center(s)`. Then drain — expose the `registryServer`
bean (autowire by name) and call `UpdateWeight(ctx, 0)` from a signal handler or admin
route; or use the nacos console/API:

```bash
# drain via nacos open-api (same UpdateInstance the starter calls):
curl -s -X PUT '127.0.0.1:8848/nacos/v1/ns/instance?serviceName=orders&ip=127.0.0.1&port=8080&weight=0&ephemeral=true&groupName=DEFAULT_GROUP'
# observe from the consumer: gateway /orders traffic stops hitting :8080;
# with 2+ provider replicas at unequal weight, `weighted` balancer shares shift accordingly.
```

If every replica is drained, the pool falls back to an even split rather than erroring — a
fully-drained service must be caught by monitoring (ErrNoAvailable only when the set is empty
or all Disabled).

### 4.3 Kill provider / heartbeat expiry

1. `kill -9 <provider-pid>` (no graceful TERM): Deregister never runs; the SDK heartbeat stops
   and Nacos drops the ephemeral instance after its heartbeat timeout (~15s default).
2. Consumer side: the Nacos push fires, the backend's snapshot loses the endpoint, a
   discovery `Loader` re-reads it, and `Pool.Pick` stops choosing it. No code change — this
   is why Register being ephemeral makes correctness independent of Deregister.
3. Contrast with graceful TERM (4.1): removal is immediate, not ~15s.

### 4.4 Freshness degradation drills

- Stop nacos (`docker compose stop nacos`): consumer logs Warn
  `push for orders failed (keeping last snapshot)` — stale addresses keep serving.
- First resolve against an empty/unready server: the seed query errors; the subscription
  recovers on the first push.

### 4.5 Fail-fast at boot

```properties
spring.registry.nacos.main.server=127.0.0.1:9999   # nothing there
```

```bash
go run ./provider   # startup aborts: "registry-nacos: startup probe failed for 127.0.0.1:9999"
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No registration, no logs at all | no `spring.registry.nacos.*` block (or its `server` missing) | Configure a named block — any key under `spring.registry.nacos.<name>.*` activates the backend. |
| Startup aborts "startup probe failed" | Nacos unreachable, or bad namespace/credentials | Fix server/namespace/username/password; the probe is a 1-row service listing. |
| Startup aborts "service-name and addr are required" | `spring.registry.*` incomplete | Set both; validation runs before readiness. |
| Startup aborts "... but no registry center is configured" | `service-name` set yet no connection block anywhere | Add at least one `spring.registry.<backend>.<name>` block. |
| Registered but consumer resolves nothing | the provider registered under a different group/namespace than the cited block's | Both sides must use the same block-level `group`/`namespace` (this starter shares one value for read and write). |
| Consumer sees the service but not the instance | block `cluster` (default DEFAULT) pinned to a cluster the provider is not in | Align the block's `cluster` with the provider's cluster. |
| Consumer keeps stale addresses forever | nacos down / push errors — the backend intentionally keeps the last snapshot | Restore nacos; check Warn `push ... failed` lines. |
| `UpdateWeight` errors "instance not registered yet" | called before Run registered | Call only after readiness (log line `registered ...`). |
| Config `weight=0` but instance still gets traffic | write-side normalization stores 1 | Use the runtime API `UpdateWeight(ctx, 0)` — only that path stores 0. |
| Instance not removed ~15s after crash | instance registered as persistent — not possible via this starter | All registrations are `Ephemeral: true`; check for a foreign writer to the same ip:port. |

Runtime logs carry the tag `_app_registry_nacos` (`log.RegisterAppTag("registry_nacos", "")`);
tune verbosity via `logger.<name>.tag=_app_registry_nacos`.

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 12 (7 per-block connection + 5 instance) |
| Required | 1 per block (`server`) + 2 when registering (`service-name`, `addr`); a pure consumer needs only one block's `server` |
| Quickstart external deps | 1 (nacos server; docker-gated in example) |
| "Watch out" entries | 6 |

Resolved 2026-08 (config-hygiene pass): dead `id` key deleted (Nacos identifies instances by
ip:port; the core still binds an `id` under `${spring.registry}` for other backends and the
nacos side ignores it). Resolved 2026-09 (multi-registry named blocks): the single-block form
was replaced by `spring.registry.nacos.<name>.*` — one backend bean per block named
`nacos.<name>` carrying BOTH the registrar and the discovery backend, registration moved into
the shared starter-registry core (one publication fanned out to every center), and discovery
now cites the bean name instead of a fixed label. Remaining ledger items: config `weight=0`
becomes 1 while API `UpdateWeight(0)` drains — same key, opposite semantics by call path; no
metrics on registration/watch health (log-only observability).

# starter-registry-nacos Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `registrar.go`, `discovery_nacos.go`)
and the runnable [example/](example/) (docker-compose + `check.sh`). **Nacos's own semantics
(services, groups, namespaces, clusters, ephemeral instances, heartbeat) are
[Nacos's documentation](https://nacos.io/en-us/docs/what-is-nacos.html)** — everything below is
go-spring's increment: registration lifecycle, discovery adapters, and weight/drain wiring.

**Scope**: this starter ships BOTH halves of Nacos service discovery —
- **Provider half** (`spring.registry.nacos.*` + `spring.registry.*`): registers this process
  into Nacos once the app is ready. For VM / bare-metal / hybrid deployments; in pure
  Kubernetes the platform registers Pods for you (`starter.go:17-27`).
- **Consumer half** (`spring.discovery.nacos.<name>.*`): named `cloud/discovery` backends that
  client starters (gateway `lb://` routes, etc.) resolve through.

**Activation**: the registrar exists only when `spring.registry.nacos.server` is set
(`starter.go:64-69`, `gs.OnProperty` prefix check); each discovery backend exists when its
`spring.discovery.nacos.<name>.server` is set. No `enabled` key anywhere.

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
    go-spring.org/starter-registry-nacos     latest   // experimental path; see README for module path
    go-spring.org/starter-gateway            latest   // consumer: lb:// routes via cloud/discovery
    go-spring.org/starter-echo               latest   // the served HTTP surface of the provider
)
```

**provider/main.go** — one import line is the whole integration:

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
# --- nacos naming connection (setting server ACTIVATES the registrar) --------
spring.registry.nacos.server=127.0.0.1:8848
spring.registry.nacos.namespace=            # empty = "public"
spring.registry.nacos.group=DEFAULT_GROUP   # discovery clients must use the same group
spring.registry.nacos.cluster=DEFAULT       # must match the discovery side's cluster
# spring.registry.nacos.username= / password=   # when nacos auth is enabled

# --- the instance to advertise (backend-agnostic keys) -----------------------
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080         # required; never guessed for you
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

**gateway/conf/app.properties**:

```properties
# One named discovery backend per nacos cluster/environment.
spring.discovery.nacos.prod.server=127.0.0.1:8848
spring.discovery.nacos.prod.namespace=
spring.discovery.nacos.prod.group=DEFAULT_GROUP   # ⚠ must match the provider's group
spring.discovery.nacos.prod.cluster=DEFAULT       # "" = span all clusters

spring.gateway.server.addr=:9440
spring.gateway.discovery=prod                     # default backend for lb:// routes

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

```
import starter-registry-nacos
  └─ gs.Provide(NewServer)  Name("registryServer").Export(gs.As[gs.Server]())
        └─ Condition: OnProperty("spring.registry.nacos.server")     [starter.go:64-69]
gs.Run()
  ├─ bind ${spring.registry.nacos} → NacosConfig (TagArg ctor arg)
  ├─ NewServer: build nacos naming client + FAIL-FAST PROBE
  │    (GetAllServicesInfo page-1 — unreachable/auth-bad server aborts startup)  [registrar.go:83-90]
  ├─ bind ${spring.registry} → Server.Config (RegistrationConfig)
  ├─ Run: validate service-name/addr non-empty BEFORE readiness                      [starter.go:100-102]
  ├─ <-sig.TriggerAndWait()   ← readiness gate: register only when the whole app is up
  ├─ RegisterInstance(ephemeral=true, weight normalized <=0→1)                       [registrar.go:98-128]
  │    SDK background heartbeat keeps the entry alive; Nacos drops it ~15s after the
  │    process dies without Deregister — correctness never depends on Deregister
  ├─ <-ctx.Done()            ← blocks until shutdown
  └─ SIGTERM: PreStop → Deregister FIRST (before pre-stop delay, before any server
       stops serving) → in-flight requests drain losslessly                        [starter.go:127-129]
       Stop/StopContext deregister again as fallback; deregister is idempotent
```

Why register-on-ready and not at boot: consumers must never resolve an address whose HTTP
server is not yet listening. Why deregister in PreStop: discovery stops handing the address
out while the server is still serving in-flight requests — that ordering is what makes a
rolling restart lossless (`starter.go:30-34`).

### 2.2 The WATCH path (consumer half)

One push, end to end (`discovery_nacos.go:177-245`):

1. `client.Subscribe(ServiceName, GroupName, cb)` — Nacos pushes the full instance list on
   every change (register/deregister/weight/health). Subscribe failure fails the `Watch` call.
2. The callback **never touches the channel**: it maps instances → endpoints
   (`Enable` inverted into `Disabled`, `Metadata["scheme"]` carried, sorted by addr), stores
   the snapshot under a mutex, and posts one non-blocking signal into a `cap-1` channel —
   a pending signal suffices, snapshots are full, not deltas (`discovery_nacos.go:184-195`).
3. A single goroutine owns the output channel as sole writer AND closer (no send-after-close
   race — the k8s starter's discipline), seeds it with an explicit `SelectInstances`
   (`HealthyOnly=true`) so the first result does not depend on SDK callback timing, then
   loops: on signal, render the snapshot to a comparable key (`addr,scheme,weight;disabled,healthy`,
   `endpointsKey`) and **only forward when the key changed** — no-op re-deliveries must not
   churn consumers (`discovery_nacos.go:226-242`).
4. Downstream, `cloud/discovery.Resolver.loop` replaces its atomic snapshot per push;
   `loadbalance.Pool.Pick` reads the live snapshot on every pick.

Failure modes: a callback error **keeps the last snapshot** (stale addresses beat none — Warn
log, `discovery_nacos.go:197-205`); a failed initial query logs a Warn and waits for the first
push (`discovery_nacos.go:220-224`); ctx cancel closes the channel and unsubscribes.

### 2.3 The DRAIN path (UpdateWeight(0))

1. Operator/`preStop` hook calls `Server.UpdateWeight(ctx, 0)` (inject the `gs.Server` bean by
   name `registryServer`, or keep a reference from the ctor). Precondition: registration has
   happened (`starter.go:149-154` errors "instance not registered yet" otherwise).
2. Write semantics (`registrar.go:155-185`): a single `UpdateInstance` call — **0 passes
   through** (Nacos natively treats 0 as receive-no-traffic); only negatives normalize to 1.
   The instance stays ephemeral, so its heartbeat keeps running and subscribers get the new
   weight on the next push — no re-register.
3. Watchers see 0: the push re-renders endpoints with `Weight: 0`; the key changes → snapshot
   forwarded; `endpointsKey` includes weight, so this is a real change event.
4. `loadbalance.Pool.Pick` runs `excludeDrained` after eligibility/ejection filtering: every
   `Weight == 0` endpoint is dropped; **fallback**: if ALL endpoints are zero-weighted the
   filter yields to its input (degrades to an even split) so unnormalized snapshots never
   blackhole the pool (`cloud/loadbalance/pool.go:93-134`). Negative weights are kept
   (misconfiguration must not silently remove an instance).
5. Deregister-on-shutdown then removes the entry entirely.

---

## 3. Per-key behavior reference

### 3.1 `spring.registry.nacos.*` (nacos connection — 7 keys, `config.go:21-44`)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `server` | string | — | **Activation key** (OnProperty). `host:port`; split/parsed at ctor. | Missing → registrar silently absent. Malformed → startup error "must be host:port". |
| `namespace` | string | "" (public) | Namespace id registered into. | Mismatch with clients → they resolve a different (empty) namespace. |
| `group` | string | DEFAULT_GROUP | Group the instance is published under. | ⚠ must equal the discovery side's `group` or resolution silently returns empty. |
| `cluster` | string | DEFAULT | Nacos cluster name; symmetric with the discovery side's default (3.3). | Provider in cluster X + consumer pinned elsewhere → consumer misses it. |
| `username` / `password` | string | "" | Nacos auth; validated by the startup probe. | Bad credentials → startup fails at the probe, not at first Register. |
| `timeout-ms` | uint64 | 5000 | Bounds each nacos API call incl. the probe. | Too low → flaky probe/register on slow links. |

### 3.2 `spring.registry.*` (the instance — 4 keys, `config.go:50-66`)

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `service-name` | string | "" | Logical name clients resolve. **Required** — validated in Run pre-readiness. | Empty → startup error listing both required keys. |
| `addr` | string | "" | Advertised `host:port`. **Required**; never guessed. | Empty → startup error; malformed → register-time error from `splitAddr`. |
| `weight` | int | 0 | Write-side normalization: `<=0` is stored as **1** (registrar.go:104-107). Only the runtime API `UpdateWeight(0)` can store 0 (drain). ⚠ same field, opposite semantics by call path. | Config `weight=0` does NOT drain — instance is reachable at weight 1. |
| `metadata` | map | empty | Stored with the instance; `scheme` key is the transport convention (`tls`/`https`/...), others are free-form (zone, version). | Consumers using scheme-filtering and no `scheme` key treat the instance as plain TCP. |

### 3.3 `spring.discovery.nacos.<name>.*` (7 keys per named backend, `discovery_nacos.go:58-78`)

| Key | Type | Default | Behavior | Misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `server` | string | — | **Required**, `expr:"$ != ''"` validated. Duplicate `<name>` → startup error "already registered". | Missing → module errors at bind. |
| `namespace` | string | "" | Must match the provider's. When the registrar side is configured, divergence triggers a startup WARN (`discovery_nacos.go` warnRegistryDivergence). | Silent empty resolution across processes; the WARN only covers same-process registrar+discovery. |
| `group` | string | DEFAULT_GROUP | Resolution scope. Divergence from the registrar side also WARNs at startup. | ⚠ mismatch with provider group → empty result set, no error. |
| `cluster` | string | DEFAULT | Narrows to one nacos cluster; an explicit empty value spans all clusters. Default aligned (2026-08) with the registrar's `DEFAULT` so a no-config consumer sees a no-config provider. | **Migration**: pre-2026-08 the default was "" (all clusters) — consumers relying on that must now set `cluster=` explicitly (empty = all) or pin the provider's cluster. |
| `username` / `password` | string | "" | Same auth idiom + startup probe as the registrar. | Startup fails at probe. |
| `timeout-ms` | uint64 | 5000 | Per-call bound. | Same as 3.1. |

Backends are named adapters in the global `cloud/discovery` registry, not beans: a client
starter cites the name (gateway: `spring.gateway.discovery` / per-route `upstream.discovery`).

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

### 4.2 Weight propagation & drain (the §2.3 path)

Expose the `gs.Server` bean (autowire by name `registryServer`) and call
`UpdateWeight(ctx, 0)` from a signal handler or admin route; or use nacos console/API:

```bash
# drain via nacos open-api (same UpdateInstance the starter calls):
curl -s -X PUT '127.0.0.1:8848/nacos/v1/ns/instance?serviceName=orders&ip=127.0.0.1&port=8080&weight=0&ephemeral=true&groupName=DEFAULT_GROUP'
# observe from the consumer: gateway /orders traffic stops hitting :8080;
# with 2+ provider replicas at unequal weight, `weighted` balancer shares shift accordingly.
```

Watch the consumer log for absence of churn: no-op re-deliveries are suppressed by the
`endpointsKey` check; a weight change IS forwarded (weight is in the key). If every replica is
drained, the pool falls back to an even split rather than erroring — a fully-drained service
must be caught by monitoring (ErrNoAvailable only when the set is empty or all Disabled).

### 4.3 Kill provider / heartbeat expiry

1. `kill -9 <provider-pid>` (no graceful TERM): Deregister never runs; the SDK heartbeat stops
   and Nacos drops the ephemeral instance after its heartbeat timeout (~15s default).
2. Consumer side: the Nacos push fires, the snapshot loses the endpoint, `Resolver` swaps it,
   `Pool.Pick` stops choosing it. No code change — this is why Register being ephemeral makes
   correctness independent of Deregister (`registrar.go:44-47`).
3. Contrast with graceful TERM (4.1): removal is immediate, not ~15s.

### 4.4 Watch degradation drills

- Stop nacos (`docker compose stop nacos`): consumer logs Warn
  `watch orders callback error (keeping last snapshot)` — stale addresses keep serving.
- First query against an empty/unready server: Warn
  `initial query for orders failed (waiting for first push)` — recovers on first push.

### 4.5 Fail-fast at boot

```bash
spring.registry.nacos.server=127.0.0.1:9999   # nothing there
go run ./provider   # startup aborts: "registry-nacos: startup probe failed for 127.0.0.1:9999"
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| No registration, no logs at all | `spring.registry.nacos.server` missing | Set it — the key is the activation switch. |
| Startup aborts "startup probe failed" | Nacos unreachable, or bad namespace/credentials | Fix server/namespace/username/password; the probe is a 1-row service listing. |
| Startup aborts "service-name and addr are required" | `spring.registry.*` incomplete | Set both; validation runs before readiness. |
| Registered but consumer resolves nothing | group or namespace mismatch provider↔consumer | Align `spring.registry.nacos.group/namespace` with `spring.discovery.nacos.<name>.group/namespace`. |
| Consumer sees the service but not the instance | consumer `cluster` (default DEFAULT) pinned to a cluster the provider is not in | Set `cluster=` (empty = all clusters) or match the provider's cluster. |
| Consumer keeps stale addresses forever | nacos down / push errors — Watch intentionally keeps the last snapshot | Restore nacos; check Warn `callback error` lines. |
| `UpdateWeight` errors "instance not registered yet" | called before Run registered | Call only after readiness (log line `registered ...`). |
| Config `weight=0` but instance still gets traffic | write-side normalization stores 1 | Use the runtime API `UpdateWeight(ctx, 0)` — only that path stores 0. |
| Instance not removed ~15s after crash | instance registered as persistent — not possible via this starter | All registrations are `Ephemeral: true`; check for a foreign writer to the same ip:port. |

Runtime logs carry the tag `_app_registry_nacos` (`log.RegisterAppTag("registry_nacos", "")`,
`starter.go:53`); tune verbosity via `logger.<name>.tag=_app_registry_nacos`.

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 18 (7 connection + 4 instance + 7 per discovery backend) |
| Required | 3 provider-side (`server`, `service-name`, `addr`) + 1 per discovery backend (`server`) |
| Quickstart external deps | 1 (nacos server; docker-gated in example) |
| "Watch out" entries | 6 |

Resolved 2026-08 (config-hygiene pass): dead `id` key deleted (Nacos identifies instances by
ip:port); `cluster` defaults aligned to `DEFAULT` on both sides (explicit empty still spans all
clusters); registrar/discovery namespace+group divergence now WARNs at startup
(warnRegistryDivergence). Remaining ledger items: config `weight=0` becomes 1 while API
`UpdateWeight(0)` drains — same key, opposite semantics by call path; no metrics on
registration/watch health (log-only observability).

# starter-registry-consul Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against the
starter source (`starter.go`, `registrar.go`, `config.go`, `registrar_test.go`) and the runnable
[example/](example/) (`check.sh` runs unit tests + a docker-compose Consul end-to-end boot).
**Consul's own semantics (agents, TTL checks, weights, catalog) are
[Consul documentation](https://developer.hashicorp.com/consul/docs)** — everything below is go-spring's
increment.

**Model**: config is NAMED BLOCKS — each `spring.registry.consul.<name>.*` block describes ONE
Consul agent and becomes ONE backend bean named `consul.<name>` (`center.go`). The bean implements
BOTH sides of the naming idiom: `discovery.Registrar` (write — collected by the `registryServer`
from the [starter-registry](../starter-registry) core, imported transitively, which registers into
EVERY configured center across backends) and `discovery.Discovery` (read — consumers cite the bean
name, e.g. `discovery=consul.main`; the bean is lazy). Read and write share the block's client, so
they can never diverge. There is no default/unnamed block. Registration activates only when
`spring.registry.service-name` is set — a pure consumer app configures only connection blocks and
registers nothing. Multi-center (dual registration, cross-backend mixes) is just more blocks. It
opens no port — the exported `gs.Server` (in the starter-registry core) exists purely to plug
registration into the app lifecycle.

---

## 1. Complete worked project

Two sides: a **provider** (this starter + a served endpoint) and a **consumer** (any discovery-aware
client starter resolving through a Consul-backed `discovery.Discovery`). File tree:

```
demo/
├── go.mod
├── main.go
├── provider.go
└── conf/
    └── app.properties
```

**Prerequisite** (single external dependency): a Consul dev agent —

```bash
docker run -d --name consul -p 127.0.0.1:8500:8500 hashicorp/consul:1.18 agent -dev -client=0.0.0.0
```

**go.mod**:

```
require (
    github.com/hashicorp/consul/api      latest
    go-spring.org/spring                 v1.3.x
    go-spring.org/starter-registry-consul latest
    go-spring.org/starter-redigo          latest   // any discovery-aware client starter
)
```

**main.go**:

```go
package main

import (
    _ "demo/conf"
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-consul"
)

func main() { gs.Run() }
```

**conf/app.properties** (verbatim from `example/conf/app.properties`):

```properties
spring.app.name=orders-provider

# One named block per Consul agent; each block becomes the backend bean
# "consul.<name>" serving both registration (via the starter-registry core)
# and discovery.
spring.registry.consul.main.address=127.0.0.1:8500
spring.registry.consul.main.ttl=10s                        # heartbeat at half this
spring.registry.consul.main.deregister-critical-after=30s  # crash auto-cleanup

# The advertised instance (backend-agnostic keys, shared by every center).
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

**Consumer side.** The starter ships the Consul discovery backend. With the provider config above
the block's bean (`consul.main`) already serves both halves — nothing more to configure. A client
starter cites the block's bean name:

```properties
spring.redis.demo.service-name=orders
spring.redis.demo.discovery=consul.main   # the block's bean name
```

A pure consumer app configures ONLY connection blocks (no `service-name`/`addr`) — it registers
nothing and just cites `discovery=consul.<name>`.

**Verify** (mirrors `example/check.sh`):

```bash
curl -fsS http://127.0.0.1:8500/v1/status/leader          # agent up (has a host:port)
go run .                                                  # logs: registered "orders" at 127.0.0.1:8080
curl -fsS 'http://127.0.0.1:8500/v1/health/service/orders?passing=true'
# -> one entry, Weights.Passing=100, Meta contains zone/version
```

Drain check: `curl -X PUT .../v1/agent/service/deregister/orders-127.0.0.1:8080` after stopping, or
flip the weight via the app's `UpdateWeight` (§2.3) and re-query — `Weights.Passing` becomes 0.

---

## 2. Assembly & timing

Timeline (`center.go`, `registrar.go`, and the starter-registry core's `starter.go`):

1. For each `${spring.registry.consul.<name>}` block the module (`OnProperty("spring.registry.consul")`,
   bound via `BindEach`) builds ONE `*api.Client` and probes the agent (`Catalog().Services`, 5s
   timeout) so a bad address fails startup here, once per block. The bean named `consul.<name>`
   exports BOTH `discovery.Registrar` and `discovery.Discovery` (lazy read half) on that client.
2. The starter-registry core provides `gs.Provide(NewServer).Name("registryServer")` conditioned
   on `spring.registry.service-name`; its `Registrars []discovery.Registrar` field slice-collects
   EVERY backend's registrar (consul, etcd, ... mixed). `Run` validates `service-name`/`addr` and
   ≥1 registrar **before** signalling readiness, waits `<-sig.TriggerAndWait()` (the ready-gate:
   registration happens only after every other server is up), then `Agent().ServiceRegister` into
   every center with a TTL check and an immediate `UpdateTTL(passing)` (`registrar.go`). Log:
   `registered %q at %s in N registry center(s)`.
3. A background heartbeat re-passes the check every `ttl/2` until deregister (`registrar.go`);
   crash safety never depends on Deregister: the process dies → check misses → Consul marks
   critical after one TTL → auto-dropped after `deregister-critical-after` (`config.go`).
4. Shutdown: `PreStop` deregisters from **every** center **first** — before the pre-stop delay and
   before any server stops, so discovery stops handing the instance out while in-flight requests
   drain (starter-registry `starter.go`). `Stop` is an idempotent fallback.

### 2.1 Registration write semantics

- Service ID = configured `id`, else `"<service-name>-<addr>"` — restarts replace the same entry
  (`registrar.go`).
- A negative (misconfigured) weight is normalized at write time: `<0 → 1`. 0 passes through as the
  drain signal on both write paths, so `weight=0` in config registers a drained instance
  (`registrar.go`, asserted by `registrar_test.go TestNormalizeWeight`).
- `addr` must be `host:port` with a numeric port; anything else fails Register
  (`registrar.go`).
- The entry carries `Weights{Passing: weight, Warning: 1}` and the TTL check
  (`registrar.go`). Consul routes no traffic to a zero-Passing-weight service — that is the
  drain mechanism.

### 2.2 WATCH path (consumer side)

`consulDiscovery` (`discovery_consul.go`) keeps each resolved service fresh with a Consul
blocking query: the first `Resolve` seeds a per-service cache from
`Health().Service(name, tag, passingOnly=true, nil)`, then a background long poll
(`WaitIndex` = last seen index, `WaitTime` 5m) delivers the fresh full snapshot on every catalog
change. A failed poll keeps the stale snapshot and retries after 5s; an agent index reset
(smaller `LastIndex`) drops the held index so the next query answers immediately. Later Resolves
are in-memory reads. Weight-0 filtering is the consumer pool's: `excludeDrained` drops
`Weight == 0` endpoints, falling back to the full set only when every endpoint is drained
(`cloud/loadbalance/pool.go`).

### 2.3 DRAIN path — UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)` (starter-registry `starter.go`) → each center's registrar looks up
the last registered value (error if never registered, `registrar.go`), maps negative weights to 1
but passes 0 through (`registrar.go`) → re-issues `ServiceRegister` with the new weight.
`ServiceRegister` is an **upsert on the service ID**, so the existing TTL check and its heartbeat
goroutine survive unchanged — the entry never leaves discovery (`registrar.go`). Consumers' next snapshot sees
`Weights.Passing = 0` → weight 0 endpoint → `excludeDrained` filters it from every load-balance
strategy. Restore with `UpdateWeight(ctx, 100)`.

---

## 3. Configuration reference

Two prefixes. `${spring.registry.consul.<name>.*}` binds ONE agent block each (`config.go`); the
block name is yours to choose and becomes the backend bean `consul.<name>`.
`${spring.registry.*}` binds the advertised instance (starter-registry `config.go`), shared by
every center.

| key | type | default | behavior | misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `spring.registry.consul.<name>.address` | string | — (required) | Consul HTTP API address; **setting a block activates it** | unset: block fails bind (`address is required`); wrong value: startup probe fails |
| `spring.registry.consul.<name>.scheme` | string | `http` | `http`/`https` for the agent API | mismatch with TLS deployment → connection errors |
| `spring.registry.consul.<name>.datacenter` | string | `` | datacenter to register into; empty = agent's | cross-dc mismatch → register/query against wrong dc |
| `spring.registry.consul.<name>.token` | string | `` | ACL token for requests | ACL-enabled cluster without token → 403s |
| `spring.registry.consul.<name>.namespace` | string | `` | Consul Enterprise namespace | silently wrong partition on CE |
| `spring.registry.consul.<name>.ttl` | duration | `15s` | TTL check interval; heartbeat at `ttl/2` | ⚠ too long delays crash detection to ~TTL + `deregister-critical-after` |
| `spring.registry.consul.<name>.deregister-critical-after` | duration | `1m` | auto-drop after check critical this long; `0` disables | ⚠ must exceed `ttl` or Consul may drop live instances on a hiccup |
| `spring.registry.service-name` | string | `` | logical service name clients resolve; **its presence is the registration intent signal** | empty: pure consumer; set with no block: Run error `... no registry center is configured` |
| `spring.registry.addr` | string | `` (required when registering) | advertised `host:port` | empty with service-name set: startup error; malformed: Register error (`registrar.go`) |
| `spring.registry.id` | string | `` | instance ID override; empty derives `<name>-<addr>` | ⚠ duplicate IDs across processes → one entry overwrites the other |
| `spring.registry.weight` | int | `100` | advertised weight; negative normalized to 1 at write time | 0 = drained; drains from startup as well as via `UpdateWeight(0)` |
| `spring.registry.metadata.*` | map[string]string | empty | instance attributes (zone, version, ...) passed through to discovery Metadata | — |

Discovery needs **no configuration**: each block's bean IS a `cloud/discovery.Discovery` named
`consul.<name>`; clients cite that bean name (`discovery=consul.main`). The bean is lazy — a
pure provider never pays for the read half; a pure consumer configures only connection blocks and
registers nothing. Multi-agent discovery is just multiple blocks — cite whichever agent you want
to read from. Per-call, `discovery.WithTag` still narrows a query by Consul service tag.

Mapping: `Service.Address:Port` → `Endpoint.Addr` (node address fallback when the service has
none), `Weights.Passing` → weight, `Meta` → metadata, `Meta["scheme"]` → `Endpoint.Scheme`;
passing-only queries keep unhealthy instances out of the snapshot.

---

## 4. Verification & failure drills

1. **Register → resolve**: boot the app, `curl '.../v1/health/service/orders?passing=true'` shows
   the entry with weight and metadata; the example's verify runner resolves the same registration
   back through the derived `consul` discovery bean and prints
   `discovered endpoint=... weight=... metadata=...` (`example/example.go`).
2. **Weight change propagation**: call `server.UpdateWeight(ctx, 0)` (inject the `gs.Server` named
   `registryServer`), re-query the catalog — `Weights.Passing=0`; a consumer pool stops picking the
   instance on its next snapshot. `UpdateWeight(ctx, 100)` restores it. Unit-asserted in
   `registrar_test.go:71 TestBuildRegistration_AdvertisesDrainWeight`.
3. **Graceful shutdown drain**: `kill <pid>` → PreStop deregisters before servers stop;
   `.../v1/health/service/orders` returns empty immediately.
4. **Instance loss (crash)**: `kill -9 <pid>` → no deregister runs; the check stops being passed and
   goes critical after ~one TTL (`ttl` default 15s; example uses 10s), then Consul auto-drops the
   entry after `deregister-critical-after` (example 30s). Watch with
   `curl -s .../v1/health/service/orders | jq '.[].Checks[0].Status'` — `passing` → `critical` → gone.
5. **Heartbeat death**: pause the process (`kill -STOP`) — same visible course as a crash; resume
   (`kill -CONT`) before the critical window and the next heartbeat re-passes the check with no
   re-registration needed.
6. **Bad address fail-fast**: set `address=127.0.0.1:9999`, boot → startup fails at the center
   probe with `registry-consul: startup probe failed for 127.0.0.1:9999` (`center.go`).
7. **Unregistered UpdateWeight**: calling `UpdateWeight` before Run registers returns
   `registry: instance not registered yet` (starter-registry `starter.go`).

All runtime logs carry the tag `_app_registry_consul` (`log.RegisterAppTag("registry_consul", "")`):
`creating consul registrar`, `registering service=...`, `registered %q at %s`, `deregister %q`
(Warn). Tune verbosity via `logger.<name>.tag=_app_registry_consul`.

Observability: registration and discovery emit OTel metrics through the global providers — a no-op unless `starter-otel` is imported. `register`, `deregister` and `update_weight` each produce a client span plus a `registry.operation.duration` record labelled `system`/`operation`/`service`/`status`; `registry.registration.attempts_total` counts attempts by `reason` and `status`; the `registry.instance.registered` gauge reads 1 while this instance is published and 0 while it is not, so a failed self-heal lands there instead of only in a log line. The discovery half reports every background cache sync to `discovery.sync_total` and keeps `discovery.cache.age_seconds` (seconds since the snapshot was last confirmed fresh), so a dead watch shows a climbing age rather than a silently stale address list. `reason` is `initial` for the first publish and `self_heal` for the background re-registration.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| startup error `...service-name} and ${spring.registry.addr} are required` | `service-name` set but `addr` unset | set both (starter-registry `starter.go`) |
| `register "orders" ... connection refused` | Consul agent unreachable / wrong address | start agent, fix `address` |
| entry appears then vanishes ~TTL later while app runs | heartbeat goroutine died or agent unreachable mid-run | check agent health; heartbeat logs nothing — watch the check status |
| consumer still sends traffic after `UpdateWeight(0)` | consumer snapshot stale (blocking-query interval) | re-query; check the consumer backend maps Weights to Endpoint.Weight |
| instance not dropped after crash | `deregister-critical-after=0` (disabled) | set a positive value > `ttl` |
| two processes, only one entry in catalog | derived ID collision (same name+addr) | set distinct `spring.registry.id` per instance |
| query with `?passing=true` empty though registered | check went critical (paused/dead process) | drills 4/5; verify `ttl`/heartbeat |
| `403` / `Permission denied` from agent | ACL enabled, no `token` | set `spring.registry.consul.token` |
| restart leaves stale duplicate entry | previous crash auto-drop not yet elapsed | wait `deregister-critical-after`, or deregister manually via API |

---

## 6. Design health

| metric | value |
|--------|-------|
| config keys | 7 per block (agent) + 5 instance (`spring.registry.*`) |
| required | 1 per block (`address`) + 2 at Run (`service-name`, `addr`) — registration only |
| quickstart external deps | 1 (Consul agent, docker) |
| "notes/gotchas" | 4 |

Suspect ledger:

- The blocking query's `WaitTime` (5m) and failure retry interval (5s) are hardcoded, not
  configuration keys.
- `Warning` weight hardcoded to 1 (`registrar.go`) — not configurable, undocumented in keys.
- Startup validation happens in Run, not bind time: an empty `service-name` fails only after the app
  is otherwise up.
- Drain relies on Consul's Passing-weight semantics; a consumer backend that ignores Weights
  silently breaks weight-0 drain — the shipped backend maps `Weights.Passing` → `Endpoint.Weight`,
  but the contract lives only in docs.

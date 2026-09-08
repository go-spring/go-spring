# starter-registry-consul Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against the
starter source (`starter.go`, `registrar.go`, `config.go`, `registrar_test.go`) and the runnable
[example/](example/) (`check.sh` runs unit tests + a docker-compose Consul end-to-end boot).
**Consul's own semantics (agents, TTL checks, weights, catalog) are
[Consul documentation](https://developer.hashicorp.com/consul/docs)** — everything below is go-spring's
increment.

**Activation**: the registrar server bean exists only when `spring.registry.consul.address` is set —
that key is the on/off switch (`starter.go`). The same center config derives a discovery backend bean
labeled `discovery-name` (default `consul`, `center.go`) on one shared Consul client, so one starter
serves both halves of the naming idiom; explicit `${spring.discovery.consul.<name>}` blocks add more
backends (§1 consumer side). It opens no port — it exports a `gs.Server` purely to plug registration
into the app lifecycle (`config.go:27-32`).

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

# Consul agent. Setting address activates the starter.
spring.registry.consul.address=127.0.0.1:8500
spring.registry.consul.ttl=10s                        # heartbeat at half this
spring.registry.consul.deregister-critical-after=30s  # crash auto-cleanup

# The advertised instance (backend-agnostic keys).
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

**Consumer side.** The starter ships the Consul discovery backend. With the provider config above
a backend bean labeled `consul` (the `discovery-name` default) is already derived from the same
center config and shared client — nothing more to configure. A client starter cites that label:

```properties
spring.redis.demo.service-name=orders
spring.redis.demo.discovery=consul   # the derived backend's label
```

Pure consumers (or a second agent) configure a standalone block under
`spring.discovery.consul.<name>` instead — see §3.

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

Timeline (all `starter.go`):

1. The center module (`center.go`) binds `${spring.registry.consul}`, builds the ONE shared
   `*api.Client`, and probes the agent (`Catalog().Services`, 5s timeout) so a bad address fails
   startup here. It also derives the `discovery-name`-labeled (default `consul`) discovery backend
   bean on the same client. `gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())`
   is conditioned on `spring.registry.consul.address` and nullable-injects the center
   (`starter.go`).
2. `Run` validates `service-name`/`addr` **before** signalling readiness (`starter.go:91-94`), waits
   `<-sig.TriggerAndWait()` (the ready-gate: registration happens only after every other server is
   up), then `Agent().ServiceRegister` with a TTL check and an immediate `UpdateTTL(passing)`
   (`starter.go:103-110`, `registrar.go:90-108`). Log: `registered %q at %s` (`log.TagAppDef`).
3. A background heartbeat re-passes the check every `ttl/2` until deregister
   (`registrar.go:177-193`); crash safety never depends on Deregister: the process dies → check
   misses → Consul marks critical after one TTL → auto-dropped after
   `deregister-critical-after` (`config.go:39-47`).
4. Shutdown: `PreStop` deregisters **first** — before the pre-stop delay and before any server stops,
   so discovery stops handing the instance out while in-flight requests drain (`starter.go:116-121`).
   `Stop`/`Stop` are idempotent fallbacks (`starter.go:123-135`, `registrar.go:195-211`).

### 2.1 Registration write semantics

- Service ID = configured `id`, else `"<service-name>-<addr>"` — restarts replace the same entry
  (`registrar.go:78-85`).
- Weight is normalized at write time: `<=0 → 1`, so "default" is never stored as 0 — 0 is reserved
  for the runtime drain signal (`registrar.go:90-96`, asserted by
  `registrar_test.go:49 TestRegister_NormalizesDefaultWeight`).
- `addr` must be `host:port` with a numeric port; anything else fails Register
  (`registrar.go:97-102`).
- The entry carries `Weights{Passing: weight, Warning: 1}` and the TTL check
  (`registrar.go:127-148`). Consul routes no traffic to a zero-Passing-weight service — that is the
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

`Server.UpdateWeight(ctx, 0)` (`starter.go:142-147`) → registrar looks up the last registered value
(error if never registered, `registrar.go:156-163`), maps negative weights to 1 but passes 0 through
(`registrar.go:164-166`) → re-issues `ServiceRegister` with the new weight. `ServiceRegister` is an
**upsert on the service ID**, so the existing TTL check and its heartbeat goroutine survive
unchanged — the entry never leaves discovery (`registrar.go:150-155`). Consumers' next snapshot sees
`Weights.Passing = 0` → weight 0 endpoint → `excludeDrained` filters it from every load-balance
strategy. Restore with `UpdateWeight(ctx, 100)`.

---

## 3. Configuration reference

Two prefixes. `${spring.registry.consul.*}` binds the agent connection (`config.go:22-48`);
`${spring.registry.*}` binds the advertised instance (`config.go:50-74`).

| key | type | default | behavior | misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `spring.registry.consul.address` | string | — (required) | Consul HTTP API address; **its presence activates the starter** | unset: starter inert; wrong value: first `Register` fails, Run returns error |
| `spring.registry.consul.scheme` | string | `http` | `http`/`https` for the agent API | mismatch with TLS deployment → connection errors |
| `spring.registry.consul.datacenter` | string | `` | datacenter to register into; empty = agent's | cross-dc mismatch → register/query against wrong dc |
| `spring.registry.consul.token` | string | `` | ACL token for requests | ACL-enabled cluster without token → 403s |
| `spring.registry.consul.namespace` | string | `` | Consul Enterprise namespace | silently wrong partition on CE |
| `spring.registry.consul.ttl` | duration | `15s` | TTL check interval; heartbeat at `ttl/2` | ⚠ too long delays crash detection to ~TTL + `deregister-critical-after` |
| `spring.registry.consul.deregister-critical-after` | duration | `1m` | auto-drop after check critical this long; `0` disables | ⚠ must exceed `ttl` or Consul may drop live instances on a hiccup |
| `spring.registry.consul.discovery-name` | string | `consul` | derives a discovery backend bean for this same agent under that label (shared client); empty disables | label colliding with another bean name fails loudly in the container |
| `spring.registry.service-name` | string | `` (required) | logical service name clients resolve | empty: startup error from Run validation (`starter.go:92-94`) |
| `spring.registry.addr` | string | `` (required) | advertised `host:port` | empty: startup error; malformed: Register error (`registrar.go:97-102`) |
| `spring.registry.id` | string | `` | instance ID override; empty derives `<name>-<addr>` | ⚠ duplicate IDs across processes → one entry overwrites the other |
| `spring.registry.weight` | int | `0` | advertised weight; `<=0` normalized to 1 at write time | 0 does **not** drain here (normalized); drain is `UpdateWeight(0)` only |
| `spring.registry.metadata.*` | map[string]string | empty | instance attributes (zone, version, ...) passed through to discovery Metadata | — |

Discovery blocks, one per backend under `${spring.discovery.consul.<name>.*}` (bean name = the
label clients cite). `address` empty = inherit the center client (no destructor of its own);
non-empty = own client plus startup probe:

| key | type | default | behavior | misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `address` | string | `` | empty INHERITS the `spring.registry.consul` center connection | empty with no center configured → bean construction error |
| `scheme` | string | `http` | `http`/`https` for the agent API | mismatch with TLS deployment → connection errors |
| `datacenter` | string | `` | datacenter to query; empty = agent's | cross-dc mismatch → queries against wrong dc |
| `token` | string | `` | ACL token for queries | ACL-enabled cluster without token → 403s |
| `namespace` | string | `` | Consul Enterprise namespace | silently wrong partition on CE |
| `tag` | string | `` | Consul service tag narrowing every query; `discovery.WithTag` overrides per call | wrong tag → empty snapshot |

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
   `registry-consul: instance not registered yet` (`starter.go:143-145`, unit-tested at
   `registrar_test.go:62`).

All runtime logs carry `log.TagAppDef`: `creating consul registrar`, `registering service=...`,
`registered %q at %s`, `deregister %q` (Warn). No metrics/traces are emitted by this starter.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| startup error `...service-name} and ${spring.registry.addr} are required` | either key unset | set both (`starter.go:92-94`) |
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
| config keys | 20 (9 agent + 5 instance + 6 discovery block) |
| required | 3 (`address`, `service-name`, `addr`) |
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

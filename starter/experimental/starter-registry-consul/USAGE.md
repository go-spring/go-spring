# starter-registry-consul Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against the
starter source (`starter.go`, `registrar.go`, `config.go`, `registrar_test.go`) and the runnable
[example/](example/) (`check.sh` runs unit tests + a docker-compose Consul end-to-end boot).
**Consul's own semantics (agents, TTL checks, weights, catalog) are
[Consul documentation](https://developer.hashicorp.com/consul/docs)** — everything below is go-spring's
increment.

**Activation**: the registrar server bean exists only when `spring.registry.consul.address` is set —
that key is the on/off switch (`starter.go:61`). This starter is **register-side only**: it advertises
this instance to Consul; it ships no client-side discovery backend (see §1 consumer note).
It opens no port — it exports a `gs.Server` purely to plug registration into the app lifecycle
(`config.go:27-32`).

---

## 1. Complete worked project

Two sides: a **provider** (this starter + a served endpoint) and a **consumer** (any discovery-aware
client starter resolving through a Consul-backed `discovery.Discovery`). File tree:

```
demo/
├── go.mod
├── main.go
├── provider.go
├── consumer_side/
│   └── discovery.go     # Consul-backed Discovery (see note below)
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

**Consumer side.** This starter does not ship a Consul discovery backend. The intended seam is
`cloud/discovery`: register one Consul-backed `Discovery` under a name, then any client starter's
`discovery:` field resolves through it (the reference implementation is
`examples/fullstack/internal/consuldisc`, ~60 lines):

```go
// consumer_side/discovery.go — call once at startup.
func registerConsulDiscovery(name, addr string) error {
    b, err := consuldisc.New(addr) // wraps Health().Service + blocking-query Watch
    if err != nil { return err }
    discovery.RegisterDiscovery(name, b)
    return nil
}
```

```properties
spring.redis.demo.service-name=orders
spring.redis.demo.discovery=consul   # matches the name registered above
```

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

1. `gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())` conditioned on
   `spring.registry.consul.address` (`starter.go:56-62`). The Consul client does not dial eagerly —
   a bad address surfaces on the first `Register`, not at construction (`starter.go:64-67`).
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
   `Stop`/`StopContext` are idempotent fallbacks (`starter.go:123-135`, `registrar.go:195-211`).

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

This starter has no watch; consumers use the `cloud/discovery` seam. The reference Consul backend
(`examples/fullstack/internal/consuldisc`) uses Consul blocking queries: each catalog change returns
a fresh full snapshot, which the `discovery.Resolver`/loadbalance `Pool` consumes. Change detection
and weight-0 filtering are the pool's: `excludeDrained` drops `Weight == 0` endpoints, falling back
to the full set only when every endpoint is drained (`cloud/loadbalance/pool.go:93-134`).

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
| `spring.registry.service-name` | string | `` (required) | logical service name clients resolve | empty: startup error from Run validation (`starter.go:92-94`) |
| `spring.registry.addr` | string | `` (required) | advertised `host:port` | empty: startup error; malformed: Register error (`registrar.go:97-102`) |
| `spring.registry.id` | string | `` | instance ID override; empty derives `<name>-<addr>` | ⚠ duplicate IDs across processes → one entry overwrites the other |
| `spring.registry.weight` | int | `0` | advertised weight; `<=0` normalized to 1 at write time | 0 does **not** drain here (normalized); drain is `UpdateWeight(0)` only |
| `spring.registry.metadata.*` | map[string]string | empty | instance attributes (zone, version, ...) passed through to discovery Metadata | — |

---

## 4. Verification & failure drills

1. **Register → resolve**: boot the app, `curl '.../v1/health/service/orders?passing=true'` shows
   the entry with weight and metadata; the example's built-in verifier prints
   `registered addr=... meta=...` (`example/example.go:83-97`).
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
6. **Bad address fail-fast**: set `address=127.0.0.1:9999`, boot → Run fails with
   `registry: register "orders" ... connection refused` (`starter.go:106-109`).
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
| config keys | 12 (7 agent + 5 instance) |
| required | 3 (`address`, `service-name`, `addr`) |
| quickstart external deps | 1 (Consul agent, docker) |
| "notes/gotchas" | 4 |

Suspect ledger:

- No shipped consumer side: every user must hand-roll a Consul `discovery.Discovery`
  (~60 lines, cf. `examples/fullstack/internal/consuldisc`) — candidate for a
  starter-discovery-consul sibling.
- Drain relies on Consul's Passing-weight semantics; a consumer backend that ignores Weights
  silently breaks weight-0 drain — the contract lives only in docs.
- `Warning` weight hardcoded to 1 (`registrar.go:141`) — not configurable, undocumented in keys.
- Startup validation happens in Run, not bind time: an empty `service-name` fails only after the app
  is otherwise up.

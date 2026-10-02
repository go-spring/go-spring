# starter-discovery-zookeeper Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `registry.go`, `discovery.go`, `config.go`,
`starter_test.go`, `registry_test.go`, `discovery_test.go`), the registration core (`../starter-discovery/starter.go`,
`../starter-discovery/config.go`), the `cloud/discovery` seam (`cloud/discovery/registry.go`,
`cloud/loadbalance/pool.go`) and the runnable [example/](example/) (`example/check.sh` runs unit
tests + a docker-compose ZooKeeper end-to-end boot). **ZooKeeper's own semantics (sessions,
ephemeral znodes, watchers, digest auth) are
[ZooKeeper documentation](https://zookeeper.apache.org/doc/current/zookeeperProgrammers.html)** —
everything below is go-spring's increment.

**Activation**: each `spring.discovery.zookeeper.<name>` block is one discovery center — one shared
session, one lifecycle (a startup probe only when `ping=true`; `starter.go`). The bean named `zookeeper.<name>` exports
BOTH `discovery.Registry` (collected by the cloud/discovery core when
`spring.discovery.service-name` is set — a pure consumer app registers nothing) and
`discovery.Discovery` (cited by bean name, so a pure provider never pays for the read half). This
starter serves **both sides**: it advertises this instance to ZooKeeper as an ephemeral znode AND
ships the client-side discovery backend (see §1 consumer note). It opens no port — registration
plugs into the app lifecycle via the `discoveryServer` from the core. No default/unnamed block: the
block name is mandatory and becomes part of the bean name.

---

## 1. Complete worked project

Two sides: a **provider** (this starter + a served endpoint) and a **consumer** (any
discovery-aware client starter resolving through a ZooKeeper-backed `discovery.Discovery`).
File tree:

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**Prerequisite** (single external dependency): a ZooKeeper node — verbatim from
`example/docker-compose.yml` (zookeeper:3.9 on 127.0.0.1:2181):

```bash
docker compose -f example/docker-compose.yml up -d
# readiness (four-letter word; what check.sh polls):
( echo ruok; sleep 1 ) | nc 127.0.0.1 2181        # -> imok
```

**go.mod**:

```
require (
    github.com/go-zookeeper/zk           v1.0.4
    go-spring.org/spring                 v1.3.x
    go-spring.org/starter-discovery-zookeeper latest
    go-spring.org/starter-redigo         latest   // any discovery-aware client starter
)
```

**main.go** (mirrors `example/main.go`):

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-discovery-zookeeper"
)

func main() { gs.Run() }
```

**conf/app.properties** (verbatim from `example/conf/app.properties`):

```properties
spring.app.name=discovery-zookeeper-example

# ZooKeeper ensemble to register into. One NAMED block per ensemble; setting
# servers activates that block (the bean name is "zookeeper." + the block name).
spring.discovery.zookeeper.main.servers=127.0.0.1:2181
spring.discovery.zookeeper.main.session-timeout=10s
spring.discovery.zookeeper.main.base-path=/services

# The instance to advertise (backend-agnostic; switching discovery backends is a
# blank-import swap, not a config change).
spring.discovery.service-name=orders
spring.discovery.addr=127.0.0.1:8080
spring.discovery.weight=100
spring.discovery.metadata.zone=cn-north
spring.discovery.metadata.version=v1
```

**Consumer side.** This starter ships the ZooKeeper discovery backend
(`discovery.go`): the `zookeeper.<name>` bean lists the children of
`<base-path>/<service-name>` (each child name is the instance id), `Get`s each znode's data,
decodes the self-describing `instanceValue` JSON, maps to
`discovery.Endpoint{Addr, Weight, Metadata}`, and keeps the snapshot fresh with
`ChildrenW`/`GetW` watchers (see §2.3 for the watch loop shape). There is no discovery config:
the backend IS the block's bean, sharing the block's connection and base-path — read and write
can never diverge. A pure consumer sets only the connection block (no `service-name`) and
registers nothing; a pure provider never cites the bean and never pays for it.

```properties
# pure consumer: connection block only, registration stays off
spring.discovery.zookeeper.main.servers=10.0.0.9:2181

spring.redis.demo.service-name=orders
spring.redis.demo.discovery=zookeeper.main   # the backend bean's name
```

**Verify** (mirrors `example/main.go` and `example/check.sh`):

```bash
go run .                                   # logs: registered "orders" at 127.0.0.1:8080
# the example self-verifies and prints: discovered endpoint=127.0.0.1:8080 weight=100 metadata=map[version:v1 zone:cn-north]
```

Interactive verification with the ZooKeeper shell:

```bash
docker exec -it starter-discovery-zookeeper zkCli.sh
[zk: ...] ls /services/orders                       # -> [orders-127.0.0.1:8080]
[zk: ...] get /services/orders/orders-127.0.0.1:8080
#   {"service_name":"orders","addr":"127.0.0.1:8080","weight":100,
#    "metadata":{"version":"v1","zone":"cn-north"}}
#   ... ctime/ephemeralOwner != 0 marks it ephemeral
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline (`starter.go` / `registry.go` / the core's `starter.go`)

```
blank-import starter-discovery-zookeeper
  ├─ per block ${spring.discovery.zookeeper.<name>}: Provide(newZkBackend)
  │    Name("zookeeper."+name).Export(As[discovery.Discovery], As[discovery.Registry])
  │    condition: gs.OnProperty("spring.discovery.zookeeper")           [starter.go]
  └─ import starter-discovery (core, exactly once per process)
       └─ gs.Provide(NewServer).Name("discoveryServer").Export(gs.As[gs.Server]())
            condition: gs.OnProperty("spring.discovery.service-name")
gs.Run()
  ├─ conf.BindEach over spring.discovery.zookeeper.* → one ZookeeperConfig per block
  ├─ newZkBackend per block: zk.Connect(servers, session-timeout) + digest AddAuth
  │    when set; ping=true → fail-fast probe Exists("/") — blocks until the
  │    session connects, so an unreachable ensemble fails STARTUP, not the first Register   [starter.go]
  ├─ discoveryServer collects every backend's Registry via []discovery.Registry
  │    slice injection (across ALL backends — zookeeper, nacos, ...)
  ├─ Run: validate service-name/addr AND ≥1 registry BEFORE readiness
  ├─ wait <-sig.TriggerAndWait() — the ready-gate: registration only after
  │    every other server is up
  ├─ Register per center: ensureParents (persistent dirs, on demand) + Create
  │    ephemeral znode at <base-path>/<service>/<id>                   [registry.go]
  ├─ Run blocks on <ctx.Done() — no heartbeat goroutine: liveness IS the session
  └─ on SIGTERM: PreStop → Deregister from EVERY center FIRST (before the pre-stop
       delay and any server stops) so discovery stops handing the instance out
       while in-flight requests drain; Stop deregisters again as idempotent
       fallbacks (ErrNoNode tolerated)
```

### 2.2 Session & ephemeral node mechanics — the crash-safety contract

- Registration is `Create(path, val, zk.FlagEphemeral, ...)` — one ephemeral znode per instance,
  owned by that block's client session (`registry.go`).
- An ephemeral node lives only as long as the session: if the process dies without deregistering,
  ZooKeeper removes the node once the session expires — self-healing with no reaper, no TTL
  heartbeat goroutine, nothing to configure beyond `session-timeout`. Correctness never depends
  on Deregister running.
- The zk library reconnects and re-establishes the session automatically on transient network
  loss; if it cannot within the session timeout the session expires and **the node silently
  disappears** — see the drill in §4 and the troubleshooting row "instance vanished while the
  app was running". The starter does not re-register after session loss (no reconnect callback
  is wired).
- Restart replacement: an ephemeral node from a previous session may linger briefly; Register
  tolerates `ErrNodeExists` by deleting and re-creating the node, so a restart refreshes the
  entry instead of failing (`registry.go`).

### 2.3 WATCH path (consumer side)

The shipped backend's watch loop (`discovery.go`) mirrors how the pool consumes
snapshots:

1. `ChildrenW(basePath/service)` fires on any instance join/leave (including ephemeral deletion
   on session expiry — ZooKeeper's own watch event, not our code).
2. The backend lists children and `Get`s each one's data (an instance-id child per instance);
   `GetW` covers in-place data rewrites (weight changes).
3. Each event yields a fresh full `[]discovery.Endpoint` snapshot, stored into the backend's
   internal per-service cache. The backend does not diff; every stored snapshot is
   authoritative.
4. A discovery `Loader` re-reads the backend snapshot on every call; the loadbalance `Pool`
   picks from that set.

### 2.4 DRAIN path — UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)` on the `discoveryServer` bean (the core's `starter.go`) broadcasts
to EVERY registry; the zk side (`registry.go`) verifies the node still exists (error
`update weight for unregistered instance` otherwise), maps negative weights to 1 but passes 0
through, then rewrites the payload with `conn.Set(path, val, -1)` — an **in-place data set, not
delete+recreate**. Design rationale (source comment): the ephemeral owner and the watchers are
undisturbed — the session keeps owning the node, and a `GetW` consumer simply observes the new
value. Weight 0 serializes as an *omitted* `weight` field (`json:"weight,omitempty"`), which
readers reconstruct as 0. Consumers' next snapshot carries `Endpoint.Weight == 0` →
`excludeDrained` drops it from every load-balance strategy, falling back to the full set only
when every endpoint is drained. Restore with `UpdateWeight(ctx, 100)`.

At initial Register a negative weight is normalized to 1; **0 passes through as the drain signal**,
so `weight=0` in config registers an already-drained instance.

---

## 3. Per-key behavior reference

Connection keys bind per block under `${spring.discovery.zookeeper.<name>}` (`config.go`);
instance keys under `${spring.discovery}` (`../starter-discovery/config.go`). There is no
discovery config: the backend bean IS the block's bean, named `zookeeper.<name>` (`starter.go`).

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.discovery.zookeeper.<n>.servers` | []string | — (required) | ensemble members; **its presence activates the block** | unset everywhere: starter inert, no error; wrong value with `ping=true`: startup fails at the probe (`discovery-zookeeper: startup probe failed`) |
| `spring.discovery.zookeeper.<n>.session-timeout` | duration | `10s` | zk session timeout; bounds the `ping=true` startup probe AND how long a crashed process's ephemeral node lingers | ⚠ too long delays crash-driven instance removal; too short risks session expiry on GC pauses / transient partitions → silent deregistration |
| `spring.discovery.zookeeper.<n>.base-path` | string | `/services` | persistent parent znode; trailing `/` trimmed; service dirs created on demand | consumers must list the same path; a mismatch is invisible to the provider |
| `spring.discovery.zookeeper.<n>.ping` | bool | `false` | Probes the ensemble once at construction (an `Exists("/")` call that blocks until the session connects) and fails startup if unreachable. Off by default so an ensemble that is not up yet does not block boot; connectivity surfaces on first use. | — |
| `spring.discovery.zookeeper.<n>.health` | bool | `true` | Contributes a `health.Indicator` bean named `discovery-zookeeper:<name>` probing the ensemble with one `Exists("/")` call (same check as the startup probe). Only instantiated when a collector (e.g. starter-actuator) autowires it. | `false` → the ensemble's health is invisible to readiness probes |
| `spring.discovery.zookeeper.<n>.username` | string | `` | digest auth, applied via `AddAuth("digest", user:pass)`; set together with `password` | ⚠ one set, one empty → auth scheme error / ACL-denied writes |
| `spring.discovery.zookeeper.<n>.password` | string | `` | digest password (see above) | as above |
| `spring.discovery.service-name` | string | `` | logical name; becomes the znode directory and what discovery clients resolve. **Registration intent signal**: unset with blocks configured → a valid pure consumer | set with `addr` empty: Run returns `discovery: ${spring.discovery.service-name} and ${spring.discovery.addr} are required` — after the app is otherwise up |
| `spring.discovery.addr` | string | `` (required when registering) | advertised `host:port`; never guessed | empty: same Run error; malformed is NOT validated (no numeric-port check unlike consul) — stored verbatim, consumers fail to dial |
| `spring.discovery.id` | string | `` | instance-id override; empty derives `<service-name>-<addr>` so restarts replace the same znode (`registry.go`) | ⚠ duplicate ids across processes → one process's Register deletes and replaces the other's node |
| `spring.discovery.weight` | int | `100` | advertised LB weight; negative normalized to 1 at write time | 0 = drained, honored from startup as well as via `UpdateWeight(0)` |
| `spring.discovery.metadata.*` | map[string]string | empty | arbitrary attributes (zone, version, ...) stored in the znode payload and passed through to discovery Metadata | — |

Two blocks with the same `<name>` fail loudly in the container (duplicate bean name); block names
across backends never collide (the bean name carries the backend type, e.g. `zookeeper.main` vs
`nacos.main`).

---

## 4. Verification & fault drills

All zk-side checks work with `zkCli.sh` inside the container (see §1) or any zk client.

1. **Register → resolve**: boot the example — it self-verifies by listing `/services/orders` and
   printing `discovered endpoint=... weight=... metadata=...`; `check.sh` greps exactly that marker. In zkCli:
   `ls /services/orders` shows one child named `orders-127.0.0.1:8080`; `get` shows the JSON
   payload with `"weight":100` and an `ephemeralOwner != 0` (the ephemeral marker).
2. **Drain via UpdateWeight(0) + restore**: inject the `gs.Server` named `discoveryServer`, call
   `server.UpdateWeight(ctx, 0)`, then in zkCli `get /services/orders/orders-127.0.0.1:8080` —
   the payload now has **no `weight` field** (omitted at 0); the node still exists
   (ephemeralOwner unchanged — it was `Set`, not recreated). A consumer pool's next snapshot
   excludes it. `UpdateWeight(ctx, 100)` restores `"weight":100`. Calling it before Run
   registers returns `discovery: instance not registered yet`.
3. **Graceful shutdown deregister**: the example SIGTERMs itself after verifying — PreStop
   deregisters before servers stop; `ls /services/orders` returns empty immediately after exit.
   `Stop` runs Deregister again: idempotent, `ErrNoNode` tolerated.
4. **Crash / session-expiry instance loss**: boot the example in manual mode
   (`go run . -manual` — server stays up), then `kill -9 <pid>` → no Deregister runs; the
   ephemeral node disappears when the session expires, ~one `session-timeout` (10s in the
   example) later — no critical-marking phase, no reaper config, unlike TTL-based registries.
   Watch it vanish: `zkCli.sh ls -w /services/orders` or poll `get` until `NoNode`. A consumer's
   `ChildrenW` fires on the deletion and its next snapshot drops the endpoint.
5. **Bad ensemble fail-fast**: set `servers=127.0.0.1:9999` on the block and `ping=true`, boot →
   startup fails with `discovery-zookeeper: startup probe failed` — by design, instead of surfacing
   on the first Register.
6. **Restart replacement**: kill -9, restart immediately (before the old session expires) —
   Register succeeds despite the lingering old ephemeral node (delete + recreate); zkCli shows
   exactly one child.
7. **Dual write (two blocks)**: add a second block
   (`spring.discovery.zookeeper.dr.servers=...`) — the same instance appears under both
   ensembles (one publication fanned out), and the startup log says `in 2 discovery center(s)`.

Runtime logs carry the tag `_app_discovery_zookeeper`
(`log.RegisterAppTag("discovery_zookeeper", "")`), tuned via
`logger.<name>.tag=_app_discovery_zookeeper`; the core's `_app_discovery` tag carries the
shared registration lifecycle lines. Observability: registration and discovery emit OTel metrics through the global providers — a no-op unless `starter-otel` is imported. `register`, `deregister` and `update_weight` each produce a client span plus a `discovery.operation.duration` record labelled `system`/`operation`/`service`/`status`; `discovery.registration.attempts_total` counts attempts by `reason` and `status`; the `discovery.instance.registered` gauge reads 1 while this instance is published and 0 while it is not, so a failed self-heal lands there instead of only in a log line. The discovery half reports every background cache sync to `discovery.sync_total` and keeps `discovery.cache.age_seconds` (seconds since the snapshot was last confirmed fresh), so a dead watch shows a climbing age rather than a silently stale address list. `reason` is `initial` for the first publish and `self_heal` for the background re-registration. A session loss is still logged by the monitor, and now also lands in the metrics above.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| starter inert, nothing registered | no `spring.discovery.zookeeper.*` block anywhere | configure a named block — its `servers` is the activation switch |
| startup fails `startup probe failed` (only with `ping=true`) | ensemble unreachable / wrong servers | start ZooKeeper, fix the block's `servers`; the probe blocks up to `session-timeout` |
| startup error `service-name and addr are required` | either instance key unset while registering | set both — note this fires at Run, after other servers are already up |
| startup aborts "... but no discovery center is configured" | `service-name` set yet no connection block anywhere | add at least one `spring.discovery.<backend>.<name>` block |
| instance vanished while the app was running | session expired (long GC pause, network partition, session-timeout too low); zk removed the ephemeral node and the starter never re-registers | raise `session-timeout`; restart the process; watch for reconnect gaps in the zk client logs |
| node lingers long after `kill -9` | session not yet expired — removal takes up to one `session-timeout` | wait it out or lower `session-timeout`; do not add a reaper, the mechanism is ephemeral-by-design |
| consumer still sends traffic after `UpdateWeight(0)` | consumer snapshot stale (watch not yet fired) or backend ignores the omitted-weight field and defaults it to 1 | re-read the node; ensure the backend decodes absent `weight` as 0, not 1 |
| two processes, only one node | derived id collision (same name+addr) — later Register deletes and replaces the earlier node | set distinct `spring.discovery.id` per instance |
| `UpdateWeight` errors `update weight for unregistered instance` | node gone (session expired) or called before Run | re-register by restarting, or call after readiness |
| ACL / auth errors on write | ensemble requires digest auth, `username`/`password` unset or half-set | set both together |
| restart failed `create ... ErrNodeExists`-style replace error | replace raced a concurrent registrant on the same path | distinct ids per instance fixes it |

---

## 6. Design health

| metric | value |
|--------|-------|
| config keys | 10 (5 per-block connection + 5 instance) |
| required | 1 per block (`servers`) + 2 when registering (`service-name`, `addr`) |
| quickstart external deps | 1 (ZooKeeper, docker) |
| "notes/gotchas" | 4 (session-expiry silence; weight normalization; restart replacement; id collision) |

Suspect ledger (kept from the previous edition, plus new findings):

1. ~~`spring.discovery.*` instance keys bound in this one starter~~ RESOLVED 2026-09: the
   `${spring.discovery}` identity block now lives in the shared cloud/discovery core
   (`RegistrationConfig`), so every backend starter binds it exactly once.
2. No re-registration after session loss: the zk library reconnects transparently, but once the
   session has expired the ephemeral node is gone and the running process never notices — the
   instance silently disappears from discovery until restart. A SessionW/State-driven
   re-register loop is the candidate fix.
3. ~~No shipped consumer side~~ RESOLVED: `discovery.go` ships the backend — since the
   2026-09 multi-discovery named-block redesign it IS the block's bean (`zookeeper.<name>`),
   implementing both `discovery.Registry` and `discovery.Discovery`; registration moved into
   the shared cloud/discovery core.
4. `addr` is not validated for `host:port` shape at write time (consul validates a numeric port);
   a malformed value is stored verbatim and only fails on the consumer dial.
5. Startup validation happens in Run, not bind time — an empty `service-name` fails only after
   the app is otherwise up (still a latency-of-failure smell).

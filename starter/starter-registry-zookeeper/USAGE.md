# starter-registry-zookeeper Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `registrar.go`, `center.go`, `discovery_zookeeper.go`,
`config.go`, `registrar_test.go`), the registration core (`../starter-registry/starter.go`,
`../starter-registry/config.go`), the `cloud/discovery` seam (`cloud/discovery/registrar.go`,
`cloud/loadbalance/pool.go`) and the runnable [example/](example/) (`example/check.sh` runs unit
tests + a docker-compose ZooKeeper end-to-end boot). **ZooKeeper's own semantics (sessions,
ephemeral znodes, watchers, digest auth) are
[ZooKeeper documentation](https://zookeeper.apache.org/doc/current/zookeeperProgrammers.html)** —
everything below is go-spring's increment.

**Activation**: each `spring.registry.zookeeper.<name>` block is one registry center — one shared
session, one startup probe, one lifecycle (`center.go`). The bean named `zookeeper.<name>` exports
BOTH `discovery.Registrar` (collected by the starter-registry core when
`spring.registry.service-name` is set — a pure consumer app registers nothing) and
`discovery.Discovery` (cited by bean name, so a pure provider never pays for the read half). This
starter serves **both sides**: it advertises this instance to ZooKeeper as an ephemeral znode AND
ships the client-side discovery backend (see §1 consumer note). It opens no port — registration
plugs into the app lifecycle via the `registryServer` from the core. No default/unnamed block: the
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
    go-spring.org/starter-registry-zookeeper latest
    go-spring.org/starter-redigo         latest   // any discovery-aware client starter
)
```

**main.go** (mirrors `example/example.go`):

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-zookeeper"
)

func main() { gs.Run() }
```

**conf/app.properties** (verbatim from `example/conf/app.properties`):

```properties
spring.app.name=registry-zookeeper-example

# ZooKeeper ensemble to register into. One NAMED block per ensemble; setting
# servers activates that block (the bean name is "zookeeper." + the block name).
spring.registry.zookeeper.main.servers=127.0.0.1:2181
spring.registry.zookeeper.main.session-timeout=10s
spring.registry.zookeeper.main.base-path=/services

# The instance to advertise (backend-agnostic; switching registry backends is a
# blank-import swap, not a config change).
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

**Consumer side.** This starter ships the ZooKeeper discovery backend
(`discovery_zookeeper.go`): the `zookeeper.<name>` bean lists the children of
`<base-path>/<service-name>` (each child name is the instance id), `Get`s each znode's data,
decodes the self-describing `instanceValue` JSON, maps to
`discovery.Endpoint{Addr, Weight, Metadata}`, and keeps the snapshot fresh with
`ChildrenW`/`GetW` watchers (see §2.3 for the watch loop shape). There is no discovery config:
the backend IS the block's bean, sharing the block's connection and base-path — read and write
can never diverge. A pure consumer sets only the connection block (no `service-name`) and
registers nothing; a pure provider never cites the bean and never pays for it.

```properties
# pure consumer: connection block only, registration stays off
spring.registry.zookeeper.main.servers=10.0.0.9:2181

spring.redis.demo.service-name=orders
spring.redis.demo.discovery=zookeeper.main   # the backend bean's name
```

**Verify** (mirrors `example/example.go` and `example/check.sh`):

```bash
go run .                                   # logs: registered "orders" at 127.0.0.1:8080
# the example self-verifies and prints: registered node=orders-127.0.0.1:8080 value={...}
```

Interactive verification with the ZooKeeper shell:

```bash
docker exec -it starter-registry-zookeeper zkCli.sh
[zk: ...] ls /services/orders                       # -> [orders-127.0.0.1:8080]
[zk: ...] get /services/orders/orders-127.0.0.1:8080
#   {"service_name":"orders","addr":"127.0.0.1:8080","weight":100,
#    "metadata":{"version":"v1","zone":"cn-north"}}
#   ... ctime/ephemeralOwner != 0 marks it ephemeral
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline (`center.go` / `registrar.go` / the core's `starter.go`)

```
blank-import starter-registry-zookeeper
  ├─ per block ${spring.registry.zookeeper.<name>}: Provide(newZkBackend)
  │    Name("zookeeper."+name).Export(As[discovery.Discovery], As[discovery.Registrar])
  │    condition: gs.OnProperty("spring.registry.zookeeper")           [center.go]
  └─ import starter-registry (core, exactly once per process)
       └─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
            condition: gs.OnProperty("spring.registry.service-name")
gs.Run()
  ├─ conf.BindEach over spring.registry.zookeeper.* → one ZookeeperConfig per block
  ├─ newZkBackend per block: zk.Connect(servers, session-timeout) + digest AddAuth
  │    when set + fail-fast probe Exists("/") — blocks until the session connects,
  │    so an unreachable ensemble fails STARTUP, not the first Register   [center.go]
  ├─ registryServer collects every backend's Registrar via []discovery.Registrar
  │    slice injection (across ALL backends — zookeeper, nacos, ...)
  ├─ Run: validate service-name/addr AND ≥1 registrar BEFORE readiness
  ├─ wait <-sig.TriggerAndWait() — the ready-gate: registration only after
  │    every other server is up
  ├─ Register per center: ensureParents (persistent dirs, on demand) + Create
  │    ephemeral znode at <base-path>/<service>/<id>                   [registrar.go]
  ├─ Run blocks on <ctx.Done() — no heartbeat goroutine: liveness IS the session
  └─ on SIGTERM: PreStop → Deregister from EVERY center FIRST (before the pre-stop
       delay and any server stops) so discovery stops handing the instance out
       while in-flight requests drain; Stop deregisters again as idempotent
       fallbacks (ErrNoNode tolerated)
```

### 2.2 Session & ephemeral node mechanics — the crash-safety contract

- Registration is `Create(path, val, zk.FlagEphemeral, ...)` — one ephemeral znode per instance,
  owned by that block's client session (`registrar.go`).
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
  entry instead of failing (`registrar.go`).

### 2.3 WATCH path (consumer side)

The shipped backend's watch loop (`discovery_zookeeper.go`) mirrors how the pool consumes
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

`Server.UpdateWeight(ctx, 0)` on the `registryServer` bean (the core's `starter.go`) broadcasts
to EVERY registrar; the zk side (`registrar.go`) verifies the node still exists (error
`update weight for unregistered instance` otherwise), maps negative weights to 1 but passes 0
through, then rewrites the payload with `conn.Set(path, val, -1)` — an **in-place data set, not
delete+recreate**. Design rationale (source comment): the ephemeral owner and the watchers are
undisturbed — the session keeps owning the node, and a `GetW` consumer simply observes the new
value. Weight 0 serializes as an *omitted* `weight` field (`json:"weight,omitempty"`), which
readers reconstruct as 0. Consumers' next snapshot carries `Endpoint.Weight == 0` →
`excludeDrained` drops it from every load-balance strategy, falling back to the full set only
when every endpoint is drained. Restore with `UpdateWeight(ctx, 100)`.

At initial Register the weight is normalized `<=0 → 1`, so "default" is never stored as 0 — 0
is reserved for the runtime drain signal, only reachable through `UpdateWeight`.

---

## 3. Per-key behavior reference

Connection keys bind per block under `${spring.registry.zookeeper.<name>}` (`config.go`);
instance keys under `${spring.registry}` (`../starter-registry/config.go`). There is no
discovery config: the backend bean IS the block's bean, named `zookeeper.<name>` (`center.go`).

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.registry.zookeeper.<n>.servers` | []string | — (required) | ensemble members; **its presence activates the block** | unset everywhere: starter inert, no error; wrong value: startup fails at the probe (`registry-zookeeper: startup probe failed`) |
| `spring.registry.zookeeper.<n>.session-timeout` | duration | `10s` | zk session timeout; bounds the startup probe AND how long a crashed process's ephemeral node lingers | ⚠ too long delays crash-driven instance removal; too short risks session expiry on GC pauses / transient partitions → silent deregistration |
| `spring.registry.zookeeper.<n>.base-path` | string | `/services` | persistent parent znode; trailing `/` trimmed; service dirs created on demand | consumers must list the same path; a mismatch is invisible to the provider |
| `spring.registry.zookeeper.<n>.username` | string | `` | digest auth, applied via `AddAuth("digest", user:pass)`; set together with `password` | ⚠ one set, one empty → auth scheme error / ACL-denied writes |
| `spring.registry.zookeeper.<n>.password` | string | `` | digest password (see above) | as above |
| `spring.registry.service-name` | string | `` | logical name; becomes the znode directory and what discovery clients resolve. **Registration intent signal**: unset with blocks configured → a valid pure consumer | set with `addr` empty: Run returns `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` — after the app is otherwise up |
| `spring.registry.addr` | string | `` (required when registering) | advertised `host:port`; never guessed | empty: same Run error; malformed is NOT validated (no numeric-port check unlike consul) — stored verbatim, consumers fail to dial |
| `spring.registry.id` | string | `` | instance-id override; empty derives `<service-name>-<addr>` so restarts replace the same znode (`registrar.go`) | ⚠ duplicate ids across processes → one process's Register deletes and replaces the other's node |
| `spring.registry.weight` | int | `100` | advertised LB weight; `<=0` normalized to 1 at write time | 0 does **not** drain here (normalized); drain is `UpdateWeight(0)` only |
| `spring.registry.metadata.*` | map[string]string | empty | arbitrary attributes (zone, version, ...) stored in the znode payload and passed through to discovery Metadata | — |

Two blocks with the same `<name>` fail loudly in the container (duplicate bean name); block names
across backends never collide (the bean name carries the backend type, e.g. `zookeeper.main` vs
`nacos.main`).

---

## 4. Verification & fault drills

All zk-side checks work with `zkCli.sh` inside the container (see §1) or any zk client.

1. **Register → resolve**: boot the example — it self-verifies by listing `/services/orders` and
   printing `registered node=... value=...`; `check.sh` greps exactly that marker. In zkCli:
   `ls /services/orders` shows one child named `orders-127.0.0.1:8080`; `get` shows the JSON
   payload with `"weight":100` and an `ephemeralOwner != 0` (the ephemeral marker).
2. **Drain via UpdateWeight(0) + restore**: inject the `gs.Server` named `registryServer`, call
   `server.UpdateWeight(ctx, 0)`, then in zkCli `get /services/orders/orders-127.0.0.1:8080` —
   the payload now has **no `weight` field** (omitted at 0); the node still exists
   (ephemeralOwner unchanged — it was `Set`, not recreated). A consumer pool's next snapshot
   excludes it. `UpdateWeight(ctx, 100)` restores `"weight":100`. Calling it before Run
   registers returns `registry: instance not registered yet`.
3. **Graceful shutdown deregister**: the example SIGTERMs itself after verifying — PreStop
   deregisters before servers stop; `ls /services/orders` returns empty immediately after exit.
   `Stop` runs Deregister again: idempotent, `ErrNoNode` tolerated.
4. **Crash / session-expiry instance loss**: boot the example in manual mode
   (`go run . -manual` — server stays up), then `kill -9 <pid>` → no Deregister runs; the
   ephemeral node disappears when the session expires, ~one `session-timeout` (10s in the
   example) later — no critical-marking phase, no reaper config, unlike TTL-based registries.
   Watch it vanish: `zkCli.sh ls -w /services/orders` or poll `get` until `NoNode`. A consumer's
   `ChildrenW` fires on the deletion and its next snapshot drops the endpoint.
5. **Bad ensemble fail-fast**: set `servers=127.0.0.1:9999` on the block, boot → startup fails
   with `registry-zookeeper: startup probe failed` — by design, instead of surfacing on the
   first Register.
6. **Restart replacement**: kill -9, restart immediately (before the old session expires) —
   Register succeeds despite the lingering old ephemeral node (delete + recreate); zkCli shows
   exactly one child.
7. **Dual write (two blocks)**: add a second block
   (`spring.registry.zookeeper.dr.servers=...`) — the same instance appears under both
   ensembles (one publication fanned out), and the startup log says `in 2 registry center(s)`.

Runtime logs from this module carry the default app tag (`logger.appdef`) plus the core's
`_app_registry` tag for registration lifecycle lines. No metrics/traces/health indicator of its
own — a silently expired session logs **nothing**.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| starter inert, nothing registered | no `spring.registry.zookeeper.*` block anywhere | configure a named block — its `servers` is the activation switch |
| startup fails `startup probe failed` | ensemble unreachable / wrong servers | start ZooKeeper, fix the block's `servers`; the probe blocks up to `session-timeout` |
| startup error `service-name and addr are required` | either instance key unset while registering | set both — note this fires at Run, after other servers are already up |
| startup aborts "... but no registry center is configured" | `service-name` set yet no connection block anywhere | add at least one `spring.registry.<backend>.<name>` block |
| instance vanished while the app was running | session expired (long GC pause, network partition, session-timeout too low); zk removed the ephemeral node and the starter never re-registers | raise `session-timeout`; restart the process; watch for reconnect gaps in the zk client logs |
| node lingers long after `kill -9` | session not yet expired — removal takes up to one `session-timeout` | wait it out or lower `session-timeout`; do not add a reaper, the mechanism is ephemeral-by-design |
| consumer still sends traffic after `UpdateWeight(0)` | consumer snapshot stale (watch not yet fired) or backend ignores the omitted-weight field and defaults it to 1 | re-read the node; ensure the backend decodes absent `weight` as 0, not 1 |
| two processes, only one node | derived id collision (same name+addr) — later Register deletes and replaces the earlier node | set distinct `spring.registry.id` per instance |
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

1. ~~`spring.registry.*` instance keys bound in this one starter~~ RESOLVED 2026-09: the
   `${spring.registry}` identity block now lives in the shared starter-registry core
   (`RegistrationConfig`), so every backend starter binds it exactly once.
2. No re-registration after session loss: the zk library reconnects transparently, but once the
   session has expired the ephemeral node is gone and the running process never notices — the
   instance silently disappears from discovery until restart. A SessionW/State-driven
   re-register loop is the candidate fix.
3. ~~No shipped consumer side~~ RESOLVED: `discovery_zookeeper.go` ships the backend — since the
   2026-09 multi-registry named-block redesign it IS the block's bean (`zookeeper.<name>`),
   implementing both `discovery.Registrar` and `discovery.Discovery`; registration moved into
   the shared starter-registry core.
4. `addr` is not validated for `host:port` shape at write time (consul validates a numeric port);
   a malformed value is stored verbatim and only fails on the consumer dial.
5. Startup validation happens in Run, not bind time — an empty `service-name` fails only after
   the app is otherwise up (still a latency-of-failure smell).

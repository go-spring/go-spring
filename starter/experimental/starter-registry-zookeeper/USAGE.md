# starter-registry-zookeeper Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `registrar.go`, `config.go`, `registrar_test.go`), the
`cloud/discovery` seam (`cloud/discovery/discovery.go`, `cloud/discovery/loader.go`,
`cloud/loadbalance/pool.go`) and the runnable [example/](example/) (`example/check.sh` runs unit
tests + a docker-compose ZooKeeper end-to-end boot). **ZooKeeper's own semantics (sessions,
ephemeral znodes, watchers, digest auth) are
[ZooKeeper documentation](https://zookeeper.apache.org/doc/current/zookeeperProgrammers.html)** —
everything below is go-spring's increment.

**Activation**: the registrar server bean exists only when `spring.registry.zookeeper.servers` is
set — that key is the on/off switch (`starter.go:63-68`). This starter is **register-side only**: it
advertises this instance to ZooKeeper as an ephemeral znode; it ships no client-side discovery
backend (see §1 consumer note). It opens no port — it exports a `gs.Server` purely to plug
registration into the app lifecycle (`starter.go:84-93`).

---

## 1. Complete worked project

Two sides: a **provider** (this starter + a served endpoint) and a **consumer** (any
discovery-aware client starter resolving through a ZooKeeper-backed `discovery.Discovery`).
File tree:

```
demo/
├── go.mod
├── main.go
├── consumer_side/
│   └── discovery.go     # ZooKeeper-backed Discovery (see note below)
└── conf/
    └── app.properties
```

**Prerequisite** (single external dependency): a ZooKeeper node — verbatim from
`example/docker-compose.yml` (zookeeper:3.9 on 127.0.0.1:2181):

```bash
docker compose -f example/docker-compose.yml up -d
# readiness (four-letter word; what check.sh polls, example/check.sh:52-58):
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

# ZooKeeper ensemble to register into. Setting the servers activates the starter.
spring.registry.zookeeper.servers=127.0.0.1:2181
spring.registry.zookeeper.session-timeout=10s
spring.registry.zookeeper.base-path=/services

# The instance to advertise (backend-agnostic; switching registry backends is a
# blank-import swap, not a config change).
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1
```

**Consumer side.** This starter does not ship a ZooKeeper discovery backend. The intended seam is
`cloud/discovery`: register one ZooKeeper-backed `Discovery` under a name, then any client
starter's `discovery:` field resolves through it. The backend is small because the payload is
self-describing JSON (`registrar.go:44-50`): list the children of
`<base-path>/<service-name>` (each child name is the instance id), `Get` each znode's data, decode
`instanceValue`, map to `discovery.Endpoint{Addr, Weight, Metadata}`, and keep it fresh with
`ChildrenW`/`GetW` watchers (see §2.2 for the watch loop shape):

```go
// consumer_side/discovery.go — call once at startup.
func registerZkDiscovery(name, servers, basePath string) error {
    b, err := zkdisc.New(servers, basePath) // ChildrenW/GetW → snapshot → internal cache
    if err != nil { return err }
    discovery.RegisterDiscovery(name, b)    // cloud/discovery seam
    return nil
}
```

```properties
spring.redis.demo.service-name=orders
spring.redis.demo.discovery=zookeeper   # matches the name registered above
```

**Verify** (mirrors `example/example.go:87-105 verifyOnce` and `example/check.sh`):

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

### 2.1 Bean lifecycle timeline (all `starter.go` / `registrar.go`)

```
blank-import starter-registry-zookeeper
  └─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
         condition: gs.OnProperty("spring.registry.zookeeper.servers")   [starter.go:63-68]
        │
gs.Run()
  ├─ bind: ${spring.registry.zookeeper} → ZookeeperConfig (connection, config.go:23-42)
  ├─ bind: ${spring.registry} → Server.Config field (instance, config.go:48-68)
  ├─ construction: zk.Connect(servers, session-timeout) + digest AddAuth when set
  │    + fail-fast probe Exists("/") — blocks until the session connects, so an
  │      unreachable ensemble fails STARTUP, not the first Register
  │    (registrar.go:64-90; rationale comments registrar.go:61-63, 79-80)
  ├─ Run: validate service-name/addr BEFORE readiness                        [starter.go:98-101]
  ├─ wait <-sig.TriggerAndWait() — the ready-gate: registration only after
  │    every other server is up                                              [starter.go:110]
  ├─ Register: ensureParents (persistent dirs, on demand) + Create ephemeral
  │    znode at <base-path>/<service>/<id>                                   [registrar.go:109-147]
  │    log: registered %q at %s                                              [starter.go:117]
  ├─ Run blocks on <ctx.Done() — no heartbeat goroutine: liveness IS the session
  └─ on SIGTERM: PreStop → Deregister FIRST (before the pre-stop delay and any
       server stops) so discovery stops handing the instance out while in-flight
       requests drain; Stop/StopContext deregister again as idempotent
       fallbacks (ErrNoNode tolerated)                            [starter.go:126-142, registrar.go:181-187]
```

### 2.2 Session & ephemeral node mechanics — the crash-safety contract

- Registration is `Create(path, val, zk.FlagEphemeral, WorldACL(PermAll))` — one ephemeral znode
  per instance, owned by this client's session (`registrar.go:135`, `registrar.go:85-89`).
- An ephemeral node lives only as long as the session: if the process dies without deregistering,
  ZooKeeper removes the node once the session expires — self-healing with no reaper, no TTL
  heartbeat goroutine, nothing to configure beyond `session-timeout` (package doc
  `starter.go:27-30`; `config.go:29-32`). Correctness never depends on Deregister running
  (`registrar.go:31-34`).
- The zk library reconnects and re-establishes the session automatically on transient network
  loss; if it cannot within the session timeout the session expires and **the node silently
  disappears** — see the drill in §4.4 and the troubleshooting row "instance vanished while the
  app was running". The starter does not re-register after session loss (no reconnect callback is
  wired).
- Restart replacement: an ephemeral node from a previous session may linger briefly; Register
  tolerates `ErrNodeExists` by deleting and re-creating the node, so a restart refreshes the entry
  instead of failing (`registrar.go:133-145`).

### 2.3 WATCH path (consumer side)

This starter has no watch; consumers use the `cloud/discovery` seam. A ZooKeeper backend's loop
mirrors how the pool consumes snapshots:

1. `ChildrenW(basePath/service)` fires on any instance join/leave (including ephemeral deletion on
   session expiry — ZooKeeper's own watch event, not our code).
2. The backend lists children and `Get`s each one's data (an instance-id child per instance,
   `registrar.go:101-104`); `GetW` covers in-place data rewrites (weight changes).
3. Each event yields a fresh full `[]discovery.Endpoint` snapshot, stored into the backend's
   internal cache (`cloud/discovery/discovery.go`). The backend does not diff; every stored
   snapshot is authoritative.
4. A discovery `Loader` re-reads the backend snapshot on every call
   (`cloud/discovery/loader.go`); the loadbalance `Pool` (via `SourceFunc`) picks from that set.

### 2.4 DRAIN path — UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)` (`starter.go:149-154`) → the registrar verifies the node still exists
(error `update weight for unregistered instance` otherwise, `registrar.go:168-172`), maps negative
weights to 1 but passes 0 through (`registrar.go:155-157`), then rewrites the payload with
`conn.Set(path, val, -1)` — an **in-place data set, not delete+recreate** (`registrar.go:173`).
Design rationale (source comment, `registrar.go:149-153`): the ephemeral owner and the watchers are
undisturbed — the session keeps owning the node, and a `GetW` consumer simply observes the new
value. Weight 0 serializes as an *omitted* `weight` field (`json:"weight,omitempty"`,
`registrar.go:48`), which readers reconstruct as 0 (`registrar_test.go:41-51`). Consumers' next
snapshot carries `Endpoint.Weight == 0` → `excludeDrained` drops it from every load-balance
strategy, falling back to the full set only when every endpoint is drained
(`cloud/loadbalance/pool.go:93-100, 122-134`). Restore with `UpdateWeight(ctx, 100)`.

At initial Register the weight is normalized `<=0 → 1`, so "default" is never stored as 0 — 0 is
reserved for the runtime drain signal, only reachable through `UpdateWeight`
(`registrar.go:113-117`).

---

## 3. Per-key behavior reference

Verified against `grep -rhoE 'value:"[^"]+"' ... | sort -u` — 10 keys + the struct-level
`${spring.registry}` field bind (`starter.go:89`). Connection keys bind under
`${spring.registry.zookeeper}` (`config.go:23-42`); instance keys under `${spring.registry}`
(`config.go:48-68`).

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `spring.registry.zookeeper.servers` | []string | — (required) | ensemble members; **its presence activates the starter** (`starter.go:68`) | unset: starter inert, no error; wrong value: startup fails at the probe (`registry-zookeeper: startup probe failed`, `registrar.go:81-84`) |
| `spring.registry.zookeeper.session-timeout` | duration | `10s` | zk session timeout; bounds the startup probe AND how long a crashed process's ephemeral node lingers (`config.go:29-32`, `registrar.go:68,81`) | ⚠ too long delays crash-driven instance removal; too short risks session expiry on GC pauses / transient partitions → silent deregistration |
| `spring.registry.zookeeper.base-path` | string | `/services` | persistent parent znode; trailing `/` trimmed (`registrar.go:87`); service dirs created on demand | consumers must list the same path; a mismatch is invisible to the provider |
| `spring.registry.zookeeper.username` | string | `` | digest auth, applied via `AddAuth("digest", user:pass)`; set together with `password` (`registrar.go:73-78`) | ⚠ one set, one empty → auth scheme error / ACL-denied writes |
| `spring.registry.zookeeper.password` | string | `` | digest password (see above) | as above |
| `spring.registry.service-name` | string | `` (required) | logical name; becomes the znode directory and what discovery clients resolve | empty: Run returns `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` (`starter.go:99-101`) — after the app is otherwise up |
| `spring.registry.addr` | string | `` (required) | advertised `host:port`; never guessed | empty: same Run error; malformed is NOT validated (no numeric-port check unlike consul) — stored verbatim, consumers fail to dial |
| `spring.registry.id` | string | `` | instance-id override; empty derives `<service-name>-<addr>` so restarts replace the same znode (`registrar.go:94-99`) | ⚠ duplicate ids across processes → one process's Register deletes and replaces the other's node (`registrar.go:139-144`) |
| `spring.registry.weight` | int | `0` | advertised LB weight; `<=0` normalized to 1 at write time (`registrar.go:116-118`) | 0 does **not** drain here (normalized); drain is `UpdateWeight(0)` only |
| `spring.registry.metadata.*` | map[string]string | empty | arbitrary attributes (zone, version, ...) stored in the znode payload and passed through to discovery Metadata | — |

---

## 4. Verification & fault drills

All zk-side checks work with `zkCli.sh` inside the container (see §1) or any zk client.

1. **Register → resolve**: boot the example — it self-verifies by listing `/services/orders` and
   printing `registered node=... value=...` (`example/example.go:87-105`); `check.sh` greps exactly
   that marker (`example/check.sh:76-79`). In zkCli: `ls /services/orders` shows one child named
   `orders-127.0.0.1:8080`; `get` shows the JSON payload with `"weight":100` and an
   `ephemeralOwner != 0` (the ephemeral marker).
2. **Drain via UpdateWeight(0) + restore**: inject the `gs.Server` named `registryServer`, call
   `server.UpdateWeight(ctx, 0)`, then in zkCli `get /services/orders/orders-127.0.0.1:8080` —
   the payload now has **no `weight` field** (omitted at 0, `registrar_test.go:44-46`); the node
   still exists (ephemeralOwner unchanged — it was `Set`, not recreated). A consumer pool's next
   snapshot excludes it. `UpdateWeight(ctx, 100)` restores `"weight":100`. Calling it before Run
   registers returns `registry-zookeeper: instance not registered yet` (`starter.go:150-152`).
3. **Graceful shutdown deregister**: the example SIGTERMs itself after verifying
   (`example/example.go:52-53`) — PreStop deregisters before servers stop; `ls /services/orders`
   returns empty immediately after exit. `Stop` runs Deregister again: idempotent, `ErrNoNode`
   tolerated (`registrar.go:181-187`).
4. **Crash / session-expiry instance loss**: boot the example in manual mode
   (`go run . -manual` — server stays up, `example/example.go:29,55-59`), then
   `kill -9 <pid>` → no Deregister runs; the ephemeral node disappears when the
   session expires, ~one `session-timeout` (10s in the example) later — no critical-marking phase,
   no reaper config, unlike TTL-based registries. Watch it vanish:
   `zkCli.sh ls -w /services/orders` or poll `get` until `NoNode`. A consumer's `ChildrenW` fires
   on the deletion and its next snapshot drops the endpoint.
5. **Bad ensemble fail-fast**: set `servers=127.0.0.1:9999`, boot → startup fails with
   `registry-zookeeper: build registrar ... startup probe failed` (`registrar.go:79-84`,
   `starter.go:77-80`) — by design, instead of surfacing on the first Register.
6. **Restart replacement**: kill -9, restart immediately (before the old session expires) —
   Register succeeds despite the lingering old ephemeral node (delete + recreate,
   `registrar.go:133-145`); zkCli shows exactly one child.

Runtime logs all carry `log.TagAppDef` (`logger.appdef`): `creating zookeeper registrar servers=...`
(Debug), `registering service=...` (Debug), `registered %q at %s` (Info), `deregister %q` (Warn).
No metrics/traces/health indicator of its own — a silently expired session logs **nothing**.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| starter inert, nothing registered | `spring.registry.zookeeper.servers` unset | set it — that key is the activation switch |
| startup fails `startup probe failed` | ensemble unreachable / wrong servers | start ZooKeeper, fix `servers`; the probe blocks up to `session-timeout` |
| startup error `service-name and addr are required` | either instance key unset | set both — note this fires at Run, after other servers are already up (`starter.go:99-101`) |
| instance vanished while the app was running | session expired (long GC pause, network partition, session-timeout too low); zk removed the ephemeral node and the starter never re-registers | raise `session-timeout`; restart the process; watch for reconnect gaps in the zk client logs |
| node lingers long after `kill -9` | session not yet expired — removal takes up to one `session-timeout` | wait it out or lower `session-timeout`; do not add a reaper, the mechanism is ephemeral-by-design |
| consumer still sends traffic after `UpdateWeight(0)` | consumer snapshot stale (watch not yet fired) or backend ignores the omitted-weight field and defaults it to 1 | re-read the node; ensure the backend decodes absent `weight` as 0, not 1 (`registrar_test.go:48-50`) |
| two processes, only one node | derived id collision (same name+addr) — later Register deletes and replaces the earlier node (`registrar.go:139-144`) | set distinct `spring.registry.id` per instance |
| `UpdateWeight` errors `update weight for unregistered instance` | node gone (session expired) or called before Run | re-register by restarting, or call after readiness |
| ACL / auth errors on write | ensemble requires digest auth, `username`/`password` unset or half-set | set both together |
| restart failed `create ... ErrNodeExists`-style replace error | replace raced a concurrent registrant on the same path | distinct ids per instance fixes it |

---

## 6. Design health

| metric | value |
|--------|-------|
| config keys | 10 (5 connection + 5 instance) |
| required | 3 (`servers`, `service-name`, `addr`) |
| quickstart external deps | 1 (ZooKeeper, docker) |
| "notes/gotchas" | 4 (session-expiry silence; weight normalization; restart replacement; id collision) |

Suspect ledger (kept from the previous edition, plus new findings):

1. `spring.registry.*` instance keys are shared vocabulary across registry starters but bound in
   this one — a second registry starter re-implements `RegistrationConfig` (pre-existing entry).
2. Experimental: lives under `experimental/` (unreviewed marker, not a quality grade) (pre-existing).
3. No re-registration after session loss: the zk library reconnects transparently, but once the
   session has expired the ephemeral node is gone and the running process never notices — the
   instance silently disappears from discovery until restart. A SessionW/State-driven re-register
   loop is the candidate fix.
4. No shipped consumer side: every user hand-rolls a ZooKeeper `discovery.Discovery`
   (ChildrenW/GetW loop, §2.3) — same gap as consul, candidate for a discovery sibling.
5. `addr` is not validated for `host:port` shape at write time (consul validates a numeric port);
   a malformed value is stored verbatim and only fails on the consumer dial.
6. Startup validation happens in Run, not bind time — an empty `service-name` fails only after the
   app is otherwise up (parity with consul, still a latency-of-failure smell).

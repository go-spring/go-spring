# starter-registry-etcd Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `config.go`, `registrar.go`, `discovery.go`,
`registrar_weight_test.go`) and the runnable [example/](example/) (`example/check.sh` runs unit
tests plus a docker-compose etcd end-to-end boot). **etcd's own semantics (leases, watches, KV,
auth) are [etcd documentation](https://etcd.io/docs/)** — everything below is go-spring's increment.

**Model**: config is NAMED BLOCKS — each `spring.registry.etcd.<name>.*` block describes ONE etcd
cluster and becomes ONE backend bean named `etcd.<name>` (`center.go`). The bean implements BOTH
sides of the naming idiom: `discovery.Registrar` (write — collected by the `registryServer` from
the [starter-registry](../starter-registry) core, imported transitively, which registers into
EVERY configured center across backends) and `discovery.Discovery` (read — consumers cite the bean
name, e.g. `discovery=etcd.main`; the bean is lazy, so a pure provider never pays for the read
half). Read and write share the block's client and key prefix, so they can never diverge. There is
no default/unnamed block. Registration activates only when `spring.registry.service-name` is set —
a pure consumer app configures only connection blocks and registers nothing. Multi-center (dual
registration, cross-backend mixes) is just more blocks: configure two blocks + service-name and
every center gets the instance. It opens no port — the exported `gs.Server` (in the
starter-registry core) exists purely to plug registration into the app lifecycle.

---

## 1. Complete worked project

A **provider** (this starter + a served endpoint) and a **consumer** (a discovery-aware client
starter resolving through the etcd discovery backend this same starter registers). File tree:

```
demo/
├── go.mod
├── main.go
├── provider.go
└── conf/
    └── app.properties
```

**Prerequisite** (single external dependency): one etcd node —

```bash
docker run -d --name etcd -p 127.0.0.1:2379:2379 \
  gcr.io/etcd-development/etcd:v3.5.15 etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://0.0.0.0:2379
curl -fsS http://127.0.0.1:2379/health        # {"health":"true",...}
```

**go.mod**:

```
require (
    go-spring.org/spring                   v1.3.x
    go-spring.org/starter-registry-etcd    latest
    go-spring.org/starter-redigo           latest   // any discovery-aware client starter
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-etcd"
)

func main() { gs.Run() }
```

**provider.go** — hold the registry server to drive runtime weight changes (optional):

```go
package main

import (
    "context"

    "go-spring.org/spring/gs"
    StarterRegistry "go-spring.org/starter-registry"
)

func init() {
    // The registrar bean is named "registryServer" and exported as gs.Server.
    // Inject it if you need runtime drain/restore; otherwise omit this file.
    gs.Provide(func(s *StarterRegistry.Server) *Drainer {
        return &Drainer{srv: s}
    })
}

type Drainer struct{ srv *StarterRegistry.Server }

// Drain takes the instance out of load-balancing without stopping it:
// rewrites the stored weight on the SAME lease in every center (no
// re-register, no key churn).
func (d *Drainer) Drain(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 0) }
func (d *Drainer) Restore(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 100) }
```

**conf/app.properties** (verbatim from `example/conf/app.properties`):

```properties
spring.app.name=registry-etcd-example

# One named block per etcd cluster; each block becomes the backend bean
# "etcd.<name>" serving both registration (via the starter-registry core)
# and discovery.
spring.registry.etcd.main.endpoints=127.0.0.1:2379
spring.registry.etcd.main.ttl=10s
spring.registry.etcd.main.key-prefix=/services/

# The instance to advertise (backend-agnostic, shared by every center;
# switching registry backends is a blank-import swap, not a config change).
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1

# Consumer side: no extra config. Client starters cite the block's bean name:
# spring.redis.demo.service-name=orders
# spring.redis.demo.discovery=etcd.main
```

**Verify** (mirrors `example/check.sh`):

```bash
go run ./example          # logs: registered "orders" at 127.0.0.1:8080
                          #       discovered endpoint=127.0.0.1:8080 weight=100 ...
etcdctl get /services/orders/ --prefix
# /services/orders/orders-127.0.0.1:8080
# {"service_name":"orders","addr":"127.0.0.1:8080","weight":100,"metadata":{...}}
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-registry-etcd (transitively imports starter-registry)
  ├─ per ${spring.registry.etcd.<name>} block: gs.Provide(newEtcdBackend).Name("etcd.<name>")
  │      Module Condition: OnProperty("spring.registry.etcd")       [center.go]
  │      Export(As[discovery.Discovery], As[discovery.Registrar]) + Destroy(Close)
  │
  ├─ starter-registry core: gs.Provide(NewServer).Name("registryServer")
  │      Condition: OnProperty("spring.registry.service-name")      [starter-registry/starter.go]
  │      Registrars []discovery.Registrar — the container collects EVERY backend
  │      bean's registrar (etcd, zookeeper, ... mixed) by slice injection
  │
gs.Run()
  ├─ bind: ${spring.registry.etcd} → one EtcdConfig per block (BindEach);
  │        ${spring.registry} → Server.Config field
  ├─ newEtcdBackend (per block): clientv3.New + Status probe on endpoints[0] —
  │      unreachable/misauthed cluster FAILS STARTUP here, not on first
  │      Register; once per block                                   [center.go]
  ├─ discovery half of each backend (lazy) resolves on the block's client at first use
  ├─ Runners start → readiness signal fires
  ├─ registryServer.Run: validate service-name/addr + ≥1 registrar
  │      wait <-sig.TriggerAndWait()  (ready-gate: register only after ALL servers up)
  │      Register into EVERY center → log `registered "orders" at ... in N registry center(s)`
  ├─ steady state: per-center keep-alive goroutine renews the lease (clientv3 ≈ TTL/3)
  └─ SIGTERM: PreStop deregisters FIRST (before pre-stop delay, before servers stop)
         → discovery stops handing the instance out while in-flight drains
         Stop is an idempotent fallback                  [starter-registry/starter.go]
```

Design rationale (source comments): registration is keyed to app readiness because "the instance
is published once the application is ready ... That ordering is what makes a rolling restart
lossless" (`starter-registry/starter.go`); crash safety never depends on Deregister — the lease
TTL removes dead keys "self-healing without a reaper" (`starter.go`).

### 2.2 Registration write semantics

- Key = `<key-prefix><service-name>/<instance-id>`, id = configured `id` else
  `<service-name>-<addr>` — restarts replace the same key (`registrar.go`).
- Value = JSON `instanceValue{service_name, addr, weight, metadata}` (`registrar.go`).
- A negative weight at Register is normalized to 1; **0 passes through as the drain signal** on
  both write paths, so `weight=0` in config registers a drained instance (`registrar.go`).
- `Register` = Grant(TTL seconds) → `Put(key, val, WithLease)` → `KeepAlive` goroutine that must
  drain the renewal channel or the lease dies (`registrar.go`). Re-registering the same
  instance retires the old lease first (`registrar.go`).
- `ttlSeconds()` rounds sub-second values up and clamps to ≥1s; `TTL<=0` silently becomes 15s
  (`config.go`).

### 2.3 DISCOVERY path (consumer side)

`etcdDiscovery.Resolve` (`discovery.go`):

1. The first Resolve of a service takes a **full snapshot** (`Get` + `WithPrefix`, bounded by the
   caller's ctx) and caches it, then starts a background watcher
   (`clientv3.Watch(bgCtx, prefix, WithPrefix())`).
2. Every etcd event on the watch channel triggers a **fresh full snapshot** (not a delta) stored
   into the cache; a failed refresh keeps the stale one — stale addresses are safer than none.
3. Later Resolve calls are in-memory reads over the cached (unfiltered) set, narrowed by scheme
   via `FilterByScheme` per call.
4. Each KV is decoded from the registrar's JSON; `Healthy` is always true — **key existence IS the
   health signal** (lease expiry deletes the key) (header comment lines 29-32). A malformed
   payload is skipped with a Warn, not a broken snapshot.
5. `Services` enumerates service names from the key layout.

Downstream, a loadbalance `Pool` consumes these snapshots and applies **weight-0 filtering** on
every Pick: `excludeDrained` drops `Weight == 0` endpoints, falling back to the full set when
every endpoint is drained so an unnormalized snapshot cannot blackhole the pool
(`cloud/loadbalance/pool.go:97-99,122-135`).

### 2.4 DRAIN path — UpdateWeight(0)

`Server.UpdateWeight(ctx, 0)` (starter-registry `starter.go`) → guard: never-registered →
error `registry: instance not registered yet` → `etcdRegistrar.UpdateWeight` per center
(`registrar.go`):

- Looks up the `hold` (leaseID + last payload) for the key; no hold → explanatory error
  (`registrar.go`; unit-tested in `registrar_weight_test.go`).
- Marshals the **full instanceValue** with the new weight and `Put`s it **on the existing lease**
  (`clientv3.WithLease(h.leaseID)`) — "no lease is granted, revoked or re-kept-alive, so the
  instance never disappears from discovery mid-update and watchers simply see a new value for the
  same key" (`registrar.go`).
- Unlike Register, `UpdateWeight` passes **0 through untouched** (only Register clamps) — 0 reaches
  the stored JSON and thus the consumer pool's `excludeDrained`.

Consumers observe it purely through WATCH: the etcd event → snapshot → `endpointsKey` changes
(weight is in the key) → push → pool filters the 0-weight endpoint on every strategy. Restore with
`UpdateWeight(ctx, 100)`. The same-lease hot-reload is proven end-to-end against a live etcd in
`registrar_weight_test.go` `TestUpdateWeightHotReloadLive` (watch distribution flips 9:1 → 1:9
with no Register call).

### 2.5 TTL / keep-alive mechanics — and the known fragility

- One lease per instance per center, TTL in whole seconds (default 15s; example 10s). The
  keep-alive goroutine exists solely to drain clientv3's renewal channel — "the returned channel
  must be drained or the lease will not be renewed" (`registrar.go`). Renewal cadence is
  clientv3's (≈ TTL/3), not configurable here.
- **Keep-alive loss self-heals**: if the lease dies server-side (etcd restart/compaction, lease
  revoked out-of-band), the keep-alive channel closes and the watcher goroutine re-runs the
  publish step (re-grant lease + re-put the last payload) with exponential backoff (base 1s
  doubling, capped at 1min) until the instance is registered again — the entry always comes back
  without operator action (`watchKeepAlive`, `registrar.go`).
- Deregister = cancel keep-alive + revoke lease (deletes the key immediately, no TTL wait)
  (`registrar.go`). Failed deregister on shutdown logs a Warn and leaves the key to TTL expiry
  (starter-registry `starter.go`).

---

## 3. Per-key behavior reference

### 3.1 `${spring.registry.etcd.<name>.*}` — per-cluster block (`config.go`)

One block per etcd cluster; the block name is yours to choose and becomes the backend bean
`etcd.<name>`. Any block present activates the module (`OnProperty("spring.registry.etcd")` is a
prefix check); every block is bound via `BindEach` and probed at boot.

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoints` | []string | — | Required per block. Probed at boot: `Status` on `endpoints[0]`. | unset: block fails bind (`endpoints is required`); unreachable: startup fails `registry-etcd: startup probe failed` (`center.go`) |
| `username` / `password` | string | "" | etcd auth credentials. | auth-enabled cluster without them → probe fails at startup |
| `dial-timeout` | duration | 5s | Bounds client dial AND the startup probe timeout. | too low → flaky startup failures on slow networks |
| `ttl` | duration | 15s | Lease TTL; rounded up to whole seconds, min 1s. ⚠ `<=0` silently becomes 15s, not a bind error. | too long delays crash-eviction to ~TTL; 0 does not disable anything |
| `key-prefix` | string | `/services/` | Prepended to every key (read AND write share it). ⚠ must equal the block the consumer cites — the coupling is documented but never validated. | mismatch → provider registers, consumers resolve nothing, both "succeed" |
| `tls.*` | tlsconf | off | Shared `cloud/tlsconf` block: `enabled`, `cert-file`, `key-file`, `ca-file`, `server-name`, `insecure-skip-verify`. | wrong CA → startup probe failure |

Two blocks = two centers = dual registration (the registryServer registers into both). Mixing
backends (an etcd block + a zookeeper block in one app) works identically — the registrar
collection is backend-agnostic.

### 3.2 `${spring.registry.*}` — the advertised instance (starter-registry `config.go`)

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `service-name` | string | "" | Logical name; becomes the key segment and the name consumers resolve/watch. **Its presence is the registration intent signal** — set registers into every center, unset = pure consumer. | empty → pure consumer; set without any block → Run error `registry: ${spring.registry.service-name} is set but no registry center is configured` |
| `addr` | string | "" | Advertised `host:port`. Never guessed. | empty with service-name set → same Run error; Register also `RequireField`s it (`registrar.go`) |
| `id` | string | "" | Instance id override; empty derives `<service-name>-<addr>` so restarts replace the same key. ⚠ duplicate ids across processes overwrite each other's lease holds. | same name+addr in two processes → one entry, lease ping-pong |
| `weight` | int | 100 | Stored weight; negative normalized to 1 at write time (`registrar.go`). 0 = drained, honored from startup. | negative weight silently becomes 1 rather than erroring |
| `metadata` | map[string]string | empty | Arbitrary attributes (zone, version); `scheme` is the reserved key driving consumer-side scheme filtering (`discovery.go`). | wrong `scheme` value → instance filtered out of scheme-scoped queries |

### 3.3 Discovery — cite the block's bean name (no config)

There is no discovery configuration path. Every block's backend bean IS a
`cloud/discovery.Discovery` named `etcd.<name>` on the block's shared client (`center.go`).
Client starters cite that bean name (`spring.http-client.backends.<n>.discovery=etcd.main`).
The bean is lazy — a pure provider never resolves, never pays for the read half. A pure consumer
configures only connection blocks (no `service-name`/`addr`) and registers nothing.
Multi-cluster discovery is just multiple blocks: cite whichever block's cluster you want to read
from — it need not be one you register into.

---

## 4. Verification & fault drills

All drills use `etcdctl` (or `curl` v3 API) against the §1 stack. Keys:
`/services/orders/orders-127.0.0.1:8080`.

1. **Register → resolve**: `go run .` (example mode) logs `registered "orders" at 127.0.0.1:8080`
   then `discovered endpoint=127.0.0.1:8080 weight=100 metadata=map[...]` — the full
   register→discover loop in one process (`example/example.go:86-111`). Raw view:

   ```bash
   etcdctl get /services/orders/ --prefix
   # value JSON: {"service_name":"orders","addr":"...","weight":100,...}
   ```

2. **Weight-0 drain + restore**: run the example with `-manual`, then call `UpdateWeight(ctx, 0)`
   (via the §1 Drainer). Same key, new JSON, same lease:

   ```bash
   etcdctl get /services/orders/orders-127.0.0.1:8080    # "weight":0
   # a consumer pool's next watch snapshot drops the endpoint (excludeDrained)
   # restore: UpdateWeight(ctx, 100) → weight back, endpoint re-enters rotation
   ```

   Live-asserted in `TestUpdateWeightHotReloadLive` (`registrar_weight_test.go:53-133`).

3. **Manual weight edit vs in-process state**: `etcdctl put /services/orders/orders-127.0.0.1:8080
   '{"service_name":"orders","addr":"127.0.0.1:8080","weight":7}'` (without a lease — key becomes
   permanent!). Watchers see weight 7, but a later in-process `UpdateWeight` overwrites it (suspect
   #6), and the manual key never expires on crash. Drill to understand, not to operate.

4. **Graceful shutdown deregister**: `kill <pid>` → PreStop revokes the lease → key gone instantly:

   ```bash
   etcdctl get /services/orders/ --prefix     # empty
   ```

5. **Crash / TTL expiry (instance loss)**: `kill -9 <pid>` → no revoke; key vanishes ~TTL after
   the last keep-alive renewal (default 15s, example 10s). Watch it happen:

   ```bash
   etcdctl watch /services/orders/ --prefix   # a DELETE event for the key fires at expiry
   ```

6. **Lease death self-healing**: restart the etcd container (or
   `etcdctl lease revoke <id>` found via `etcdctl get <key> -w json | jq .kvs[0].lease`) while the
   app runs: the key disappears after TTL, then the registrar's keep-alive watcher re-runs the
   publish step with backoff and the key comes back (`watchKeepAlive`, `registrar.go`) — no
   operator action, no process restart. Consumers see a brief eviction via watch, then the
   endpoint returns.

7. **Refresh degradation**: block etcd briefly (e.g. iptables drop 2379) after a service is cached:
   the refresh Get fails → Warn `registry-etcd: refresh %q failed (keeping stale snapshot)` and the
   stale snapshot keeps serving — "stale addresses are safer than none"; the cache refreshes when
   events resume.

8. **Bad cluster fail-fast**: set a block's `endpoints=127.0.0.1:9999`, boot → startup fails with
   `registry-etcd: startup probe failed for 127.0.0.1:9999` (`center.go`) — not a
   silent runtime gap.

Runtime logs carry tag `_app_registry_etcd`: `creating etcd registrar`, `registering service=...`,
`registered %q at %s`, `deregister %q` (Warn on failure), `registered etcd discovery backend
name=...`. Tune via `logger.<name>.tag=_app_registry_etcd`.

Observability: registration and discovery emit OTel metrics through the global providers — a no-op unless `starter-otel` is imported. `register`, `deregister` and `update_weight` each produce a client span plus a `registry.operation.duration` record labelled `system`/`operation`/`service`/`status`; `registry.registration.attempts_total` counts attempts by `reason` and `status`; the `registry.instance.registered` gauge reads 1 while this instance is published and 0 while it is not, so a failed self-heal lands there instead of only in a log line. The discovery half reports every background cache sync to `discovery.sync_total` and keeps `discovery.cache.age_seconds` (seconds since the snapshot was last confirmed fresh), so a dead watch shows a climbing age rather than a silently stale address list. `reason` is `initial` for the first publish and `self_heal` for the background re-registration.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| startup error `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` | `service-name` set but `addr` unset | set both (starter-registry `starter.go`) |
| startup error `... is set but no registry center is configured` | `service-name` set but no `spring.registry.etcd.<name>` block | add at least one block with `endpoints` |
| startup error `startup probe failed` | etcd unreachable / wrong TLS / missing auth | fix the block's `endpoints`/`tls.*`/credentials; the probe proves them at boot |
| provider runs fine, consumers resolve nothing | `key-prefix` differs between the block the provider registers into and the block the consumer cites | align both (default `/services/`); the mismatch is never validated |
| consumer cites `etcd.main` and gets no such bean | block named differently, or block missing entirely | the bean name is `etcd.<block-name>`; check the exact block key |
| instance briefly vanishes then returns | lease died (etcd restart, out-of-band revoke) and the self-healing loop re-registered | nothing to do — self-heals (drill 6); check etcd health if it recurs often |
| consumer keeps routing to a drained instance | pool saw only weight-0 endpoints and fell back to full set (`pool.go:97-99`), or snapshot stale | check whether ALL endpoints are drained; verify the watch delivered the 0-weight snapshot |
| restart leaves stale duplicate key | prior manual `etcdctl put` created a lease-less key that never expires | delete manual keys; never hand-write instance keys |
| `registry: instance not registered yet` on UpdateWeight | called before Run registered (or Run failed) | call after the `registered` log line |
| two processes, one etcd key | same name+addr → same derived id | set distinct `spring.registry.id` |
| `skip malformed instance` Warn in consumer logs | non-JSON value under the service prefix | remove the offending key (manual write, wrong-tool write) |

---

## 6. Design health

| metric | value |
|--------|-------|
| config keys | 7 per block (connection, incl. shared `tls.*`) + 5 instance (`spring.registry.*`) |
| required | 1 at bind per block (`endpoints`) + 2 at Run (`service-name`, `addr`) — registration only |
| quickstart external deps | 1 (etcd; docker-gated in example) |
| "watch out" entries | 6 |

Suspect ledger (updated for the named-block model):

1. `key-prefix` coupling between the block the provider registers into and the block the consumer
   cites is documented but never validated (within one block the two halves share one prefix; a
   cross-block mismatch remains possible).
2. ~~Lease keep-alive death = silent disappearance~~ RESOLVED: the keep-alive watcher re-registers
   with exponential backoff (`watchKeepAlive`, `registrar.go`).
3. Background refresh errors are logged but a permanently dead etcd keeps serving the stale snapshot.
4. `ttl<=0` silently becomes 15s instead of a bind-time error.
5. README weight row implies config `weight:=0` drains — it never stores 0 (clamped to 1); drain
   is `UpdateWeight`-only.
6. Manual weight edits at the registry are visible to watchers but a later in-process
   `UpdateWeight` overwrites them (no reconciliation).

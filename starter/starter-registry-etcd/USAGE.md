# starter-registry-etcd Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against
the starter source (`starter.go`, `config.go`, `registrar.go`, `discovery_etcd.go`,
`registrar_weight_test.go`) and the runnable [example/](example/) (`example/check.sh` runs unit
tests plus a docker-compose etcd end-to-end boot). **etcd's own semantics (leases, watches, KV,
auth) are [etcd documentation](https://etcd.io/docs/)** — everything below is go-spring's increment.

**Activation**: the registrar server bean exists only when `spring.registry.etcd.endpoints` is set
— that key is the on/off switch. The same block also builds the ONE shared etcd client
(`center.go`) and — unless `discovery-name` is empty — derives a discovery backend bean labeled
`etcd` (default) for the same cluster, so a dual-role app configures the cluster once. Standalone
`${spring.discovery.etcd.<name>}` blocks cover other clusters / pure consumers; a block with empty
`endpoints` inherits the center connection (`discovery_etcd.go`). Unlike starter-registry-consul,
this starter ships **both sides**: registration and discovery over the same key layout. It opens
no port — it exports a `gs.Server` purely to plug registration into the app lifecycle.

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
    StarterRegistryEtcd "go-spring.org/starter-registry-etcd"
)

func init() {
    // The registrar bean is named "registryServer" and exported as gs.Server.
    // Inject it if you need runtime drain/restore; otherwise omit this file.
    gs.Provide(func(s *StarterRegistryEtcd.Server) *Drainer {
        return &Drainer{srv: s}
    })
}

type Drainer struct{ srv *StarterRegistryEtcd.Server }

// Drain takes the instance out of load-balancing without stopping it:
// rewrites the stored weight on the SAME lease (no re-register, no key churn).
func (d *Drainer) Drain(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 0) }
func (d *Drainer) Restore(ctx context.Context) error { return d.srv.UpdateWeight(ctx, 100) }
```

**conf/app.properties** (verbatim from `example/conf/app.properties`):

```properties
spring.app.name=registry-etcd-example

# etcd cluster to register into. Setting the endpoints activates the starter.
spring.registry.etcd.endpoints=127.0.0.1:2379
spring.registry.etcd.ttl=10s
spring.registry.etcd.key-prefix=/services/

# The instance to advertise (backend-agnostic; switching registry backends is a
# blank-import swap, not a config change).
spring.registry.service-name=orders
spring.registry.addr=127.0.0.1:8080
spring.registry.weight=100
spring.registry.metadata.zone=cn-north
spring.registry.metadata.version=v1

# Consumer side: an etcd discovery backend named "local" resolving the same
# key prefix. Any client starter's `discovery:` field cites the name.
spring.discovery.etcd.local.endpoints=127.0.0.1:2379
spring.discovery.etcd.local.key-prefix=/services/

# Example consumer wiring (any discovery-aware client starter):
# spring.redis.demo.service-name=orders
# spring.redis.demo.discovery=local
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
import starter-registry-etcd
  ├─ gs.Provide(NewServer).Name("registryServer").Export(gs.As[gs.Server]())
  │      Condition: OnProperty("spring.registry.etcd.endpoints")   [starter.go:66-71]
  ├─ gs.Module(OnProperty("spring.discovery.etcd"))                 [discovery_etcd.go:82]
  │      conf.BindEach → one backend per <name> → a NAMED bean per <name> (r.Provide().Name(name))
  │
gs.Run()
  ├─ bind: ${spring.registry.etcd} → EtcdConfig; ${spring.registry} → Server.Config field
  ├─ NewServer: clientv3.New + Status probe on endpoints[0] — unreachable/misauthed
  │      cluster FAILS STARTUP here, not on first Register           [registrar.go:95-100]
  ├─ discovery backends probe identically at bind time               [discovery_etcd.go:116-121]
  ├─ Runners start → readiness signal fires
  ├─ Server.Run: validate service-name/addr                          [starter.go:103-105]
  │      wait <-sig.TriggerAndWait()  (ready-gate: register only after ALL servers up)
  │      Register → log `registered "orders" at ...`                 [starter.go:114-121]
  ├─ steady state: keep-alive goroutine renews the lease (clientv3 default ≈ TTL/3)
  └─ SIGTERM: PreStop deregisters FIRST (before pre-stop delay, before servers stop)
         → discovery stops handing the instance out while in-flight drains
         Stop is an idempotent fallback                    [starter.go:130-146]
```

Design rationale (source comments): registration is keyed to app readiness because "the instance
is published once the application is ready ... That ordering is what makes a rolling restart
lossless" (`starter.go:30-36`); crash safety never depends on Deregister — the lease TTL removes
dead keys "self-healing without a reaper" (`starter.go:27-29`).

### 2.2 Registration write semantics

- Key = `<key-prefix><service-name>/<instance-id>`, id = configured `id` else
  `<service-name>-<addr>` — restarts replace the same key (`registrar.go:111-122`).
- Value = JSON `instanceValue{service_name, addr, weight, metadata}` (`registrar.go:43-48,137-142`).
- Weight ≤ 0 at Register is normalized to 1: "default" is never stored as 0 — **0 is reserved for
  the runtime drain signal**, reachable only via `UpdateWeight` (`registrar.go:131-136`).
- `Register` = Grant(TTL seconds) → `Put(key, val, WithLease)` → `KeepAlive` goroutine that must
  drain the renewal channel or the lease dies (`registrar.go:147-170`). Re-registering the same
  instance retires the old lease first (`registrar.go:172-179`).
- `ttlSeconds()` rounds sub-second values up and clamps to ≥1s; `TTL<=0` silently becomes 15s
  (`config.go:85-98`).

### 2.3 DISCOVERY path (consumer side)

`etcdDiscovery.Resolve` (`discovery_etcd.go`):

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

`Server.UpdateWeight(ctx, 0)` (`starter.go:152-157`) → guard: registrar nil or never-registered →
error `registry-etcd: instance not registered yet` → `etcdRegistrar.UpdateWeight`
(`registrar.go:187-213`):

- Looks up the `hold` (leaseID + last payload) for the key; no hold → explanatory error
  (`registrar.go:189-194`; unit-tested `registrar_weight_test.go:34-39`).
- Marshals the **full instanceValue** with the new weight and `Put`s it **on the existing lease**
  (`clientv3.WithLease(h.leaseID)`) — "no lease is granted, revoked or re-kept-alive, so the
  instance never disappears from discovery mid-update and watchers simply see a new value for the
  same key" (`registrar.go:183-186,206`).
- Unlike Register, `UpdateWeight` passes **0 through untouched** (only Register clamps) — 0 reaches
  the stored JSON and thus the consumer pool's `excludeDrained`.

Consumers observe it purely through WATCH: the etcd event → snapshot → `endpointsKey` changes
(weight is in the key) → push → pool filters the 0-weight endpoint on every strategy. Restore with
`UpdateWeight(ctx, 100)`. The same-lease hot-reload is proven end-to-end against a live etcd in
`registrar_weight_test.go` `TestUpdateWeightHotReloadLive` (watch distribution flips 9:1 → 1:9
with no Register call).

### 2.5 TTL / keep-alive mechanics — and the known fragility

- One lease per instance, TTL in whole seconds (default 15s; example 10s). The keep-alive
  goroutine (`registrar.go:166-170`) exists solely to drain clientv3's renewal channel — "the
  returned channel must be drained or the lease will not be renewed" (`registrar.go:157-158`).
  Renewal cadence is clientv3's (≈ TTL/3), not configurable here.
- **Known behavior (suspect #2 below)**: if the lease dies server-side (etcd restart/compaction,
  lease revoked out-of-band), the key disappears after TTL and the keep-alive channel simply ends —
  **no re-registration, no log; the instance silently vanishes from discovery while the process
  keeps running**. Recovery is process restart. Monitor instance count, or `etcdctl watch` the key.
- Deregister = cancel keep-alive + revoke lease (deletes the key immediately, no TTL wait)
  (`registrar.go:217-233`). Failed deregister on shutdown logs a Warn and leaves the key to TTL
  expiry (`starter.go:159-166`).

---

## 3. Per-key behavior reference

### 3.1 `${spring.registry.etcd.*}` — cluster connection (`config.go:26-55`)

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoints` | []string | — | **Activation key** (OnProperty). Also probed at boot: `Status` on `endpoints[0]`. | unset: starter inert; unreachable: startup fails `registry-etcd: startup probe failed` (`registrar.go:95-100`) |
| `username` / `password` | string | "" | etcd auth credentials. | auth-enabled cluster without them → probe fails at startup |
| `dial-timeout` | duration | 5s | Bounds client dial AND the startup probe timeout. | too low → flaky startup failures on slow networks |
| `ttl` | duration | 15s | Lease TTL; rounded up to whole seconds, min 1s. ⚠ `<=0` silently becomes 15s, not a bind error. | too long delays crash-eviction to ~TTL; 0 does not disable anything |
| `key-prefix` | string | `/services/` | Prepended to every key. ⚠ must equal every consumer block's `key-prefix` — the coupling is documented but never validated. | mismatch → provider registers, consumers resolve nothing, both "succeed" |
| `tls.*` | tlsconf | off | Shared `cloud/tlsconf` block: `enabled`, `cert-file`, `key-file`, `ca-file`, `server-name`, `insecure-skip-verify`. | wrong CA → startup probe failure |

### 3.2 `${spring.registry.*}` — the advertised instance (`config.go:57-81`)

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `service-name` | string | "" | Logical name; becomes the key segment and the name consumers resolve/watch. Required. | empty → Run returns `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` (`starter.go:103-105`) |
| `addr` | string | "" | Advertised `host:port`. Never guessed. | empty → same Run error; Register also `RequireField`s it (`registrar.go:128-130`) |
| `id` | string | "" | Instance id override; empty derives `<service-name>-<addr>` so restarts replace the same key. ⚠ duplicate ids across processes overwrite each other's lease holds. | same name+addr in two processes → one entry, lease ping-pong |
| `weight` | int | 0 | Stored weight; `<=0` normalized to 1 **at Register** (`registrar.go:134-136`). ⚠ config 0 does NOT drain — drain is `UpdateWeight(0)` only. | expecting config-0 to drain → instance stays at weight 1 |
| `metadata` | map[string]string | empty | Arbitrary attributes (zone, version); `scheme` is the reserved key driving consumer-side scheme filtering (`discovery_etcd.go:248`). | wrong `scheme` value → instance filtered out of scheme-scoped queries |

### 3.3 `${spring.discovery.etcd.<name>.*}` — consumer backends (`discovery_etcd.go:58-79`)

| key | type | default | behavior / interactions | misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `endpoints` | []string | — | Required with bind-time `expr:"len($) > 0"`; each block registers one named backend into `cloud/discovery`. | duplicate `<name>` → startup error `discovery backend %q already registered` |
| `username` / `password` | string | "" | Same as registrar side. | auth failures at probe |
| `dial-timeout` | duration | 5s | Bounds dial and probe. | as above |
| `key-prefix` | string | `/services/` | Must match the registrar's prefix (§3.1 ⚠). | resolves nothing, no error |
| `tls.*` | tlsconf | off | Same shared block. | probe failure |

No TTL here: the consumer never writes (`discovery_etcd.go:56-57`).

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

6. **Lease-death silent disappearance (known issue)**: restart the etcd container (or
   `etcdctl lease revoke <id>` found via `etcdctl get <key> -w json | jq .kvs[0].lease`) while the
   app runs: the key disappears after TTL, the process keeps running, **no log is emitted, no
   re-registration happens** (`registrar.go:166-170` — the drain loop just ends). Consumers evict
   the endpoint via watch; the provider is unaware it is unregistered. Recovery: restart the
   process. This is suspect #2 in §6.

7. **Refresh degradation**: block etcd briefly (e.g. iptables drop 2379) after a service is cached:
   the refresh Get fails → Warn `registry-etcd: refresh %q failed (keeping stale snapshot)` and the
   stale snapshot keeps serving — "stale addresses are safer than none"; the cache refreshes when
   events resume.

8. **Bad cluster fail-fast**: set `endpoints=127.0.0.1:9999`, boot → startup fails with
   `registry-etcd: startup probe failed for 127.0.0.1:9999` (`registrar.go:95-100`) — not a
   silent runtime gap.

Runtime logs carry tag `_app_registry_etcd`: `creating etcd registrar`, `registering service=...`,
`registered %q at %s`, `deregister %q` (Warn on failure), `registered etcd discovery backend
name=...`. Tune via `logger.<name>.tag=_app_registry_etcd`. No metrics/traces are emitted.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| startup error `registry: ${spring.registry.service-name} and ${spring.registry.addr} are required` | either key unset | set both (`starter.go:103-105`) |
| startup error `startup probe failed` | etcd unreachable / wrong TLS / missing auth | fix `endpoints`/`tls.*`/credentials; the probe proves them at boot |
| provider runs fine, consumers resolve nothing | `key-prefix` mismatch between registry and discovery blocks | align both (default `/services/`); the mismatch is never validated |
| instance vanishes from discovery while process is healthy | lease died (etcd restart, out-of-band revoke) — silent, no re-registration | restart the process; monitor instance counts (drill 6) |
| `discovery backend "x" already registered` | two `${spring.discovery.etcd.x.*}` blocks | rename one |
| consumer keeps routing to a drained instance | pool saw only weight-0 endpoints and fell back to full set (`pool.go:97-99`), or snapshot stale | check whether ALL endpoints are drained; verify the watch delivered the 0-weight snapshot |
| restart leaves stale duplicate key | prior manual `etcdctl put` created a lease-less key that never expires | delete manual keys; never hand-write instance keys |
| `registry-etcd: instance not registered yet` on UpdateWeight | called before Run registered (or Run failed) | call after the `registered` log line |
| two processes, one etcd key | same name+addr → same derived id | set distinct `spring.registry.id` |
| `skip malformed instance` Warn in consumer logs | non-JSON value under the service prefix | remove the offending key (manual write, wrong-tool write) |

---

## 6. Design health

| metric | value |
|--------|-------|
| config keys | 18 (7 connection + 5 instance + 6 discovery) + 6 shared `tls.*` per tls block |
| required | 2 at bind (`endpoints` x2 sides) + 2 at Run (`service-name`, `addr`) |
| quickstart external deps | 1 (etcd; docker-gated in example) |
| "watch out" entries | 6 |

Suspect ledger (kept from previous audit; still open):

1. `key-prefix` coupling between registrar and discovery blocks is documented but never validated,
   even when both live in the same app.properties.
2. Lease keep-alive death = silent disappearance after TTL (no re-registration, no log) — the
   biggest operational hole.
3. Background refresh errors are logged but a permanently dead etcd keeps serving the stale snapshot.
4. `ttl<=0` silently becomes 15s instead of a bind-time error.
5. README weight row implies config `weight:=0` drains — it never stores 0 (clamped to 1); drain
   is `UpdateWeight`-only.
6. Manual weight edits at the registry are visible to watchers but a later in-process
   `UpdateWeight` overwrites them (no reconciliation).

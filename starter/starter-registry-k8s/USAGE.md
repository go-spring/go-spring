# starter-registry-k8s Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against the
starter source (`starter.go`, `config.go`, `dns.go`, `endpointslice.go`) and the runnable
[example/](example/) (unit tests + boot smoke in `check.sh`; full end-to-end needs a live cluster).
**Kubernetes' own semantics (Services, EndpointSlices, DNS, readiness) are
[Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/)** — everything below is
go-spring's increment.

**Activation**: one `spring.registry.k8s.<name>` block becomes one backend bean named `k8s.<name>`
(`starter.go:39-47`). This starter is **discovery-only**: inside a cluster the platform already registers every
Pod behind a Service, so it contributes no registrar and does not read the family-wide
`${spring.registry.service-name}` / `.addr` keys (`config.go:52-60`).

**Two modes** (`Config.Mode`, `config.go:47-50`):

| mode | mechanism | freshness | needs | metadata |
|------|-----------|-----------|-------|----------|
| `dns` (default) | headless-Service SRV/A lookup, cached per `refresh-interval` | `refresh-interval` (default 10s) | cluster DNS only, no RBAC | SRV weight only |
| `endpointslice` | client-go informer on EndpointSlices | real-time (watch events) | client-go + get/list/watch RBAC | zone, ready state |

---

## 1. Complete worked project

Two sides: a **provider** (any Deployment behind a headless Service — Kubernetes itself is the registrar)
and a **consumer** (a Go-Spring app that resolves the Service through this starter). File tree:

```
demo/
├── go.mod
├── main.go
├── conf/
│   └── app.properties
└── deploy/
    ├── demo-service.yaml    # provider: Deployment + headless Service "demo"
    ├── consumer.yaml        # consumer Deployment
    └── rbac.yaml            # only for endpointslice mode
```

**go.mod**:

```
require (
    go-spring.org/spring                v1.3.x
    go-spring.org/starter-registry-k8s  latest
    go-spring.org/starter-redigo        latest   // the consuming client (any discovery-aware starter works)
)
```

**main.go**:

```go
package main

import (
    _ "demo/conf"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-registry-k8s"
    _ "go-spring.org/starter-redigo"
)

func main() { gs.Run() }
```

**conf/app.properties** — one k8s backend plus a Redis-style client consuming it:

```properties
spring.app.name=demo-consumer

# The block "main" under spring.registry.k8s declares the bean "k8s.main",
# which is the name a client's `discovery:` field references.
spring.registry.k8s.main.mode=dns                 # or: endpointslice
spring.registry.k8s.main.namespace=default
# dns SRV mode: port-name "grpc" queries _grpc._tcp.demo.default.svc.cluster.local
spring.registry.k8s.main.port-name=grpc
spring.registry.k8s.main.cluster-domain=cluster.local
spring.registry.k8s.main.refresh-interval=5s      # dns re-resolve period
# endpointslice mode extras:
#   spring.registry.k8s.main.kubeconfig=           # empty = in-cluster ServiceAccount
#   spring.registry.k8s.main.resync-period=0       # 0 = event-driven only

# Consumer: a redigo client whose addresses come from the backend above.
spring.redis.demo.service-name=demo
spring.redis.demo.discovery=k8s.main
spring.redis.demo.conn-max-lifetime=30s           # short-lived conns follow endpoint churn
```

(The example/ app demonstrates the direct form — the injected `k8s.main` backend bean then `Resolve("demo")` —
which is what every client starter does under the hood; see `example/example.go:77-100`.)

**Provider manifests** (verbatim from `example/deploy/demo-service.yaml`): a 2-replica Deployment with a
readiness probe and a `clusterIP: None` Service with a named port `grpc`. Headless is required for dns
mode — a normal Service resolves to the ClusterIP, not per-Pod records.

**Verify** (inside a cluster, mirroring `example/README.md` + `check.sh`):

```bash
kubectl apply -f deploy/demo-service.yaml
kubectl apply -f deploy/rbac.yaml                  # endpointslice mode only
kubectl apply -f deploy/consumer.yaml
kubectl logs deploy/registry-k8s-example -f
# expect one line per ready Pod: "endpoint addr=10.x.x.x:80 healthy=true zone="
```

Out-of-cluster development: set `kubeconfig=/home/me/.kube/config` and `mode=endpointslice`;
dns mode needs cluster DNS, so outside a cluster resolve fails with a lookup error (the example
treats this as a warning and exits cleanly — `example/example.go:97-99`).

---

## 2. Assembly & timing

Lifecycle (all in `starter.go`):

1. **Bean-registration phase**: `gs.Module(gs.OnProperty("spring.registry.k8s"), ...)` runs
   `conf.BindEach` over every block and provides one named bean per block — `"k8s." + <name>` — with
   `Destroy(destroyBackendBean)` (`starter.go:39-47`). Declaring happens before any client starter's bean
   constructor runs, so a client may cite `k8s.<name>` in its own config without racing the declaration.
2. **Construction is deferred to injection time** (`newBackendBean`, `starter.go:52-59`): a backend is built
   only when something cites its bean name. An invalid mode or an unreachable API server therefore fails the
   first injection, not startup ordering.
3. A duplicate bean name is rejected by the container — two blocks can never silently shadow each other.
4. Shutdown calls the bean destructor, which invokes `Close` only on backends implementing `io.Closer`
   (`starter.go:64-69`): dns backends hold nothing, informer-backed ones stop their watches.
5. Logs: `declared k8s discovery backend bean name=%s mode=%s namespace=%s` and
   `creating k8s discovery backend mode=%s namespace=%s` (`starter.go:57, 66`); informer warnings
   (`endpointslice.go:209, 218`). Every line carries this starter's own tag `_app_registry_k8s`, so it is tuned
   independently of the main log via `logger.<name>.tag=_app_registry_k8s`.

### 2.1 Freshness — endpointslice mode (the interesting one)

There is no public `Watch`: freshness is internal to the backend, which caches one snapshot per Service
(`esEntry`). `Resolve` (`endpointslice.go:143-158`):

1. The first `Resolve` of a name pays the **seed list** — one `EndpointSlices.List` filtered by the label
   selector `kubernetes.io/service-name=<name>` (`endpointslice.go:135-137, 161-...`) — then starts a scoped
   informer goroutine.
2. The informer factory is built with the namespace and the same label selector, and with
   `ResyncPeriod` (`endpointslice.go:181-191`).
3. Handlers only **signal** (non-blocking enqueue into a cap-1 channel); the goroutine owns all writes to the
   entry (`endpointslice.go:193-209`).
4. `WaitForCacheSync` gates the snapshot; on failure the backend logs a warning and keeps serving seed/stale
   snapshots forever rather than erroring every later call (`endpointslice.go:213-217`).
5. Each add/update/delete recomputes from the lister cache: slices → `slicesToEndpoints` (port per
   `pickPort`, `Healthy = Conditions.Ready == nil || *Ready`, `zone` into Metadata) → sort by address
   (`endpointslice.go:228-238, 266-291`).
6. `Resolve` later reads the cached set and applies the per-call scheme filter
   (`discovery.FilterByScheme`), so `e.eps` holds the FULL unfiltered set (`endpointslice.go:66-71`).
7. `Close` stops every tracked watch — a shutdown safety net for a consumer that never let go
   (`endpointslice.go:251-262`, wired as the bean destructor).

Downstream, a client starter builds a `discovery.Resolver` from the backend
(`discovery.NewResolver(ctx, backend, serviceName, opts...)`, `cloud/discovery/discovery.go:227`) — a plain
`func() ([]Endpoint, error)` (`cloud/discovery/discovery.go:209`) that a loadbalance `Pool` calls per pick.
That is the "endpoint push" into client connection pools.

### 2.2 Freshness — dns mode

No push exists, so the snapshot is cached with a TTL: `Resolve` re-fetches only when the cached set is nil or
older than `RefreshInterval` (`dns.go:90-118`). A **failed refresh serves the stale snapshot** and only errors
when there is nothing cached yet (`dns.go:106-115`). SRV records carry per-endpoint weight
(`dns.go:130-146`); A records pair each IP with the configured `port` and mark endpoints healthy
(`dns.go:150-165`) — a headless Service publishes records only for ready addresses. Both paths sort by address
for a stable snapshot (`dns.go:169-171`).

### 2.3 REMOVAL / drain path

Kubernetes has no `UpdateWeight`; draining a Pod is the platform's job and flows through the same snapshot:

- **Scale down / pod delete** → EndpointSlice shrinks → informer delete event → recompute → the
  address disappears from the snapshot → the client pool stops picking it.
- **Readiness flips false** (probe failure) → the endpoint stays in the slice but `Conditions.Ready`
  becomes false → `Healthy=false` → excluded by `discovery.Allows` at pick time
  (`endpointslice.go:276`, `cloud/discovery/discovery.go:65-77`). dns mode instead sees the record
  disappear (headless DNS publishes ready addresses only, `dns.go:82-89`), after a `refresh-interval` lag.
- Weight semantics: only SRV records carry weight (`dns.go:140`); `Weight == 0` there is subject to the
  pool's soft-drain filter (`cloud/loadbalance/pool.go:93-134`).

---

## 3. Configuration reference

All keys live under `${spring.registry.k8s.<block-name>}`; the block name becomes the bean name
`k8s.<block-name>`. Verified against `config.go:63-97`.

| key | type | default | behavior | misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `mode` | string | `dns` | `dns` or `endpointslice` | anything else: startup error `invalid mode` (`config.go:113-114`) |
| `namespace` | string | `default` | namespace of the target Service | resolve finds nothing (empty endpoint set, not an error) |
| `port-name` | string | `` | dns: SRV query `_<port-name>._tcp.<fqdn>`; endpointslice: match slice port by name | wrong name: dns mode returns lookup error / slice skipped (`endpointslice.go:297-305`) |
| `port` | int | `0` | numeric port when `port-name` empty | ⚠ dns mode with **both** `port-name` and `port` empty fails startup (`config.go:105-108`); `0` in endpointslice mode falls back to the slice's sole port (`endpointslice.go:306-309`) |
| `cluster-domain` | string | `cluster.local` | FQDN suffix in dns mode | wrong domain → NXDOMAIN on every resolve; **ignored** in endpointslice mode |
| `refresh-interval` | duration | `10s` | dns snapshot TTL | `0` behaves as `10s` (`dns.go:91-94`); ignored in endpointslice mode |
| `kubeconfig` | string | `` | endpointslice auth; empty = in-cluster ServiceAccount | out-of-cluster with empty value → error `in-cluster config (set kubeconfig ...)` (`endpointslice.go:127-130`); ignored in dns mode |
| `resync-period` | duration | `0` | informer periodic resync | `0` = event-driven only; ignored in dns mode |

Couplings: dns mode needs a **headless** Service (a ClusterIP Service resolves to one virtual address);
endpointslice mode needs the RBAC of `deploy/rbac.yaml` (get/list/watch on `discovery.k8s.io/endpointslices`).
The client's `discovery:` field must equal the whole bean name — `k8s.<block>` — not the bare block name.

Do **not** set `${spring.registry.service-name}` / `.addr` expecting registration: this backend contributes no
registrar, and with no registrar backend configured at all the registry core fails startup with
`no registry center is configured` rather than publishing nothing silently.

---

## 4. Verification & failure drills

Prereq: `kubectl` with a dev cluster; provider applied per §1.

1. **Endpoints appear**: `kubectl logs deploy/registry-k8s-example` — one `endpoint addr=...` per ready
   Pod; endpointslice mode also prints `zone=` when the cluster is zone-aware.
2. **Scale up/down**: `kubectl scale deploy/demo --replicas=4` → within seconds endpointslice mode
   pushes 4 addresses; `--replicas=1` → the removed addresses vanish from the next snapshot.
3. **Instance loss (TTL analogue)**: `kubectl delete pod -l app=demo --grace-period=0` — the Pod's
   endpoint leaves the EndpointSlice via the kube-controller-manager, the informer fires, the pool stops
   picking the address. In dns mode the same change surfaces only after `refresh-interval`.
4. **Not-ready drain (weight=0 analogue)**: break the readiness probe
   (`kubectl patch deploy/demo -p '{"spec":{"template":{"spec":{"containers":[{"name":"demo","readinessProbe":{"tcpSocket":{"port":9999}}}]}}}}'`)
   → rolling Pods turn not-ready → endpointslice mode shows `healthy=false` (excluded by Allows);
   dns mode drops the record entirely.
5. **Watch channel hygiene**: stop the consumer — the bean destructor calls `Close`, stopping every informer
   even if a consumer leaked its snapshot (`starter.go:64-69`, `endpointslice.go:251-262`).
6. **Boot fail-fast (endpointslice)**: run out-of-cluster without `kubeconfig` → error naming the fix at
   first injection. Unit-test coverage: `dns_test.go`, `endpointslice_test.go` (fake resolver + fake
   clientset exercise resolve, port selection, ready/zone metadata, cache refresh — this is what `check.sh`
   actually asserts).

Runtime logs all carry the tag `_app_registry_k8s` (`log.RegisterAppTag("registry_k8s", "")`,
`starter.go:34`); tune verbosity via `logger.<name>.tag=_app_registry_k8s`. This starter emits a trace-free
metric-only read side (`discovery.sync_total` / `discovery.cache.age_seconds`, `cloud/discovery/observe.go`);
clients observe through their own pools.

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| `resolve "demo" failed ... lookup demo...: no such host` | not in a cluster / wrong `cluster-domain` / Service not headless | run in-cluster, fix domain, set `clusterIP: None` |
| `resolve` returns empty, no error | wrong `namespace`, or no ready endpoints | check `kubectl get endpointslices -l kubernetes.io/service-name=demo` |
| startup error `dns mode requires port-name (SRV) or port` | neither `port-name` nor `port` set | set one (`config.go:105-108`) |
| error `in-cluster config` | endpointslice mode outside a cluster | set `kubeconfig` (`endpointslice.go:127-130`) |
| `informer handler ... failed` / `cache sync ... failed` in logs | RBAC denied, or API server unreachable at watch start | apply `deploy/rbac.yaml`, check connectivity; the backend keeps serving the seed snapshot meanwhile (`endpointslice.go:206-217`) |
| endpoint stuck after scale change (dns mode) | snapshot TTL lag | lower `refresh-interval`, or switch to `endpointslice` |
| slice with multiple unnamed ports yields nothing | `pickPort` cannot choose (`endpointslice.go:306-309`) | set `port-name` or `port` |
| `:0` addresses never appear | a portless slice is skipped, not emitted with 0 | set `port-name` matching the Service port name |

---

## 6. Design health

| metric | value |
|--------|-------|
| config keys | 8 |
| required (mode-conditional) | 1 (`port-name` **or** `port` in dns mode) |
| quickstart external deps | 1 (a Kubernetes cluster) |
| "notes/gotchas" | 4 |

Suspect ledger:

- dns vs endpointslice differ in drain fidelity (not-ready → Healthy=false vs record gone) — inherent
  to DNS; documented, not fixable.
- A failed dns refresh silently serves the stale snapshot; only the first fetch errors
  (`dns.go:106-115`). Deliberate (a blip must not empty a pool), but it means a permanently broken DNS
  setup looks healthy once seeded.
- `port`-only endpointslice config silently depends on slices having exactly one port
  (`endpointslice.go:306-309`) — a multi-port Service misconfigures quietly.

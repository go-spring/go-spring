# starter-discovery-k8s Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified against the
starter source (`starter.go`, `config.go`, `dns.go`, `endpointslice.go`) and the runnable
[example/](example/) (unit tests + boot smoke in `check.sh`; full end-to-end needs a live cluster).
**Kubernetes' own semantics (Services, EndpointSlices, DNS, readiness) are
[Kubernetes documentation](https://kubernetes.io/docs/concepts/services-networking/)** — everything below is
go-spring's increment.

**Activation**: one Kubernetes discovery backend is registered per entry under `${spring.discovery.k8s}`
(`starter.go:41`). This starter is **discovery-only**: inside a cluster the platform already registers every
Pod behind a Service, so there is no registrar — the consumer reads, Kubernetes writes (`config.go:20-25`).

**Two modes** (`Config.Mode`, `config.go:47-50`):

| mode | mechanism | freshness | needs | metadata |
|------|-----------|-----------|-------|----------|
| `dns` (default) | headless-Service SRV/A lookup, polled | DNS TTL + `refresh-interval` | cluster DNS only, no RBAC | SRV weight only |
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
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-discovery-k8s latest
    go-spring.org/starter-redigo        latest   // the consuming client (any discovery-aware starter works)
)
```

**main.go**:

```go
package main

import (
    _ "demo/conf"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-discovery-k8s"
    _ "go-spring.org/starter-redigo"
)

func main() { gs.Run() }
```

**conf/app.properties** — one k8s backend plus a Redis-style client consuming it:

```properties
spring.app.name=demo-consumer

# The discovery backend. Key after the prefix ("k8s") is the backend name a
# client's `discovery:` field references.
spring.discovery.k8s.k8s.mode=dns                 # or: endpointslice
spring.discovery.k8s.k8s.namespace=default
# dns SRV mode: port-name "grpc" queries _grpc._tcp.demo.default.svc.cluster.local
spring.discovery.k8s.k8s.port-name=grpc
spring.discovery.k8s.k8s.cluster-domain=cluster.local
spring.discovery.k8s.k8s.refresh-interval=5s      # dns re-resolve period
# endpointslice mode extras:
#   spring.discovery.k8s.k8s.kubeconfig=          # empty = in-cluster ServiceAccount
#   spring.discovery.k8s.k8s.resync-period=0      # 0 = event-driven only

# Consumer: a redigo client whose addresses come from the backend above.
spring.redis.demo.service-name=demo
spring.redis.demo.discovery=k8s
spring.redis.demo.conn-max-lifetime=30s           # short-lived conns follow endpoint churn
```

(The example/ app demonstrates the direct form — `discovery.GetDiscovery("k8s")` then `Resolve("demo")` —
which is what every client starter does under the hood; see `example/example.go:71-97`.)

**Provider manifests** (verbatim from `example/deploy/demo-service.yaml`): a 2-replica Deployment with a
readiness probe and a `clusterIP: None` Service with a named port `grpc`. Headless is required for dns
mode — a normal Service resolves to the ClusterIP, not per-Pod records.

**Verify** (inside a cluster, mirroring `example/README.md` + `check.sh`):

```bash
kubectl apply -f deploy/demo-service.yaml
kubectl apply -f deploy/rbac.yaml                  # endpointslice mode only
kubectl apply -f deploy/consumer.yaml
kubectl logs deploy/discovery-k8s-example -f
# expect one line per ready Pod: "endpoint addr=10.x.x.x:80 healthy=true zone="
```

Out-of-cluster development: set `kubeconfig=/home/me/.kube/config` and `mode=endpointslice`;
dns mode needs cluster DNS, so outside a cluster resolve fails with a lookup error (the example
treats this as a warning and exits cleanly — `example/example.go:83-86`).

---

## 2. Assembly & timing

Lifecycle (all in `starter.go`):

1. **Bean-registration phase**: `gs.Module(gs.OnProperty("spring.discovery.k8s"), ...)` runs
   `conf.BindEach` over every entry; each backend is built eagerly and pushed into the global
   `discovery` registry **before any client starter's bean constructor runs** (`starter.go:31-40`).
   Eager build means a missing ServiceAccount or bad kubeconfig fails at startup
   (`endpointslice.go:71-86`), not on first resolve.
2. A duplicate backend name is skipped with a log line, not a panic (`starter.go:46-48`).
3. One `manager` bean is provided solely so container shutdown closes every informer-backed backend
   (`starter.go:61, 69-86`). dns backends hold nothing to close.
4. Startup log: `registered k8s discovery backend name=%s mode=%s` at `log.TagAppDef`
   (`starter.go:55`).

### 2.1 WATCH path — endpointslice mode (the interesting one)

`Watch(ctx, name)` (`endpointslice.go:133-221`):

1. A shared-informer factory is created scoped to the namespace and to the label selector
   `kubernetes.io/service-name=<name>` (`endpointslice.go:41, 105-107, 135-142`).
2. Informer handlers only **signal** (non-blocking enqueue into a cap-1 channel); a single goroutine
   owns the result channel as sole writer and closer — no send-after-close race
   (`endpointslice.go:147-160`).
3. `WaitForCacheSync` gates the initial list; failure returns an error and the watch never starts
   (`endpointslice.go:167-170`).
4. A **snapshot** is computed from the informer cache: list slices → `slicesToEndpoints` (port per
   `pickPort`, `Healthy = Conditions.Ready == nil || *Ready`, `zone` into Metadata) →
   `FilterByScheme` → sort by address (`endpointslice.go:175-195, 247-294`).
5. **Change detection / dedup**: the snapshot's address set is folded into a string key; an unchanged
   key is skipped — the cache-sync burst fires one Add per object and would otherwise queue stale
   duplicates (`endpointslice.go:184-190`, mirroring dns's `addrKey`, `dns.go:179-186`).
6. The seed snapshot is pushed **before** the writer goroutine starts, so the first channel result is
   the state at watch time (`endpointslice.go:198-199`); afterwards every add/update/delete enqueues
   one recompute. The channel closes on ctx cancel or `Close()` (`endpointslice.go:205-219`).

Downstream, a `discovery.Resolver` turns the channel into an `Endpoints()` snapshot that a
loadbalance `Pool` reads per `Pick` — that is the "endpoint push" into client connection pools.

### 2.2 WATCH path — dns mode

No push exists, so `Watch` polls: initial `Resolve` seeds the channel, then a ticker re-resolves every
`refresh-interval`; a changed `addrKey` is pushed, unchanged results and transient lookup errors are
skipped (`dns.go:129-169`). SRV records carry per-endpoint weight (`dns.go:87-103`); A records pair
each IP with the configured `port` (`dns.go:107-122`).

### 2.3 REMOVAL / drain path

Kubernetes has no `UpdateWeight`; draining a Pod is the platform's job and flows through the same watch:

- **Scale down / pod delete** → EndpointSlice shrinks → informer delete event → recompute → the
  address disappears from the snapshot → the client pool stops picking it.
- **Readiness flips false** (probe failure) → the endpoint stays in the slice but `Conditions.Ready`
  becomes false → `Healthy=false` → excluded by `discovery.Eligible` at pick time
  (`endpointslice.go:257`, `cloud/discovery/discovery.go:65-77`). dns mode instead sees the record
  disappear (headless DNS publishes ready addresses only, `dns.go:67-70`), after DNS-TTL +
  refresh-interval lag.
- Weight semantics: only SRV records carry weight (`dns.go:97`); `Weight == 0` there is subject to the
  pool's soft-drain filter (`cloud/loadbalance/pool.go:93-134`).

---

## 3. Configuration reference

All keys live under `${spring.discovery.k8s.<backend-name>}`. Verified against `config.go:58-94`.

| key | type | default | behavior | misconfiguration consequence |
|-----|------|---------|----------|------------------------------|
| `mode` | string | `dns` | `dns` or `endpointslice` | anything else: startup error `invalid mode` (`config.go:108-110`) |
| `namespace` | string | `default` | namespace of the target Service | resolve/watch finds nothing (empty endpoint set, not an error) |
| `port-name` | string | `` | dns: SRV query `_<port-name>._tcp.<fqdn>`; endpointslice: match slice port by name | wrong name: dns mode returns lookup error / slice skipped (`endpointslice.go:279-285`) |
| `port` | int | `0` | numeric port when `port-name` empty | ⚠ dns mode with **both** `port-name` and `port` empty fails startup (`config.go:100-103`); `0` in endpointslice mode falls back to the slice's sole port (`endpointslice.go:290-293`) |
| `cluster-domain` | string | `cluster.local` | FQDN suffix in dns mode | wrong domain → NXDOMAIN on every resolve; **ignored** in endpointslice mode |
| `refresh-interval` | duration | `10s` | dns watch poll period | `0` behaves as `10s` (`dns.go:131-133`); ignored in endpointslice mode |
| `kubeconfig` | string | `` | endpointslice auth; empty = in-cluster ServiceAccount | out-of-cluster with empty value → startup error `in-cluster config (set kubeconfig ...)` (`endpointslice.go:97-100`); ignored in dns mode |
| `resync-period` | duration | `0` | informer periodic resync | `0` = event-driven only; ignored in dns mode |

Couplings: dns mode needs a **headless** Service (a ClusterIP Service resolves to one virtual address);
endpointslice mode needs the RBAC of `deploy/rbac.yaml` (get/list/watch on `discovery.k8s.io/endpointslices`).
The backend name (map key) must equal the client's `discovery:` field — default client value is
`default`, not `k8s`.

---

## 4. Verification & failure drills

Prereq: `kubectl` with a dev cluster; provider applied per §1.

1. **Endpoints appear**: `kubectl logs deploy/discovery-k8s-example` — one `endpoint addr=...` per ready
   Pod; endpointslice mode also prints `zone=` when the cluster is zone-aware.
2. **Scale up/down**: `kubectl scale deploy/demo --replicas=4` → within seconds endpointslice mode
   pushes 4 addresses; `--replicas=1` → the removed addresses vanish from the next snapshot.
3. **Instance loss (TTL analogue)**: `kubectl delete pod -l app=demo --grace-period=0` — the Pod's
   endpoint leaves the EndpointSlice via the kube-controller-manager, the informer fires, the pool stops
   picking the address. In dns mode the same change surfaces only after DNS TTL + `refresh-interval`.
4. **Not-ready drain (weight=0 analogue)**: break the readiness probe
   (`kubectl patch deploy/demo -p '{"spec":{"template":{"spec":{"containers":[{"name":"demo","readinessProbe":{"tcpSocket":{"port":9999}}}]}}}}'`)
   → rolling Pods turn not-ready → endpointslice mode shows `healthy=false` (excluded by Eligible);
   dns mode drops the record entirely.
5. **Watch channel hygiene**: stop the consumer — `manager.Destroy` closes every informer even if a
   consumer leaked its watch context (`starter.go:79-86`, `endpointslice.go:230-243`).
6. **Boot fail-fast (endpointslice)**: run out-of-cluster without `kubeconfig` → startup error naming
   the fix. Unit-test coverage: `dns_test.go`, `endpointslice_test.go` (fake resolver + fake clientset
   exercise resolve, port selection, ready/zone metadata, watch-on-scale — this is what `check.sh`
   actually asserts).

Runtime logs all carry `log.TagAppDef`; there are no metrics/traces emitted by this starter (clients
observe through their own pools).

---

## 5. Troubleshooting

| symptom | cause | fix |
|---------|-------|-----|
| `resolve "demo" failed ... lookup demo...: no such host` | not in a cluster / wrong `cluster-domain` / Service not headless | run in-cluster, fix domain, set `clusterIP: None` |
| `resolve` returns empty, no error | wrong `namespace`, or no ready endpoints | check `kubectl get endpointslices -l kubernetes.io/service-name=demo` |
| startup error `dns mode requires port-name (SRV) or port` | neither `port-name` nor `port` set | set one (`config.go:101-103`) |
| startup error `in-cluster config` | endpointslice mode outside a cluster | set `kubeconfig` (`endpointslice.go:97-100`) |
| informer watch fails / RBAC errors in logs | ServiceAccount lacks endpointslice permissions | apply `deploy/rbac.yaml`; ClusterRole for cross-namespace |
| `cache sync failed for "demo"` | API server unreachable or watch rejected at watch start | check API-server connectivity and RBAC; watch returns error, no channel |
| endpoint stuck after scale change (dns mode) | DNS TTL + poll interval lag | lower `refresh-interval`, or switch to `endpointslice` |
| slice with multiple unnamed ports yields nothing | `pickPort` cannot choose (`endpointslice.go:290-293`) | set `port-name` or `port` |
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
- Backend name default mismatch: clients default `discovery=default`, this starter has no default name —
  every config must restate the pair; candidate for a shared convention.
- `port`-only endpointslice config silently depends on slices having exactly one port
  (`endpointslice.go:290-293`) — a multi-port Service misconfigures quietly.

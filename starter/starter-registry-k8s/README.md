# starter-registry-k8s

[English](README.md) | [中文](README_CN.md)

`starter-registry-k8s` provides **Kubernetes-native client-side service
discovery** for Go-Spring. The platform registers every Pod behind a Service
in-cluster, so an application discovers peers through that capability instead of
standing up a second registry (Nacos/Consul).

Blank-importing this starter and declaring a `spring.registry.k8s.<name>` block
provides a `discovery.Discovery` backend bean (from `cloud/discovery`) named
`k8s.<name>` — the same block-to-bean naming as the other registry backends
(`consul.<name>`, `etcd.<name>`, ...). Any client starter that supports
discovery — Redis, GORM, ... — resolves a Kubernetes **Service name** to live Pod
endpoints by setting its `discovery: k8s.<name>` field. This starter does
**client-side discovery only**; it contributes no registrar (the platform
registers Pods).

## Two modes

| Mode | Mechanism | Dependencies | RBAC | Trade-off |
| --- | --- | --- | --- | --- |
| `dns` (default) | Headless Service DNS SRV/A records via `net.Resolver` | none | none | Zero-permission and simple, but change detection is bounded by DNS TTL + `refresh-interval`, and there is no per-endpoint metadata. |
| `endpointslice` | client-go informer on `discovery.k8s.io/v1` EndpointSlices | client-go | `get/list/watch endpointslices` | Real-time (fires on scale up/down) and carries Pod metadata (zone, ready state); needs a Kubernetes client and RBAC. |

## Installation

```bash
go get go-spring.org/starter-registry-k8s
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-registry-k8s"
```

### 2. Declare a backend

DNS mode against a headless Service, using an SRV query on the named port:

```properties
spring.registry.k8s.main.mode=dns
spring.registry.k8s.main.namespace=default
spring.registry.k8s.main.port-name=grpc
spring.registry.k8s.main.cluster-domain=cluster.local
spring.registry.k8s.main.refresh-interval=5s
```

EndpointSlice mode (real-time; requires RBAC — see [example/deploy/rbac.yaml](example/deploy/rbac.yaml)):

```properties
spring.registry.k8s.main.mode=endpointslice
spring.registry.k8s.main.namespace=default
spring.registry.k8s.main.port-name=grpc
# kubeconfig is empty for in-cluster auth; set a path to run out-of-cluster.
```

### 3. Consume it from a client

The block above declares the bean `k8s.main`, which is what a client
references. For example a Redis client resolves its address through it:

```properties
spring.go-redis.instances.cache.service-name=my-redis   # the Kubernetes Service name
spring.go-redis.instances.cache.discovery=k8s.main       # this backend
```

The Redis client now dials a live Pod of the `my-redis` Service, refreshed as
Pods come and go. See [example/example.go](example/example.go) for resolving a
Service directly through the injected backend bean.

Do **not** set `spring.registry.service-name` / `.addr` expecting this backend
to register the instance: it contributes no registrar, and with no registrar
backend configured the registry core fails startup rather than silently
publishing nothing.

## Configuration

Bound under `spring.registry.k8s.<name>`:

| Key | Default | Applies to | Description |
| --- | --- | --- | --- |
| `mode` | `dns` | both | `dns` or `endpointslice`. |
| `namespace` | `default` | both | Namespace of the target Service. |
| `port-name` | (empty) | both | Named port to select. In `dns` mode a non-empty value triggers an SRV query; empty falls back to an A query with `port`. |
| `port` | `0` | both | Numeric port used when `port-name` is empty (required for `dns` A-record mode). |
| `cluster-domain` | `cluster.local` | dns | Cluster DNS suffix used to build the Service FQDN. |
| `refresh-interval` | `10s` | dns | How often the DNS watcher re-resolves to detect changes. |
| `kubeconfig` | (empty) | endpointslice | Path to a kubeconfig; empty uses in-cluster ServiceAccount auth. |
| `resync-period` | `0` | endpointslice | Informer resync period; `0` is event-driven only. |

## How It Works

- The block-to-bean mapping is declared during the container's bean-registration
  phase, before any client constructor runs — so a Redis/GORM client naming
  `k8s.<name>` in its config resolves to a bean the container already knows
  about. The backend itself is built at injection time, so an uncited block
  costs nothing.
- **DNS mode** resolves `<service>.<namespace>.svc.<cluster-domain>`. With
  `port-name` set it issues an SRV query (`_<port-name>._tcp.<fqdn>`) for
  address+port; otherwise an A query paired with `port`. The watcher polls on
  `refresh-interval` and emits a snapshot only when the endpoint set changes.
- **EndpointSlice mode** runs a client-go shared informer scoped to the
  Service's EndpointSlices (label `kubernetes.io/service-name=<service>`). Each
  add/update/delete recomputes the snapshot from the informer cache. Endpoints
  carry `Healthy` from the slice's `Ready` condition and `zone` metadata.
- On shutdown the backend bean's destructor closes the backend; only
  informer-backed backends hold resources to release.

### Log tag

Runtime logs from this module carry the tag `_app_registry_k8s` (k8s registry). Tune them independently of the
main log by binding a logger to the tag:

```properties
logger.registry_k8s.type=Logger
logger.registry_k8s.level=WARN
logger.registry_k8s.tag=_app_registry_k8s
```

## Verifying in a cluster

The unit tests cover both modes with a fake resolver and the client-go fake
clientset. Full end-to-end verification needs a real cluster:

```bash
kubectl apply -f example/deploy/demo-service.yaml   # target Deployment + headless Service
kubectl apply -f example/deploy/rbac.yaml           # only for endpointslice mode
# build/push an image for example/ and apply example/deploy/consumer.yaml, then:
kubectl logs deploy/registry-k8s-example            # prints the resolved Pod endpoints
kubectl scale deploy/demo --replicas=4              # candidate pool updates live
```

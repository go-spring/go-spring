# starter-discovery-k8s

[English](README.md) | [中文](README_CN.md)

`starter-discovery-k8s` provides **Kubernetes-native client-side service
discovery** for Go-Spring. Inside a cluster the platform already registers every
Pod behind a Service, so an application should discover peers through that
capability instead of standing up a second external registry (Nacos/Consul).

Blank-importing this starter and declaring a `spring.discovery.k8s.<name>` entry
registers a `discovery.Discovery` backend (from `stdlib/discovery`) under
`<name>`. Any client starter that supports discovery — Redis, GORM, ... —
resolves a Kubernetes **Service name** to live Pod endpoints by setting its
`discovery: <name>` field. This starter does **client-side discovery only**; it
never registers a service (the platform does that).

## Two modes

| Mode | Mechanism | Dependencies | RBAC | Trade-off |
| --- | --- | --- | --- | --- |
| `dns` (default) | Headless Service DNS SRV/A records via `net.Resolver` | none | none | Zero-permission and simple, but change detection is bounded by DNS TTL + `refresh-interval`, and there is no per-endpoint metadata. |
| `endpointslice` | client-go informer on `discovery.k8s.io/v1` EndpointSlices | client-go | `get/list/watch endpointslices` | Real-time (fires on scale up/down) and carries Pod metadata (zone, ready state); needs a Kubernetes client and RBAC. |

## Installation

```bash
go get go-spring.org/starter-discovery-k8s
```

## Quick Start

### 1. Import the package

```go
import _ "go-spring.org/starter-discovery-k8s"
```

### 2. Declare a backend

DNS mode against a headless Service, using an SRV query on the named port:

```properties
spring.discovery.k8s.k8s.mode=dns
spring.discovery.k8s.k8s.namespace=default
spring.discovery.k8s.k8s.port-name=grpc
spring.discovery.k8s.k8s.cluster-domain=cluster.local
spring.discovery.k8s.k8s.refresh-interval=5s
```

EndpointSlice mode (real-time; requires RBAC — see [example/deploy/rbac.yaml](example/deploy/rbac.yaml)):

```properties
spring.discovery.k8s.k8s.mode=endpointslice
spring.discovery.k8s.k8s.namespace=default
spring.discovery.k8s.k8s.port-name=grpc
# kubeconfig is empty for in-cluster auth; set a path to run out-of-cluster.
```

### 3. Consume it from a client

The backend name (`k8s` above) is what a client references. For example a Redis
client resolves its address through it:

```properties
spring.go-redis.cache.service-name=my-redis   # the Kubernetes Service name
spring.go-redis.cache.discovery=k8s            # this backend
```

The Redis client now dials a live Pod of the `my-redis` Service, refreshed as
Pods come and go. See [example/main.go](example/main.go) for resolving a Service
directly through `discovery.MustGet`.

## Configuration

Bound under `spring.discovery.k8s.<name>`:

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

- Registration happens during the container's bean-registration phase, before
  any client constructor runs — so when a Redis/GORM client calls
  `discovery.MustGet("<name>")`, the backend is already present.
- **DNS mode** resolves `<service>.<namespace>.svc.<cluster-domain>`. With
  `port-name` set it issues an SRV query (`_<port-name>._tcp.<fqdn>`) for
  address+port; otherwise an A query paired with `port`. The watcher polls on
  `refresh-interval` and emits a snapshot only when the endpoint set changes.
- **EndpointSlice mode** runs a client-go shared informer scoped to the
  Service's EndpointSlices (label `kubernetes.io/service-name=<service>`). Each
  add/update/delete recomputes the snapshot from the informer cache. Endpoints
  carry `Healthy` from the slice's `Ready` condition and `zone` metadata.
- On shutdown a lifecycle bean stops any running informers.

## Verifying in a cluster

The unit tests cover both modes with a fake resolver and the client-go fake
clientset. Full end-to-end verification needs a real cluster:

```bash
kubectl apply -f example/deploy/demo-service.yaml   # target Deployment + headless Service
kubectl apply -f example/deploy/rbac.yaml           # only for endpointslice mode
# build/push an image for example/ and apply example/deploy/consumer.yaml, then:
kubectl logs deploy/discovery-k8s-example           # prints the resolved Pod endpoints
kubectl scale deploy/demo --replicas=4              # candidate pool updates live
```

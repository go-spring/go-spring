# Service Registration & Discovery (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A tour of the **service registries** commonly used in the Go microservice
community, each shown as **one runnable provider + consumer** example wired the
Go-Spring way. The point is not the RPC framework or the protocol — it is the
**registry**: on startup the provider *registers* a service into the registry;
the consumer never learns the provider's `host:port` and instead *discovers* a
live address from that same registry.

Each example is a self-contained Go module with its own `docker-compose.yml`
that brings up just the registry. Run any one in isolation.

## The registries

| Registry | Example | Framework | Registry driver | Default addr | Notes |
| --- | --- | --- | --- | --- | --- |
| **etcd** | [`etcd/`](etcd) | dubbo-go (Triple) | `etcdv3` | `127.0.0.1:2379` | Raft-backed KV; the de-facto default for cloud-native Go services. |
| **Nacos** | [`nacos/`](nacos) | dubbo-go (Triple) | `nacos` | `127.0.0.1:8848` | Registry + config center; built-in console at `:8848/nacos`. |
| **ZooKeeper** | [`zookeeper/`](zookeeper) | dubbo-go (Triple) | `zookeeper` | `127.0.0.1:2181` | The classic Dubbo registry; battle-tested, ZAB-consistent. |
| **Polaris** | [`polaris/`](polaris) | dubbo-go (Triple) | `polaris` | `127.0.0.1:8091` | Tencent's service-governance platform (discovery + routing + circuit-breaking). |
| **Consul** | [`consul/`](consul) | Kitex (protobuf) | `registry-consul` | `127.0.0.1:8500` | HashiCorp; DNS/HTTP discovery with active TCP health checks. |

## How registries pair with frameworks

A registry is framework-agnostic infrastructure; what differs is how each RPC
framework plugs into it.

- **dubbo-go** ships first-class registry extensions for etcd, Nacos, ZooKeeper
  and Polaris (and more). Swapping registries is **config-only**: change
  `spring.dubbo.registries.<id>.protocol` and `.address` — the application code
  is byte-for-byte identical across the four dubbo-go examples here. That is why
  they make the best side-by-side comparison of registries.
- **Kitex** discovers through pluggable `kitex-contrib` resolvers/registrars.
  dubbo-go has no Consul extension, so Consul is demonstrated with Kitex via
  `github.com/kitex-contrib/registry-consul`, wired explicitly in
  `provider/server.go` (there is no starter for it).

## Kubernetes-native discovery (no registration step)

Inside Kubernetes there is no separate registry to register into: the platform
already registers every Pod behind a Service. Discovery is therefore
**client-side only** — resolve a Service name to its live Pod endpoints — which
is a different shape from the register-then-discover examples above, and why it
is a reusable **starter** rather than a demo here:
[`starter/starter-discovery-k8s`](../../starter/starter-discovery-k8s). It
implements `stdlib/discovery` in two modes: headless-Service DNS (zero
dependency, no RBAC) and an EndpointSlice informer (real-time, client-go +
`get/list/watch endpointslices` RBAC). The starter carries its own K8s manifests
and example; there is no docker-compose demo under this directory because the
mechanism needs a real cluster, not a single container.

## Common shape of every example

```
                ┌──────────────┐
   register     │   registry   │   discover
  ┌────────────▶│              │◀────────────┐
  │             └──────────────┘             │
  │ service name                             │ resolve live provider addr
  │                                          │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │                        │ one-shot   │
│ long-lived │──────────────────────▶│ assert+exit│
└────────────┘                        └────────────┘
```

- The **provider** is long-lived: `gs.Run()` drives its lifecycle, and on
  shutdown (SIGTERM) it deregisters from the registry.
- The **consumer** runs server-less: it discovers the provider by service name,
  makes one call, asserts on the echoed value, then exits — so its exit code is
  the smoke-test result.
- Provider and consumer each `chdir` into their own directory and load their own
  `conf/app.properties`, so the two never share a config file.

## Run any example

```bash
cd etcd            # or nacos / zookeeper / polaris / consul

docker compose up -d          # bring up just that registry
go run ./provider &           # long-lived, registers the service
go run ./consumer             # discovers, calls, asserts, exits

# or the one-shot smoke test (up → call → tear down):
bash scripts/smoke-test.sh
```

Every example is a runnable demo, **not** a reusable starter module. For the
registry-agnostic RPC mechanics (protocols, code generation, observability) see
the framework examples under [`../dubbo-go`](../dubbo-go) and
[`../kitex`](../kitex).

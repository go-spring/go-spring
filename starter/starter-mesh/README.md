# starter-mesh

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-mesh` provides a single, process-global switch that degrades Go-Spring's
client-side service discovery (`spring/discovery`) and load balancing
(`spring/loadbalance`) to a pass-through when the application runs inside a
service mesh.

## Why

When a sidecar (Istio/Envoy, Linkerd, ...) is injected, it already performs
discovery and load balancing for you. Leaving the application's own client-side
discovery and load balancing enabled on top of that causes:

- **Double load balancing** — traffic is balanced once by the app and again by
  the sidecar, defeating the sidecar's routing policy.
- **Broken locality / outlier ejection** — the app's zone-aware routing and
  outlier ejection fight the mesh's, confusing failure-domain decisions.

Turning mesh mode on hands both concerns to the sidecar: names resolve to the
stable Kubernetes Service address (the ClusterIP the sidecar intercepts), and
the balancer stops selecting.

## Installation

```bash
go get go-spring.org/starter-mesh
```

## Quick Start

### 1. Import the `starter-mesh` Package

```go
import _ "go-spring.org/starter-mesh"
```

### 2. Enable mesh mode

```properties
spring.mesh.enabled=true
```

That is all. Every client starter that resolves a `service-name` through
`spring/discovery` and every `spring/loadbalance` Pool degrades automatically —
no per-component change. The switch is read once at startup, before any client
builds its dialer.

Set `spring.mesh.enabled=auto` to let the starter infer mesh mode from the
environment: it turns on only when a sidecar is detected (via mesh-injected
environment variables such as `ISTIO_META_*` / `LINKERD2_PROXY_*`). An explicit
`true` / `false` always wins — `auto` is only the inference used when you have
not decided.

## When to enable

| Deployment | `spring.mesh.enabled` | Rationale |
| --- | --- | --- |
| Kubernetes **with** a sidecar injected (Istio/Envoy, Linkerd) | `true` | The sidecar owns discovery + load balancing; the app must not balance again. |
| VM / bare metal / any deployment **without** a mesh | `false` (default) | No sidecar exists, so the app's own client-side discovery and load balancing must stay active. |
| Same artifact deployed both ways | `auto` | Infer per-environment from sidecar signals; explicit `true`/`false` still overrides. |

## What changes when enabled

- **`spring/discovery`** — `NewClientDialer` / `NewLiveDialer` skip resolving and
  watching the backend and expose a single stable endpoint whose address is the
  service name. In Kubernetes that name resolves via DNS to the Service
  ClusterIP, which the sidecar intercepts and load-balances across the pods.
- **`spring/loadbalance`** — a `Pool` returns that single endpoint directly,
  with no strategy selection and no outlier ejection (a lone mesh endpoint must
  never be evicted).
- **Tracing** is unaffected: the OTel global propagator still injects headers, so
  application and mesh spans stay correlated. When `starter-otel` is present it
  fills the `discovery.SetTraceInjector` seam, so wrapping an outbound transport
  with `discovery.TraceRoundTripper` stamps `traceparent` on every request and
  keeps the trace unbroken across the sidecar hop.
- **Readiness semantics are unchanged** — probes behave the same with or without
  mesh mode.

## What it does not do

- It does not talk to the mesh control plane or generate `VirtualService` /
  `DestinationRule` resources — that is deployment scaffolding, not this starter.
- It does not delete the client-side load-balancing code; it only degrades it at
  runtime, so flipping the switch back off restores full client-side behavior.

## Configuration

| Property | Default | Description |
| --- | --- | --- |
| `spring.mesh.enabled` | `false` | Service-mesh mode: `true`, `false`, or `auto` (infer from sidecar signals). Enable only when a sidecar is injected. |

## Example

[example/main.go](example/main.go) is a self-contained smoke test (no docker, no
external services). It runs the same client code twice — mesh off and mesh on —
and asserts that mesh off spreads requests across three real endpoints while mesh
on sends every request to a single stable endpoint without ever resolving the
discovery backend. Run it with `bash example/check.sh`.

## License

This project is licensed under the Apache License 2.0.

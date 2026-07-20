# starter-http-client

[English](README.md) | [中文](README_CN.md)

`starter-http-client` is the runtime half of Go-Spring's **declarative HTTP
client** — the equivalent of Spring's OpenFeign / `@HttpExchange`. You declare a
remote service as an interface in an IDL, generate the call sites with
[`gs-http-gen`](../../gs/gs-http-gen), and inject an already-assembled
`*http.Client` into the generated client. Service discovery, load balancing,
resilience and trace propagation are wired for you, so a microservice call no
longer means hand-building a dialer and a circuit breaker per client.

Go has no runtime proxy, so unlike Feign the call sites are produced at build
time by code generation rather than reflection — the same declarative
experience with no runtime magic. The generated code only imports stdlib
(`net/http` + `httpclt`); this starter supplies the transport it runs on.

## Installation

```bash
go get go-spring.org/starter-http-client
```

## How It Works

```
 generated Client (gs-http-gen)      starter-http-client
 ┌───────────────────────────┐       ┌──────────────────────────────────────┐
 │ Greet(ctx, req)           │       │ *http.Client.Transport =              │
 │   HTTPClient ─────────────┼──────▶│   resilience → discovery+LB → otelhttp │
 └───────────────────────────┘       └──────────────────────────────────────┘
```

The generated `Client` holds a single `*http.Client`. The starter registers one
`*http.Client` per configuration entry, whose `http.RoundTripper` is assembled
by [`spring/httpx`](../../spring/httpx) from three composable stdlib
abstractions, all behind the single `http.RoundTripper` seam:

* [`discovery`](../../spring/discovery) — when a `service-name` is set, a
  `LiveDialer` keeps a fresh endpoint snapshot;
* [`loadbalance`](../../spring/loadbalance) — a `Pool` picks one live endpoint
  per request (any registered strategy, plus optional outlier ejection) and the
  transport rewrites the request host to it;
* [`resilience`](../../spring/resilience) — an optional executor wraps the whole
  chain, so rate limiting, circuit breaking and retry protect every call.
  Because it sits *outside* the balancer, a retry re-picks a fresh endpoint and
  the breaker keys on the logical service name.

## Quick Start

### 1. Declare the interface and generate the client

Describe the remote call in an IDL (see [example/idl/greet.idl](example/idl/greet.idl))
and generate the Go client with `gs-http-gen --client`. The generated package
([example/proto](example/proto)) exposes a `Client` struct with a `Target` and
an `HTTPClient` field.

### 2. Import the starter and configure client instances

```go
import _ "go-spring.org/starter-http-client"
```

Each entry under `spring.http-client.<name>` becomes a named `*http.Client`.
Switching a call between a direct address and a discovered service is a
config-only change — the call site never changes. See
[example/conf/app.properties](example/conf/app.properties):

```properties
# Direct address — pinned to one host, no discovery.
spring.http-client.direct.addr=127.0.0.1:9471

# Service discovery + load balancing — routed by logical name.
spring.http-client.discovered.service-name=greet-svc
spring.http-client.discovered.discovery=static
spring.http-client.discovered.balancer=round_robin

# Resilience — breaker trips after 2 consecutive failures.
spring.http-client.guarded.addr=127.0.0.1:9473
spring.http-client.guarded.resilience.enabled=true
spring.http-client.guarded.resilience.error-threshold=2
spring.http-client.guarded.resilience.open-duration=30s
```

### 3. Inject the client and call

The starter registers one `*http.Client` per key, so inject it by that name and
set it on the generated client. See [example/example.go](example/example.go):

```go
type Service struct {
    Discovered *http.Client `autowire:"discovered"`
}

client := &proto.Client{Target: "greet-svc", HTTPClient: s.Discovered}
_, resp, err := client.Greet(ctx, &proto.GreetReq{Name: "Grace"})
```

## Core Features

The [example.go](example/example.go) program starts three in-process backends
and asserts all four outcomes end to end:

* **Direct address** — the `direct` client is pinned to one backend.
* **Service discovery + load balancing** — the `discovered` client routes by
  service name and round-robins across instances (observed via the `servedBy`
  field flipping between them).
* **Resilience** — the `guarded` client points at a backend that always fails;
  after the error threshold the breaker opens and calls fast-fail with
  `resilience.ErrCircuitOpen` instead of hitting the network.
* **Trace propagation** — a client span injects a W3C `traceparent` header; the
  backend echoes it back, so the same `trace_id` is observable on both ends.

## Configuration

| Key | Default | Description |
| --- | --- | --- |
| `spring.http-client.<name>.addr` | — | Direct `host:port`. Mutually exclusive with `service-name`. |
| `spring.http-client.<name>.service-name` | — | Logical name resolved through discovery. Mutually exclusive with `addr`. |
| `spring.http-client.<name>.discovery` | — | Registered discovery backend name. Required when `service-name` is set. |
| `spring.http-client.<name>.balancer` | `round_robin` | Strategy: `round_robin`, `least_conn`, `consistent_hash`, `weighted`, `zone_aware`. |
| `spring.http-client.<name>.eject-threshold` | `0` | Consecutive failures that eject an endpoint (0 disables). |
| `spring.http-client.<name>.eject-for` | `0` | How long an ejected endpoint stays out. |
| `spring.http-client.<name>.timeout` | `0` | Per-request timeout (0 = none). |
| `spring.http-client.<name>.resilience.enabled` | `false` | Wrap the transport with resilience. |
| `spring.http-client.<name>.resilience.driver` | `default` | Registered resilience backend (`default`, or `sentinel` via `starter-resilience`). |
| `spring.http-client.<name>.resilience.rate-limit` | `0` | Sustained requests/sec (0 disables). |
| `spring.http-client.<name>.resilience.error-threshold` | `0` | Consecutive failures that trip the breaker (0 disables). |
| `spring.http-client.<name>.resilience.open-duration` | `0` | How long the breaker stays open before a trial. |
| `spring.http-client.<name>.resilience.max-retries` | `0` | Extra attempts after the first failure. |
| `spring.http-client.<name>.resilience.attempt-timeout` | `0` | Per-attempt timeout. |

The starter fails fast at wiring time: exactly one of `addr` / `service-name`
must be set, and `discovery` is mandatory when routing by service name.

## Observability

The base transport is [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)-instrumented,
so every outbound request emits a client span and injects a W3C `traceparent`
header through the OpenTelemetry globals that [`starter-otel`](../starter-otel)
installs. Without `starter-otel` the globals are no-ops — no spans, no header
changes. This is the same zero-config opt-in the other client starters use.

## Switching Resilience Backends

`resilience.driver=default` uses the bundled, zero-dependency implementation.
Switching to Sentinel is `driver=sentinel` plus a blank import of
[`starter-resilience`](../starter-resilience), with no code change.

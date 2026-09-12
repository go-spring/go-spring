# starter-http-client

[English](README.md) | [中文](README_CN.md)

`starter-http-client` is the runtime half of Go-Spring's **declarative HTTP
client**. Declare a remote service as an interface in an IDL, generate the call
sites with [`gs-http-gen`](../../../gs/gs-http-gen), and inject an assembled
`*http.Client` into the generated client. Service discovery, load balancing,
resilience and trace propagation are wired for you.

Go has no runtime proxy, so call sites are produced at build time by code
generation rather than reflection — same declarative experience, no runtime
magic. The generated code only imports stdlib (`net/http` + `httpclt`); this
starter supplies the transport it runs on.

## Installation

```bash
go get go-spring.org/starter-http-client
```

## How It Works

```
 generated Client (gs-http-gen)      starter-http-client
 ┌───────────────────────────┐       ┌──────────────────────────────────────┐
 │ Greet(ctx, req)           │       │ httpclt.DoRequest (process-global) =  │
 │   Target ─────────────────┼──────▶│   resilience → discovery+LB → trace  │
 └───────────────────────────┘       └──────────────────────────────────────┘
```

The generated `Client` holds only a `Target`. The starter assembles one
`http.RoundTripper` per configured target and installs them by replacing
`httpclt.DoRequest` (the single send seam of `stdlib/httpclt`) with a hook that
dispatches on the call's `Target`, so generated clients and imperative helpers
pick it up with zero wiring. The chain is
by [`starter-http-client/httpx`](../../../starter-http-client/httpx) from three composable stdlib
abstractions, all behind the single `http.RoundTripper` seam:

* [`discovery`](../../../cloud/discovery) — when a `service-name` is set, a
  `Resolver` keeps a fresh endpoint snapshot;
* [`loadbalance`](../../../cloud/loadbalance) — a `Pool` picks one live endpoint
  per request (any registered strategy, plus optional outlier suspension) and the
  transport rewrites the request host to it;
* [`resilience`](../../../cloud/governance/resilience) — an optional executor wraps the whole
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

Each entry under `spring.http-client.instances.<name>` contributes a route to that
process-wide transport. Switching a call between a direct address and a
discovered service is a config-only change — the call site never changes. See
[example/conf/app.properties](example/conf/app.properties):

```properties
# Direct address — pinned to one host, no discovery.
spring.http-client.instances.direct.addr=127.0.0.1:9471

# Service discovery + load balancing — routed by logical name. The LB strategy
# and endpoint suspension are governance rules too (see below), not keys here.
spring.http-client.instances.discovered.service-name=greet-svc
spring.http-client.instances.discovered.discovery=static

# Resilience and endpoint selection are NOT configured here: policy lives under
# govern.* in the governance rules document (starter-governance;
# conf/govern.properties referenced by govern.source.file.path). Breaker trips
# after 2 consecutive failures:
#   govern.enabled=true
#   govern.default.enabled=true
#   govern.default.error-threshold=2
#   govern.default.open-duration=30s
```

### 3. Call

Generated clients take the target directly — nothing to inject; the installed
transport dispatches by target. See [example/example.go](example/example.go):

```go
client := &proto.Client{Target: "greet-svc"}
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
| `spring.http-client.instances.<name>.addr` | — | Direct `host:port`. May be combined with `service-name`, which then stays a pure governance label. |
| `spring.http-client.instances.<name>.service-name` | — | Logical name resolved through discovery; whenever set it is also the governance resource label. |
| `spring.http-client.instances.<name>.discovery` | — | Registered discovery backend name. Required when `service-name` is set. |
| `spring.http-client.default.driver` | — | Family-wide Driver bean for every instance that names none of its own. |
| `spring.http-client.instances.<name>.driver` | — | Overrides the family default for this instance. Embed `DefaultDriver` to add auth/metrics around the assembled transport, or replace it entirely with your own `httpx.NewTransport` call. Empty = inject the single Driver bean by type; naming a missing bean fails startup. |
| `spring.http-client.instances.<name>.tls.enabled` | `false` | Turns the entry's TLS surface on (see the `tls.*` block: cert-file/key-file/ca-file/server-name/insecure-skip-verify). |

Resilience, fault injection and endpoint selection have **no keys here**: they
are per-resource policy and live in the governance rules document (see
starter-governance). The governance resource label is `http:<service-name>`
whenever `service-name` is set (either addressing mode), `http:<addr>` only when
no service-name exists. Per-request timeout comes from
`govern.default.attempt-timeout`; the load-balancing strategy and outlier
suspension come from `balancer` / `outlier-threshold` / `outlier-suspend-for` on
the rule matching that label.

The starter fails fast at wiring time: at least one of `addr` / `service-name`
must be set, and `discovery` is mandatory when routing by service name alone
(no `addr`). Error-rate breakers resolved for `http:*` resources get a starter
floor of `min-requests=5` (a higher explicit value in the govern rule wins) — both applied by `starter-http-client/httpx`.

## Observability

Observability is built into [`starter-http-client/httpx`](../../../starter-http-client/httpx): the base
transport is [`otelhttp`](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)-wrapped,
and an active resilience executor is wrapped with fault + observe (outcome-classified
metrics). Every outbound request
emits a client span and injects a W3C `traceparent` header through the
OpenTelemetry globals that [`starter-otel`](../../starter-otel)
installs. Without `starter-otel` the globals are no-ops. This is the same
zero-config opt-in the other client starters use.

## Switching Resilience Backends

`resilience.driver=default` uses the bundled, zero-dependency implementation.
Switching to Sentinel is `driver=sentinel` plus a blank import of
[`starter-resilience`](../starter-resilience), with no code change.

## Imperative Calls

Generated clients are the form this starter targets; one-off calls go through
`stdlib/httpclt` directly (`httpclt.JSONResponse[T]` + an `httpclt.Metadata`
with `Target` set) — they dispatch through the same `httpclt.DoRequest` seam,
so they ride the exact same transport (discovery, load balancing, resilience,
tracing) with the same route key (addr or service name). Note the runtime
decodes with `jsonflow`, which matches field names strictly — plain struct
tags, no case folding.


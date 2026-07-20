# starter-gateway

[English](README.md) | [中文](README_CN.md)

`starter-gateway` is a standalone API gateway that runs on its own listen port
(`:9440` by default) alongside your application. It brings the Spring Cloud
Gateway **Route / Predicate / Filter** model to Go-Spring, but expressed in
idiomatic Go rather than a runtime DSL: a predicate is a
`func(*http.Request) bool`, a filter is a `func(next http.Handler) http.Handler`,
and routing is plain function composition.

Routes are declared entirely in config under `spring.gateway.routes.<id>` and
are **hot-reloadable** — any standard config refresh (a `starter-config-file`
volume watch, `starter-config-nacos`, ...) rebuilds the compiled route table with
no gateway-specific machinery. A route that fails to compile leaves the previous
table in place, so a bad edit never takes the gateway down.

Upstreams are either **direct** (`http(s)://host:port`) or **discovery-backed**
(`lb://<service>`), the latter reusing `spring/discovery` + `spring/loadbalance`.
Forwarding runs through `spring/resilience` for retry / circuit-breaking / rate
limiting, and the gateway contributes `/gateway/metrics` to the actuator
management port.

## Installation

```bash
go get go-spring.org/starter-gateway
```

## Quick Start

### 1. Import the `starter-gateway` Package

Routes are pure config, so no application bean is required — a blank import is
enough. Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-gateway"
```

### 2. Declare Routes in Config

Add route configuration in your project's [configuration file](example/conf/app.properties).
Each route has an id, a set of predicates, an optional filter chain, and an
upstream target:

```properties
spring.gateway.server.addr=:9440

spring.gateway.routes.api.predicates.path=/api/**
spring.gateway.routes.api.filters=stripPrefix(1),addRequestHeader(X-From,gw)
spring.gateway.routes.api.upstream.target=http://127.0.0.1:19000
```

### 3. Run

The gateway starts as a `gs.Server` on its listen port. A request to
`http://127.0.0.1:9440/api/echo` is matched by the `api` route, has its `/api`
prefix stripped, gets the `X-From: gw` header injected, and is forwarded to the
upstream. An unmatched path returns `404` from the gateway itself.

## Predicates

All predicates on a route are combined with logical **AND**; a route with no
predicates is a catch-all. Declared under `spring.gateway.routes.<id>.predicates.*`:

| Key | Meaning | Example |
| --- | --- | --- |
| `path` | ant-style path pattern (`*` one segment, `**` any) | `/api/orders/**` |
| `methods` | comma list of HTTP methods | `GET,POST` |
| `host` | exact host or `*.suffix` wildcard | `*.example.com` |
| `headers` | `K:V;K2:V2`, all required | `X-Env:prod` |
| `queries` | `k=v;k2=v2`, all required | `version=2` |
| `after` | RFC3339 time; match only after it | `2026-01-01T00:00:00Z` |

## Filters

Filters wrap the proxy handler outermost-first in declaration order, listed under
`spring.gateway.routes.<id>.filters` as `name(args...)` entries:

| Filter | Effect |
| --- | --- |
| `stripPrefix(n)` | drop the first `n` path segments |
| `prefixPath(p)` | prepend a fixed path prefix |
| `addRequestHeader(k,v)` / `setRequestHeader(k,v)` / `removeRequestHeader(k)` | mutate request headers |
| `addResponseHeader(k,v)` / `setResponseHeader(k,v)` / `removeResponseHeader(k)` | mutate response headers |
| `rewriteHost(h)` | override the outbound Host header |
| `preserveHostHeader` | keep the incoming Host instead of the upstream's |
| `requestId([header])` | ensure an `X-Request-Id` (or a named header) is present |
| `rateLimit(rate=..,burst=..,...)` | throttle via the resilience RateLimiter |
| `jwt-auth(<bean>)` / `lua(<bean>)` | delegate to a bean-backed `FilterWrapper` (see below) |

Register your own filter with `StarterGateway.RegisterFilter(name, factory)`.

## Upstreams and Load Balancing

* **Direct**: `upstream.target=http://host:port` forwards straight to that address.
* **Discovery**: `upstream.target=lb://<service>` resolves live instances through
  `spring/discovery` and picks one with `spring/loadbalance`. Set
  `upstream.balancer` (`round_robin`, `least_conn`, `consistent_hash`, `weighted`)
  and `upstream.discovery` (backend name), or a gateway-wide default via
  `spring.gateway.discovery`.

## Resilience

Named policies under `spring.gateway.resilience.<name>` mirror `resilience.Policy`
(rate limit, burst, error threshold, open duration, max concurrent, retries,
timeout). A route references one by `resilience.policy=<name>`; routes sharing a
policy share pooled breaker/limiter state.

## Observability

* `/gateway/metrics` is contributed to the actuator management port as Prometheus
  text: per-route request counts by status class, in-flight requests, and route
  reload errors.
* A `health.Indicator` named `gateway` reports UP as long as the route table is
  loaded.

## Core Features

The [example.go](example/example.go) program demonstrates and asserts:

* **Route + predicate matching** — `/api/**` is routed while an unmatched path
  gets a clean `404` without contacting any upstream.
* **Filter chain** — the `/api` prefix is stripped and an `X-From` header is
  injected before the upstream sees the request.

## Advanced Features

* **Hot reload** — routes bind through a `gs.Dync` map; any config refresh rebuilds
  the compiled table on the next request. A route that fails to compile is
  rejected and the previous table keeps serving.
* **TLS / mTLS** — set `spring.gateway.server.tls.enabled=true` with `cert-file`/
  `key-file`; adding `ca-file` turns on mutual TLS (clients must present a cert).
* **Graceful drain** — the gateway is a `gs.Server`, so it listens early (a port
  clash fails startup), serves only after readiness, and drains in-flight requests
  on shutdown.
* **Bean-backed filters** — `jwt-auth(<bean>)` and `lua(<bean>)` resolve a bean
  exported as `gateway.FilterWrapper` (a one-method `Wrap(next) handler` seam),
  letting `starter-security-jwt` and `starter-lua-filter` plug in as filters with
  no coupling to those modules.

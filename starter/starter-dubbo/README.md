# starter-dubbo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-dubbo` wraps [dubbo.apache.org/dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3)
for Go-Spring applications. On the **server** side, register your service and the
starter builds the Dubbo server, drives its lifecycle, and handles graceful
shutdown. On the **client** side, it hands you ready-to-use `*client.Client`
beans (a default client plus any named instances) for registry-based service
discovery. Both roles share one dubbo `Instance` and the global registries
defined under `${spring.dubbo.registries}`.

## Installation

```bash
go get go-spring.org/starter-dubbo
```

## Quick Start

### 1. Import the `starter-dubbo` package

Refer to the [example.go](example/example.go) file.

```go
import StarterDubbo "go-spring.org/starter-dubbo"
```

### 2. Configure the Dubbo server

Add Dubbo configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.dubbo.application.name=greet-example
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379
spring.dubbo.server.protocols.tri.port=20000
```

`spring.dubbo.application.name` is **required**: server and client share a single
dubbo `Instance` that uses the name as the metrics/registry identity, so the
starter fails fast if it is missing. Other application fields are optional:
`organization`, `module`, `version`, `owner`, `environment`.

`spring.dubbo.registries` is also **required** and is the single source of truth
for registries — defined once at the top level, never inline under server/client.
The starter only creates Dubbo components when at least one registry is defined,
and every entry must carry an `address` (both are validated up front). Roles pick
which registries to use by ID via `registry-ids` (empty means all). Registries are
map-driven — the map key is the logical registry ID — so several can coexist:

```properties
# define registries once, at the top level (etcdv3/nacos/zookeeper/polaris/...)
spring.dubbo.registries.etcd.protocol=etcdv3
spring.dubbo.registries.etcd.address=127.0.0.1:2379
spring.dubbo.registries.nacos.protocol=nacos
spring.dubbo.registries.nacos.address=127.0.0.1:8848

# a server (or client) selects registries by ID; empty means all
spring.dubbo.server.registry-ids=etcd

# multiple protocols on one server
spring.dubbo.server.protocols.tri.port=20000
spring.dubbo.server.protocols.dubbo.port=20001
```

All settings under `${spring.dubbo.server}` are optional; empty/zero values are
skipped so dubbo-go keeps its own defaults.

Provider-wide knobs:

```properties
spring.dubbo.server.group=g1
spring.dubbo.server.version=1.0.0
spring.dubbo.server.cluster=failover        # failover|failfast|failsafe|failback|forking|available|broadcast|zoneAware
spring.dubbo.server.load-balance=random     # random|roundrobin|leastactive|consistenthashing|p2c
spring.dubbo.server.serialization=hessian2  # hessian2|protobuf|msgpack|json
spring.dubbo.server.retries=2
spring.dubbo.server.filter=echo,tps
spring.dubbo.server.token=xxx
spring.dubbo.server.auth=true
spring.dubbo.server.tag=gray
spring.dubbo.server.access-log=true
spring.dubbo.server.warmup=10m
spring.dubbo.server.not-register=false
spring.dubbo.server.adaptive-service=false
```

Per-protocol (`protocols.<name>`): `port`, `ip`, `params.<k>`.
Per-registry (`registries.<name>`): `address`, `namespace`, `group`,
`username`, `password`, `timeout` (e.g. `5s`), `ttl` (e.g. `15m`), `weight`,
`zone`, `simplified`, `preferred`, `params.<k>`.

### 3. Register your service

Refer to the [example.go](example/example.go) file. A `ServiceRegister` is a
function that registers a service onto the Dubbo `server.Server`; it returns an
error because Dubbo's generated `Register*Handler` functions do.

```go
gs.Provide(func() StarterDubbo.ServiceRegister {
    return func(svr *server.Server) error {
        return greet.RegisterGreetServiceHandler(svr, &GreetProvider{})
    }
})
```

## Client

The starter also exposes Dubbo clients as beans, gated on the same `*Instance`
(a project without registries gets none). Client config lives under
`${spring.dubbo.client}`; registries and observability are inherited from the
shared `Instance`, so a client only carries protocol/timeout/registry-ids.

A **default client** bean (name `__default__`) is always available once an
`Instance` exists:

```properties
spring.dubbo.client.protocol=tri        # dubbo(default)|tri|triple|jsonrpc
spring.dubbo.client.timeout=3s          # per-request timeout, e.g. "3s"
spring.dubbo.client.registry-ids=etcd   # select global registries by ID; empty means all
```

Inject it with the `__default__` bean name, then build your generated stub:

```go
type Consumer struct {
    Client *client.Client `autowire:"__default__"`
}
// svc, _ := greet.NewGreetService(c.Client)
```

For **multiple clients** (different protocols or registry targets), declare
named instances under `${spring.dubbo.client.instances.<name>}`; each becomes a
bean named after its map key:

```properties
spring.dubbo.client.instances.orders.protocol=tri
spring.dubbo.client.instances.orders.registry-ids=etcd
spring.dubbo.client.instances.legacy.protocol=dubbo
```

```go
type Caller struct {
    Orders *client.Client `autowire:"orders"`
    Legacy *client.Client `autowire:"legacy"`
}
```

## Filters

dubbo-go ships a set of built-in filters and enables a default chain on each
side, so you get sane behavior with no configuration. The default **provider**
chain is `echo,token,accesslog,tps,generic_service,execute,pshutdown`; the
default **consumer** chain is `cshutdown`. Common filters:

| filter | side | purpose | in default chain |
|---|---|---|---|
| `echo` | provider | echo/health probe | yes |
| `token` | provider | token authentication | yes |
| `accesslog` | provider | access logging | yes |
| `tps` | provider | TPS rate limiting | yes |
| `execute` | provider | concurrency limiting | yes |
| `generic_service` | provider | generic invocation | yes |
| `pshutdown` / `cshutdown` | provider / consumer | graceful shutdown | yes |
| `auth` / `sign` | provider / consumer | request signing | no |
| `active` | consumer | client-side active/concurrency count | no |
| `metrics` / `tracing` | both | metrics / tracing | no (built in via observability) |
| `hystrix_provider` / `hystrix_consumer` | both | Hystrix circuit breaking | no |
| `sentinel-provider` / `sentinel-consumer` | both | Sentinel flow control | no |
| `seata` | both | distributed transaction | no |
| `padasvc` | provider | adaptive concurrency | no |

`filter` is a **whole-chain override** (comma-separated). To tweak the default
chain instead of replacing it, use dubbo-go's `-name` prefix to drop one entry,
e.g. `spring.dubbo.server.filter=-tps` disables the default TPS limiter.

Filters that need tuning read **service-level** params (they apply to every
service this server exposes). All are optional; empty/negative values keep
dubbo-go's default (tps/execute default to `-1`, i.e. unlimited):

```properties
# TPS rate limiting (the "tps" filter is in the default chain)
spring.dubbo.server.tps-limit-rate=100                 # requests per interval
spring.dubbo.server.tps-limit-strategy=slidingWindow   # fixedWindow|slidingWindow|threadSafeFixedWindow
spring.dubbo.server.tps-limiter=                        # custom limiter impl; empty uses default
spring.dubbo.server.tps-limit-rejected-handler=        # handler when limit exceeded

# Concurrency limiting (the "execute" filter is in the default chain)
spring.dubbo.server.execute-limit=200                  # max concurrent executions
spring.dubbo.server.execute-limit-rejected-handler=

# Request signing — pair with the "auth" filter
spring.dubbo.server.filter=echo,token,accesslog,tps,generic_service,execute,pshutdown,auth
spring.dubbo.server.param-sign=true

# Escape hatch: any other provider-level filter param, passed straight through
spring.dubbo.server.params.some-filter-key=some-value
```

On the **client** side, `filter` and `params` mirror the server:

```properties
spring.dubbo.client.filter=cshutdown,active
spring.dubbo.client.params.some-filter-key=some-value
# also available per named instance, e.g. instances.orders.filter=...
```

### Filters that cannot be configured here

Some filters exist but are **not** driven by this starter's config, because in
dubbo-go's current (Instance-based) API they read their settings elsewhere. Add
them to the chain if you want them, but configure them through their own means:

- **`seata`** — zero-config; it only propagates the `SEATA_XID` attachment.
  Just add `seata` to the chain, there is nothing to tune.
- **`sentinel-provider` / `sentinel-consumer`** — adding the filter calls
  Sentinel's `InitDefault()`, but flow/circuit-breaking **rules** come from
  Sentinel-go itself (its config file via env var, or `flow.LoadRules` in your
  code), not from dubbo config. This starter cannot feed them.
- **`hystrix_provider` / `hystrix_consumer`** — ⚠️ these read their command
  config from dubbo-go's **legacy global config singleton**, which the
  Instance-based API (used by this starter) never populates. So Hystrix config
  is silently ignored here and the filter is effectively a no-op — don't rely on
  it. (Upstream `github.com/afex/hystrix-go` is also archived.) Use `tps` /
  `execute`, or Sentinel, for limiting/breaking instead.

## Observability (built in)

Metrics and tracing are on by default, so every server and client gets
observability with zero configuration:

```properties
# Metrics — Prometheus, on by default, exposed on http://127.0.0.1:9090/metrics
spring.dubbo.metrics.enable=true
spring.dubbo.metrics.port=9090
spring.dubbo.metrics.path=/metrics

# Tracing — OTel, on by default with the stdout exporter
spring.dubbo.tracing.enable=true
spring.dubbo.tracing.exporter=stdout        # stdout|jaeger|zipkin|otlp-http|otlp-grpc
spring.dubbo.tracing.endpoint=              # required when exporter != stdout
spring.dubbo.tracing.propagator=w3c         # w3c|b3
spring.dubbo.tracing.mode=                  # always|never|ratio (empty keeps dubbo-go default)
spring.dubbo.tracing.ratio=1.0
```

Disable either with `enable=false`. When `exporter` is not `stdout`, an
`endpoint` is required (otherwise the starter fails fast). Example, ship traces
to an OTLP collector:

```properties
spring.dubbo.tracing.exporter=otlp-grpc
spring.dubbo.tracing.endpoint=127.0.0.1:4317
```

## Logging (built in)

Importing this starter puts dubbo-go under go-spring's management: its internal
logs are bridged into go-spring's `log` module automatically (installed in an
`init()`, no configuration needed). Dubbo-go has two layered logger facades —
`dubbo-go/v3/logger` (the high-level stack) and `dubbogo/gost/log/logger` (getty
and low-level modules) — and the bridge installs under both, so every framework
log line flows through the same pipeline as your own go-spring logs instead of
dubbo-go's default stdout sink.

The bridge only redirects *who writes the log*; you must still configure a
go-spring log sink, otherwise the forwarded lines land on go-spring's default
console rather than your app's output. Configure a root logger as usual, e.g.:

```properties
logging.logger.root.type=FileLogger
logging.logger.root.level=INFO
logging.logger.root.dir=../logs
logging.logger.root.file=app.log
logging.logger.root.layout.type=JSONLayout
```

Note: dubbo-go's Logger methods carry no `context.Context`, so trace-id
propagation and precise caller (file:line) are not available on this path.

## Customization (escape hatches)

Anything the typed config does not expose can be supplied via the map-driven
`params` fields (e.g. per-protocol `params`), which are passed straight through
to dubbo-go.

## Core Features

The [example](example/example.go) demonstrates a Dubbo Triple round-trip,
asserted end-to-end by `runTest`:

1. **Unary Greet call** — the server exports `greet.GreetService` over the
   Triple protocol on the configured port. The client dials it directly via
   `client.WithClientURL`, invokes `Greet`, and receives the request name back
   as the greeting, verifying the standard request/response path.
2. **Service-agnostic server** — `SimpleDubboServer` knows nothing about
   `GreetService`. It depends only on a `ServiceRegister` bean, so the same
   server drives any Dubbo service; the concrete registration lives in the
   application layer.

## Notes

- Protocols are map-driven under `${spring.dubbo.server.protocols}` (map key =
  dubbo-go protocol name; only configured entries are enabled). Registries are
  defined once at the top level under `${spring.dubbo.registries}` and each role
  selects them by ID via `registry-ids` (empty means all). Empty option fields
  are skipped. With no protocol configured, a Triple listener on port `20000` is
  used as the default.
- The Dubbo server is enabled by default; disable it with
  `spring.dubbo.server.enabled=false`.
- Only a `ServiceRegister` bean is required to activate the server.
- `spring.dubbo.application.name` is required; metrics (Prometheus) and tracing
  (OTel/stdout) are built in and on by default — see [Observability](#observability-built-in).

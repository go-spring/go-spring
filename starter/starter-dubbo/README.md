# starter-dubbo

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-dubbo` provides a lightweight [dubbo.apache.org/dubbo-go/v3](https://pkg.go.dev/dubbo.apache.org/dubbo-go/v3)
server wrapper for Go-Spring applications: register your service, and the
starter takes care of building the Triple server, lifecycle, and graceful
shutdown.

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
spring.dubbo.server.protocols.tri.port=20000
```

Protocols and registries are map-driven — the map key is the dubbo-go name and
only configured entries are enabled, so one server can expose several protocols
and publish to several registries at once:

```properties
# multiple protocols on one server
spring.dubbo.server.protocols.tri.port=20000
spring.dubbo.server.protocols.dubbo.port=20001
# publish to a registry (etcdv3/nacos/zookeeper/polaris)
spring.dubbo.server.registries.etcdv3.address=127.0.0.1:2379
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

## Core Features

The [example](example/example.go) demonstrates a Dubbo Triple round-trip,
asserted end-to-end by `runTest`:

1. **Unary Greet call** — the server exports `greet.GreetService` over the
   Triple protocol on the configured port. The client dials it directly via
   `client.WithClientURL`, invokes `Greet`, and receives the request name back
   as the greeting, verifying the standard request/response path.
2. **Service-agnostic server** — `DubboServer` knows nothing about
   `GreetService`. It depends only on a `ServiceRegister` bean, so the same
   server drives any Dubbo service; the concrete registration lives in the
   application layer.

## Notes

- Protocols and registries are map-driven under
  `${spring.dubbo.server.protocols}` / `${spring.dubbo.server.registries}`; the
  map key is the dubbo-go name and only configured entries are enabled. Empty
  option fields are skipped. With no protocol configured, a Triple listener on
  port `20000` is used as the default.
- The Dubbo server is enabled by default; disable it with
  `spring.dubbo.server.enabled=false`.
- Only a `ServiceRegister` bean is required to activate the server.

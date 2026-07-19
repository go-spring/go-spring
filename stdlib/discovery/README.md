# discovery
[English](README.md) | [中文](README_CN.md)

`discovery` is a framework-agnostic, zero-dependency abstraction for
**client-side** service discovery. It answers "given a logical service name,
which live host:port addresses can I connect to right now?" for
infrastructure clients (Redis, MySQL, MongoDB, Kafka, ...). Provider-side
registration of RPC frameworks is deliberately out of scope; a companion
`Registrar` covers the traffic-agnostic "publish this process" case.

## Features

- `Discovery` interface with `Resolve` (cold-start snapshot) and `Watch`
  (streaming updates via `Watcher`).
- `Endpoint{Addr, Weight, Healthy, Metadata}` — the single value type
  everything else consumes.
- Package-level backend registry (`Register` / `Get` / `MustGet`), one line
  of adaptation per company naming service.
- `LiveDialer` — resolves once, watches in the background, exposes
  `DialContext` / `Dial` matching common client dialer hooks (Redis,
  go-sql-driver/mysql, pgx, ClickHouse, mssql).
- `Registrar` for VM / bare-metal deployments that publish this process to a
  registry (Nacos, Consul, ...); a matching `RegisterRegistrar` registry.
- Service-mesh switch (`SetMeshMode` / `MeshMode` / `NewClientDialer`) that
  degrades discovery and load balancing to a pass-through when a sidecar is
  present.

## Installation

```
go get go-spring.org/stdlib
```

## Usage

Adapt a company naming service in one place:

```go
import "go-spring.org/stdlib/discovery"

type myBackend struct{ /* client */ }

func (b *myBackend) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) { /* ... */ }
func (b *myBackend) Watch(ctx context.Context, name string) (discovery.Watcher, error)      { /* ... */ }

func init() { discovery.Register("default", &myBackend{}) }
```

Consume it from an infrastructure client:

```go
ld, err := discovery.NewClientDialer(ctx, "default", "orders-redis")
if err != nil { return err }
defer ld.Stop()

rdb := redis.NewClient(&redis.Options{
    Addr:            "orders-redis",   // label, ignored by the dialer
    Dialer:          ld.DialContext,
    ConnMaxLifetime: 30 * time.Second, // rotate connections onto new endpoints
})
```

Register this process to Consul/Nacos (via a starter):

```go
r, _ := discovery.MustGetRegistrar("consul")
_ = r.Register(ctx, discovery.Registration{
    ServiceName: "orders",
    Addr:        "10.0.0.5:8080",
    Metadata:    map[string]string{"zone": "us-east-1a"},
})
```

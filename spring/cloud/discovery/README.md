# discovery
[English](README.md) | [中文](README_CN.md)

`discovery` is a framework-agnostic, zero-dependency abstraction for
**client-side** service discovery. It answers one question for infrastructure
clients (Redis, MySQL, MongoDB, Kafka, ...): *"given a logical service name,
which live host:port addresses can I connect to right now?"

It owns **naming only**. Selection policy (round-robin, weighted,
consistent-hash, zone-aware) and traffic feedback (failure ejection) live one
layer up in `go-spring.org/spring/cloud/loadbalance`; the service-mesh switch
lives in `go-spring.org/spring/cloud/mesh`; trace propagation lives in
`starter-otel`. Keeping this package free of policy, mesh, and tracing is what
lets one naming adapter serve every client.

## Features

- **Read side** — `Discovery` interface with `Resolve` (one-shot snapshot) and
  `Watch` (a `<-chan WatchResult` of full snapshots; the context is the sole
  lifecycle — cancel it to stop).
- **`Endpoint{Addr, Scheme, Weight, Disabled, Healthy, Metadata}`** — the value
  type consumers draw from. `Disabled` (an operator/provider decree) is kept
  separate from `Healthy` (a probe result): a disabled instance is never picked,
  not even as a fallback.
- **`Resolver`** — the stateful counterpart of `Resolve`: seeds from one
  `Resolve`, refreshes via a background `Watch`, and picks one live endpoint
  round-robin. It is a pure decision layer — it does **not** dial; the client
  owns the socket and its connection pool.
- **Optional `Catalog`** — a separate interface (`Services`) for backends that
  can enumerate service names; backends that cannot simply don't implement it.
- **Package-level read registry** - `RegisterDiscovery` / `GetDiscovery`, with a
  descriptive not-found error listing every registered name.

## Usage

Adapt a company naming service in one place:

```go
import "go-spring.org/spring/cloud/discovery"

type myBackend struct{ /* naming client */ }

func (b *myBackend) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	/* return the current snapshot */
}
func (b *myBackend) Watch(ctx context.Context, name string) (<-chan discovery.WatchResult, error) {
	/* push a fresh full snapshot on every topology change; close when ctx is cancelled */
}

func init() { discovery.RegisterDiscovery("default", &myBackend{}) }
```

Consume it from an infrastructure client:

```go
d, err := discovery.GetDiscovery("default")
if err != nil { return err }
r, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }
defer r.Stop()

ep, err := r.Pick()              // a currently-live endpoint
if err != nil { return err }
conn, err := net.Dial("tcp", ep.Addr)   // the client owns the socket + pool
```

Publishing this process to a registry is handled by a registry starter
(`starter-registry-etcd` / `-nacos` / `-consul` / `-zookeeper`), not by this
package; see those starters and [../../starter/DESIGN.md §3](../../starter/DESIGN.md).

See [DESIGN.md](DESIGN.md) for the layering and the trade-offs.

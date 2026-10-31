# registration

[English](README.md) | [中文](README_CN.md)

`registration` is the **write side** of service discovery and is control plane:
it publishes this process into the configured naming centers and maintains that
publication across the server lifecycle. The read side (`Discovery` /
`Resolver` / the in-process endpoint directory) lives in
[capability/discovery](../../capability/discovery/); no request ever passes
through this package.

## Why control plane

- Publication **produces desired endpoint state**: this instance's address,
  weight, version and zone are written into a center so other processes can
  discover it — the same kind of action as `source.Source` acquiring desired
  state.
- It is not on the request path. A `gs.Server` drives it: register once the app
  is ready, deregister when shutdown begins (PreStop), broadcast weight changes.
- Each naming center's remote adapter lives in a `starter-discovery-*` and
  implements `Registry`, exactly as `Source` adapters live in starters.

## Contract

```go
type Registry interface {
    Register(ctx context.Context, inst Instance) error      // called once, after ready
    Deregister(ctx context.Context, inst Instance) error    // idempotent
    UpdateWeight(ctx context.Context, inst Instance, weight int) error
}
```

`Server` collects **every** backend's `Registry` from the container and drives
them in lockstep: any center failing to register fails startup (split consumer
views are worse than a failed start); deregistration is idempotent per center;
`UpdateWeight` keeps an instance discoverable while it drains. `discoveryServer`
is the one `gs.Server` bean and is activated only when
`${spring.discovery.service-name}` is set — set means "publish this process",
unset means a pure consumer.

The instance identity (`Instance`, keys under `${spring.discovery}.*`), the
naming-center blocks (`${spring.discovery.<backend>.<name>}.*`) and the
consumer-side citation rules are documented in the
[discovery README](../../capability/discovery/README.md) under "Configuration".

## Observability

This package owns its own `Observer` (registration spans/duration/counter plus
the `discovery.instance.registered` gauge), independent of the read-side sync
observation: one observer per side, neither package imports the other, so
`capability/discovery` never reverses the dependency into the control plane.

# mesh

[English](README.md) | [中文](README_CN.md)

`mesh` answers one question at startup: **is this process running behind a
service-mesh sidecar?** When it is, the sidecar already does discovery and load
balancing, so the application's own client-side discovery and load balancing
step aside instead of balancing traffic a second time.

Starters read `mesh.Enabled()` and, when it is on, dial the service's stable
DNS address instead of building a discovery Resolver or a load-balance Pool.
You normally don't call this package yourself — you set one environment
variable and the starters react.

## Install

```
go get go-spring.org/cloud
```

## Usage: you configure, starters react

Mesh mode is a fixed trait of a deployment, so it is carried by the
environment, not runtime config or code. Set `GS_MESH`:

| Value          | Behavior                                             |
|----------------|------------------------------------------------------|
| `on`           | Forced on — sidecar owns discovery + load balancing  |
| `off`          | Forced off — client-side discovery/LB stays active   |
| `auto` / unset | On iff a sidecar is detected (default)               |

```bash
# In Kubernetes with Istio injected — no config needed: GS_MESH is unset,
# auto-detect sees ISTIO_META_* and turns mesh mode on.

# Same app outside the mesh — also no config; nothing is detected.

# Force a mode when auto-detect doesn't fit (e.g. debugging double balancing):
export GS_MESH=off
```

With mesh mode on, a starter configured with `service-name` ignores client-side
discovery and dials the stable address (`host:port` / ClusterDNS name) directly.
With it off, everything behaves as if the package didn't exist.

## API

```go
import "go-spring.org/cloud/mesh"

mesh.Enabled() // bool — the one call starters make
mesh.Detect()  // bool — sidecar inference only (backs "auto"); rarely called directly
```

- `Enabled()` resolves `GS_MESH`: `on`/`off` force the answer (matched
  case-insensitively after trimming whitespace); any other value — including
  unset and `auto` — infers it via `Detect()`.
- `Detect()` reports whether sidecar-injected environment variables are present
  (`ISTIO_META_*` for Istio/Envoy, `LINKERD2_PROXY_*` for Linkerd). No network
  I/O; safe at startup.

## Writing a mesh-aware starter

```go
useDiscovery := c.ServiceName != "" && !mesh.Enabled()
```

Gate client-side discovery/LB construction on `!mesh.Enabled()`; in mesh mode
fall back to the configured static address. For infra clients that resolve via
`discovery`, prefer `discovery.NewResolver` — it already folds this check in
and returns a nil Resolver in mesh mode, so callers dial the configured address
directly.

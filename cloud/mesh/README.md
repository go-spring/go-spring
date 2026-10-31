# mesh

[English](README.md) | [中文](README_CN.md)

`mesh` answers one question at startup: **is this process running behind a
service-mesh sidecar?** When it is, the sidecar already does discovery and load
balancing, so the application's own client-side discovery and load balancing
step aside instead of balancing traffic a second time.

The [runtime composition root](../runtime) reads `mesh.Enabled()` once, at
startup, and exposes the answer as `Runtime.UsesDiscovery(serviceName)`. Client
starters ask that one question; when it says no they dial their configured
stable address instead of building a discovery Resolver or a load-balance Pool.
The discovery stack itself (backends, `discovery.NewResolver`, `loadbalance`)
carries no mesh reference at all — mesh is an ADDRESSING decision, not another
kind of discovery.

## Installation

```
go get go-spring.org/cloud
```

## Usage: you configure, starters react

Mesh mode is a fixed trait of a deployment, so it is carried by the
environment, not runtime config or code. Set `GS_MESH_MODE`:

| Value          | Behavior                                             |
|----------------|------------------------------------------------------|
| `on`           | Forced on — sidecar owns discovery + load balancing  |
| `off`          | Forced off — client-side discovery/LB stays active   |
| `auto` / unset | On iff a sidecar is detected (default)               |

```bash
# In Kubernetes with Istio injected — no config needed: GS_MESH_MODE is unset,
# auto-detect sees ISTIO_META_* and turns mesh mode on.

# Same app outside the mesh — also no config; nothing is detected.

# Force a mode when auto-detect doesn't fit (e.g. debugging double balancing):
export GS_MESH_MODE=off
```

With mesh mode on, an entry configured with `service-name` ignores in-process
discovery and dials its configured stable address (`host:port`, ClusterDNS name,
or a fixed endpoint) directly. The `service-name` is a harmless leftover of a
config shared with non-mesh deployments — it is ignored, not treated as an
error. With mesh mode off, everything behaves as if the package didn't exist.

## API

```go
import "go-spring.org/cloud/mesh"

mesh.Enabled() // bool — consulted by Runtime.UsesDiscovery, not by client code
mesh.Detect()  // bool — sidecar inference only (backs "auto"); rarely called directly
```

- `Enabled()` resolves `GS_MESH_MODE`: `on`/`off` force the answer (matched
  case-insensitively after trimming whitespace); any other value — including
  unset and `auto` — infers it via `Detect()`.
- `Detect()` reports whether sidecar-injected environment variables are present
  (`ISTIO_META_*` for Istio/Envoy, `LINKERD2_PROXY_*` for Linkerd). No network
  I/O; safe at startup.

## Writing a mesh-aware starter

```go
if center.UsesDiscovery(c.ServiceName) {
    // in-process discovery + load balancing
} else {
    // dial the configured stable address
}
```

That is the whole integration: one question at the entry's addressing-mode
decision. Do not import `mesh` from a client driver and do not check it inside
`discovery.NewResolver` or any inner resolver/pool helper — they only ever run
on the discovery branch. With a configured stable address (`addr`/DSN/ClusterDNS
name), mesh simply makes the entry take the direct branch.

## Design notes

- `mesh.Enabled()` is a pure, stateless function: no caching, no side effects,
  `os`/`strings` only. An empty value and an unset variable are
  indistinguishable to `os.Getenv`, so both fall through to auto.
- It is consulted in exactly ONE place, `Runtime.UsesDiscovery`, at client
  construction rather than per request. Being pure, it needs no caching and
  stores no `Runtime` field; client code never reads the environment.
- There is no `SetEnabled`, no tri-state `atomic.Value`, and no `starter-mesh`
  module. `cloud/mesh` carries no context, log, or `sync.Once` startup
  logging — observability is the consumer starter's job.
- Never on a request's hot path: `loadbalance` holds zero mesh references, and
  the decision is made in the client constructor, not in `RoundTrip`/`Pick`.
- Tests drive it with `t.Setenv("GS_MESH_MODE", ...)` before constructing the
  runtime; `Runtime.UsesDiscovery` itself is unit-tested against the field.

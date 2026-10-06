# endpoint

[English](README.md) | [中文](README_CN.md)

A component that wants an extra HTTP path on the actuator's management port
contributes an `Endpoint` bean. The actuator collects every `endpoint.Endpoint`
bean and mounts each on its mux, next to the built-in probe endpoints.

## Installation

```
go get go-spring.org/cloud
```

## Usage

Contribute a Prometheus `/metrics` endpoint:

```go
import (
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go-spring.org/gs"
    "go-spring.org/cloud/actuator/endpoint"
)

func init() {
    gs.Provide(&endpoint.Endpoint{Pattern: "/metrics", Handler: promhttp.Handler()})
}
```

Pattern must not collide with the actuator's built-in patterns (`/healthz`,
`/readyz`, `/info`, ...) or with another endpoint; a duplicate pattern panics
at startup. A contributed endpoint is served as-is: whether it exists at all
is the contributor's own enable switch, and access control is the management
port's authentication, not per-endpoint filtering.

## Design notes

- The actuator only assembles endpoints; it never adjudicates them. A
  contributed endpoint is registered unconditionally, exactly like the built-in
  `/info` — there is no per-endpoint filtering.
  `spring.actuator.endpoints.include` and `spring.actuator.endpoints.exclude`,
  together with the `endpoint.Endpoint.Sensitive` field, were all removed.
  Whether an endpoint exists is the contributor's own enable switch; access
  control is the management port's whole-port guard.
- Built-in and contributed endpoints share the same `Endpoint` type and mount by
  `Endpoint.Pattern` (ServeMux syntax, method-qualified patterns allowed). There
  is no `route` struct and no "name" concept.
- Self-introspection converges to just `/info` (health probes are separate:
  `/healthz`, `/readyz`, `/startupz`). `/beans`, `/configprops`, `/env`,
  `/loggers` and `/threaddump` were removed and are not coming back, nor is a
  tree-view or merged-config endpoint; a dynamic log-level endpoint
  (`POST /loggers/{name}`) is likewise rejected, because the log core only
  exports a read-only `Loggers()`. Built-in endpoints carry no sensitive items.
- If per-endpoint control conditions are ever needed, they belong on
  `endpoint.Endpoint`, declared by the contributor (e.g. an
  `Enabled func() bool` predicate) and asked uniformly at registration — not a
  central include/exclude list, which was rejected. None of this is implemented
  yet; do not add it ahead of need.

# endpoint
[English](README.md) | [中文](README_CN.md)

`endpoint` is a tiny, zero-dependency seam for contributing an operational
HTTP handler to a management server (typically the actuator's port). A
component exports an `Endpoint` bean; the collector autowires every such bean
and mounts it — no cross-starter import required.

## Features

- Single interface `Endpoint { Path() string; http.Handler }`.
- Mirrors `go-spring.org/spring/health.Indicator`: contributor and collector
  depend only on stdlib, never on each other.

## Installation

```
go get go-spring.org/stdlib
```

## Usage

Contribute Prometheus `/metrics` to the actuator without importing the
actuator starter:

```go
import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go-spring.org/gs"
    "go-spring.org/spring/actuator/endpoint"
)

type promEndpoint struct{ http.Handler }

func (promEndpoint) Path() string { return "/metrics" }

func init() {
    gs.Provide(func() endpoint.Endpoint {
        return promEndpoint{Handler: promhttp.Handler()}
    }).Export(gs.As[endpoint.Endpoint]())
}
```

The actuator collects every `endpoint.Endpoint` bean and mounts each on its
management mux. `Path` should be distinct from `/health`, `/readiness`,
`/info` and from any other contributed endpoint.

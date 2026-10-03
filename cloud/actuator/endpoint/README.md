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

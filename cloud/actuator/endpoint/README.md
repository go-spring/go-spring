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
    gs.Provide(&endpoint.Endpoint{Path: "/metrics", Handler: promhttp.Handler()})
}
```

Path must not collide with the actuator's built-in paths (`/healthz`,
`/readyz`, `/info`, ...) or with another contributed endpoint; a duplicate
path panics at startup. Each endpoint is also subject to the actuator's
`spring.actuator.endpoints.include` / `.exclude` filter under its path name
(the path without the leading slash).

# starter-consul

[English](README.md) | [中文](README_CN.md)

`starter-consul` provides a Consul client wrapper based on github.com/hashicorp/consul/api,
making it easy to integrate and use Consul in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-consul
```

## Quick Start

### 1. Import the `starter-consul` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-consul"
```

### 2. Configure the Consul Instance

Add Consul configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.consul.address=127.0.0.1:8500
```

### 3. Inject the Consul Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/hashicorp/consul/api"

type Service struct {
    Consul *api.Client `autowire:"__default__"`
}
```

### 4. Use the Consul Instance

Refer to the [example.go](example/example.go) file.

```go
_, err := s.Consul.KV().Put(&api.KVPair{Key: "key", Value: []byte("value")}, nil)
pair, _, err := s.Consul.KV().Get("key", nil)
```

## Core Features

The [example.go](example/example.go) runTest demonstrates three core Consul features:

1. **KV put/get** — write and read a key through `s.Consul.KV().Put(...)` / `s.Consul.KV().Get(...)`.
2. **Service registration + discovery** — register a service via `s.Consul.Agent().ServiceRegister(...)`
   and locate it through `s.Consul.Agent().Services()`.
3. **Deregister** — remove the service via `s.Consul.Agent().ServiceDeregister(...)` and confirm
   it no longer appears in `s.Consul.Agent().Services()`.

## Advanced Features

* **Supports multiple Consul instances**: You can define multiple Consul instances under
  `spring.consul.instances` in the configuration file and reference them by name.

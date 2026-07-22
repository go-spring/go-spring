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
spring.consul.a.address=127.0.0.1:8500
```

Each entry under `spring.consul` is a named client: the key (`a`, `b`, ...) becomes the
bean name you inject.

### 3. Inject the Consul Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/hashicorp/consul/api"

type Service struct {
    Consul *api.Client `autowire:"a"`
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

* **Multiple Consul instances**: Define one named client per entry under `spring.consul`
  and inject each by its key.

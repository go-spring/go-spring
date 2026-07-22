# starter-etcd

[English](README.md) | [中文](README_CN.md)

`starter-etcd` provides an etcd v3 client wrapper based on go.etcd.io/etcd/client/v3,
making it easy to integrate and use etcd in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-etcd
```

## Quick Start

### 1. Import the `starter-etcd` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-etcd"
```

### 2. Configure the etcd Instance

Add etcd configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.etcd.a.endpoints=127.0.0.1:2379
```

Each entry under `spring.etcd` is a named client: the key (`a`, `b`, ...) becomes the
bean name you inject. Multiple endpoints are comma-separated:
`spring.etcd.a.endpoints=127.0.0.1:2379,127.0.0.1:2380`.

### 3. Inject the etcd Instance

Refer to the [example.go](example/example.go) file.

```go
import clientv3 "go.etcd.io/etcd/client/v3"

type Service struct {
    Etcd *clientv3.Client `autowire:"a"`
}
```

### 4. Use the etcd Instance

Refer to the [example.go](example/example.go) file.

```go
_, err := s.Etcd.Put(ctx, "key", "value")
resp, err := s.Etcd.Get(ctx, "key")
```

## Advanced Features

* **Multiple etcd instances**: Define one named client per entry under `spring.etcd`
  and inject each by its key.

## Core Features

The [example.go](example/example.go) demonstrates three core etcd v3 features:

* **Put/Get** — write a key with `cli.Put` and read it back via `cli.Get`, verifying `Kvs[0].Value`.
* **Watch** — subscribe to key changes via `cli.Watch`, then trigger a `Put` and receive the event within a bounded timeout.
* **Lease + TTL** — `cli.Grant` a lease, attach it to a key with `clientv3.WithLease`, and check the remaining TTL via `cli.TimeToLive`.

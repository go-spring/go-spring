# starter-ants

[English](README.md) | [中文](README_CN.md)

`starter-ants` provides an in-process goroutine-pool wrapper based on
[ants](https://github.com/panjf2000/ants), making it easy to integrate and use
a high-performance, resource-bounded worker pool in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-ants
```

## Quick Start

### 1. Import the `starter-ants` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-ants"
```

### 2. Configure the ants Pool

Add ants configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.ants.main.size=256
```

### 3. Inject the ants Pool

Refer to the [example.go](example/example.go) file.

```go
import "github.com/panjf2000/ants/v2"

type Service struct {
    Pool *ants.Pool `autowire:"main"`
}
```

### 4. Use the ants Pool

Refer to the [example.go](example/example.go) file.

```go
err := s.Pool.Submit(func() {
    // do work on a pooled goroutine
})
```

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three core ants operations:

* **Submit** — dispatch tasks onto pooled goroutines and confirm they all run.
* **Instance isolation** — two named pools are fully independent, proven by their
  distinct capacities coming from configuration.
* **Nonblocking overload** — a full pool configured as nonblocking returns
  `ErrPoolOverload` from `Submit` instead of blocking.

## Advanced Features

* **Supports multiple ants pools**: You can define multiple pools in the
  configuration file and reference them by name in your project.
* **Support ants extensions**: You can extend pool creation by implementing the
  `Driver` interface.
* **Panic handler**: register `StarterAnts.SetPanicHandler(fn)` before startup to
  recover panics thrown by submitted tasks; without it ants re-panics on the
  worker goroutine. It is a global hook shared by every DefaultDriver-built pool;
  per-pool handlers require a custom `Driver`.
* **Runtime pool metrics**: read `pool.Running()`, `pool.Free()`, and
  `pool.Cap()` straight off the pool (no OTel) to monitor utilization.
* **Graceful shutdown**: the destroy callback calls `Release()`, stopping the
  background purge goroutine and reclaiming all workers.

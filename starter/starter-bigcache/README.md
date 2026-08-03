# starter-bigcache

[English](README.md) | [中文](README_CN.md)

`starter-bigcache` provides an in-process cache wrapper based on
[BigCache](https://github.com/allegro/bigcache), making it easy to integrate and
use fast, GC-friendly in-memory caching in Go-Spring applications.

## Installation

```bash
go get go-spring.org/starter-bigcache
```

## Quick Start

### 1. Import the `starter-bigcache` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-bigcache"
```

### 2. Configure the BigCache Instance

Add BigCache configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.bigcache.main.life-window=10m
```

### 3. Inject the BigCache Instance

Refer to the [example.go](example/example.go) file.

```go
import "github.com/allegro/bigcache/v3"

type Service struct {
    Cache *bigcache.BigCache `autowire:"main"`
}
```

### 4. Use the BigCache Instance

Refer to the [example.go](example/example.go) file.

```go
err := s.Cache.Set("key", []byte("value"))
value, err := s.Cache.Get("key")
```

## Core Features

The [example.go](example/example.go) program demonstrates and asserts three core BigCache operations:

* **SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **DELETE + miss** — remove a key with `Delete(...)` and confirm a subsequent `Get(...)` returns `ErrEntryNotFound`.
* **Instance isolation** — a key written to one named instance is not visible through another, proving multi-instance wiring.

## Advanced Features

* **Supports multiple BigCache instances**: You can define multiple BigCache instances in the configuration file and
  reference them by name in your project.
* **Support BigCache extensions**: You can extend BigCache creation by implementing the `Driver` interface.
* **Hit/miss statistics**: set `stats-enabled=true` and read `cache.Stats()` for hit/miss/collision counters — the
  read mechanism for cache-effectiveness monitoring.
* **Eviction/expiry callback**: register `StarterBigCache.SetOnRemove(fn)` before startup to be notified when an entry
  is evicted or expires. It is a global hook shared by every DefaultDriver-built cache; per-instance callbacks require a
  custom `Driver`.
* **Graceful shutdown**: the destroy callback calls `Close()`, stopping the background cleaner goroutine.
* **Near cache backend**: `AsCache(bc, codec)` adapts a BigCache instance to `spring/cache.Cache` for use as the near
  (in-process) level of a multi-level cache. Note BigCache expires by a single global `life-window`, so the per-call TTL
  is ignored; when used purely as a local level, `cache.Memory` (which keeps concrete types without serialization) is
  often the better fit.

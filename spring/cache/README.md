# cache
[English](README.md) | [中文](README_CN.md)

`cache` is a framework-agnostic, zero-dependency abstraction for key/value
caching. Declare a caching concern once and swap the backend (in-process,
Redis, memcached, bigcache, multi-level) without changing business code.

## Features

- `Cache` interface with `Get` / `Set` / `Delete`; values are `any`.
- Package-level driver registry (`Register` / `Get` / `MustGet`) for selecting
  a backend by name.
- `Memory` — zero-dependency, concurrency-safe in-process cache with per-entry
  expiry; stores concrete Go types (no serialization).
- `ByteStore` + `Codec` + `FromByteStore` — the single seam every byte-oriented
  remote backend (Redis, memcached, bigcache) implements; `JSONCodec` is the
  default codec.
- `MultiLevel` — near-to-far hierarchy with read-through backfill and
  fan-out writes/deletes.
- `Key` / `Namespace` — colon-joined composite key helpers.
- `AsStore` — bridge from `Cache` to `aspect.Store`, enabling the `Cache`
  interceptor to be backed by any registered cache.

## Installation

```
go get go-spring.org/stdlib
```

## Usage

```go
import (
    "context"
    "time"

    "go-spring.org/spring/cache"
)

// In-process cache.
c := cache.NewMemory()
_ = c.Set(ctx, "user:42", &User{Name: "Ada"}, 5*time.Minute)
v, ok, _ := c.Get(ctx, "user:42")
```

Combine a local Memory tier with a remote tier registered by a starter:

```go
remote := cache.MustGet("redis")               // registered by starter-go-redis
local  := cache.NewMemory()
c      := cache.NewMultiLevel(30*time.Second, local, remote)
```

Wire it into the aspect `Cache` interceptor:

```go
import (
    "go-spring.org/spring/aspect"
    "go-spring.org/spring/cache"
)

userKey := cache.Namespace("user")
chain := aspect.NewChain(
    aspect.Cache(
        cache.AsStore(ctx, c),
        func(jp *aspect.Joinpoint) string { return userKey(jp.Method) },
        5*time.Minute,
    ),
)
```

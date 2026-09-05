# cache
[English](README.md) | [中文](README_CN.md)

`cache` is a key/value caching abstraction with pluggable backends. You
declare the cache in config (`spring.cache`); a backend starter (go-redis,
redigo, bigcache, memcached) supplies the implementation.

## Quick start

With starter-go-redis:

```properties
# a redis client bean named "main"
spring.go-redis.main.addr=127.0.0.1:6379
# expose it as a cache.Cache bean named "main"
spring.cache.primary.driver=go-redis:main
```

Other drivers: `redigo:<pool>`, `bigcache:<instance>`, `memcached:<client>`.
The beanID after the colon names the backend client bean to wrap, and the
cache bean is registered under the same name, so inject it by that name. The
`spring.cache.<key>` slot is just an iteration key; the beanID is the cache's
identity.

## Use

```go
type User struct{ Name string }

// typed. val must be a pointer. The codec is fixed at construction
// (New(bc, WithCodec(...))); default is JSON.
err := c.Get(ctx, "user:42", &user)        // cache.ErrMiss if absent
_  = c.Set(ctx, "user:42", user, 5*time.Minute)

// the rare entry in a different format: the same WithCodec option,
// passed to that one call instead of New.
_ = c.Set(ctx, "icon:42", icon, 0, cache.WithCodec(gobCodec))
err = c.Get(ctx, "icon:42", &icon, cache.WithCodec(gobCodec))

// raw bytes, bypassing the codec, when the caller already holds bytes.
b, err := c.GetBytes(ctx, "icon:42")       // (nil, cache.ErrMiss) if absent
_  = c.SetBytes(ctx, "icon:42", png, 0)    // non-positive ttl = no expiry
_  = c.Delete(ctx, "user:42")              // deleting an absent key is fine
```

### Miss vs failure

A missing key returns the sentinel `ErrMiss`, not a backend error. Each
backend maps its native "key absent" (redis.Nil, memcache.ErrCacheMiss,
bigcache.ErrEntryNotFound) onto it, so a read-through works without
backend-specific error handling:

```go
err := c.Get(ctx, key, &v)
if errors.Is(err, cache.ErrMiss) {
    v, err = loadFromSource(ctx, key)      // fall through only on a real miss
    _ = c.Set(ctx, key, v, 5*time.Minute)
}
```

### TTL semantics

go-redis, redigo and memcached honor the per-entry ttl (non-positive means no
expiry). bigcache ignores it and uses the global `LifeWindow` set at
construction. A backend that cannot honor per-entry ttl ignores the argument
and says so in its own docs; it does not panic.

## Implementing a backend

Implement `ByteCache`, the three raw primitives a remote client maps 1:1 to
its native API. Typed access is not the backend's concern: `Cache` embeds a
`ByteCache`, adds the codec layer once, and promotes the raw methods
unchanged.

```go
type ByteCache interface {
    GetBytes(ctx context.Context, key string) ([]byte, error) // (nil, ErrMiss) when absent
    SetBytes(ctx context.Context, key string, val []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}
```

Implementations must be safe for concurrent use. To make the backend available
to `spring.cache`, register a driver in starter-cache with
`RegisterDriver("my-backend", ...)`. A driver is a bean-builder factory: given
the backend client's bean name it returns the module that provides the
`Cache` bean, so neither this package nor the driver imports a concrete client
type.

A custom serialization format does not need a new backend. `WithCodec` decides
the codec wherever it appears: passed to `New` it fixes the whole cache's
format (default JSON); passed to a single `Get`/`Set` it covers the rare entry
whose format differs from the rest. A mismatched codec fails on decode instead
of corrupting data silently.

## Boundaries

- No in-process `Memory`, no `MultiLevel`, no aspect bridge. Use bigcache for
  an in-process tier; other adapters belong to their callers.
- No stampede protection, async refresh, or negative caching. Those are caller
  policies, not interface features.
- The package imports no IoC concepts. The driver registry and the
  `spring.cache` module live in starter-cache; importing a backend starter is
  what activates the wiring.

## Installation

```
go get go-spring.org/cloud
```

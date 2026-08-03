# cache
[English](README.md) | [中文](README_CN.md)

`cache` is a backend-pluggable key/value caching abstraction. A caching concern
is declared once in config (`spring.cache`) and served by whichever starter
wires the backend.

## Features

- One `Cache` interface every backend implements: typed `Get`/`Set` (values
  cross the bytes/any boundary through a pluggable `Codec`, default
  `JSONCodec`) plus raw `GetBytes`/`SetBytes` for callers that already hold
  bytes.
- `ErrMiss` — a missing key is a sentinel, distinct from a backend error, so a
  caller falls through to the source of truth only on a real miss.
- A driver registry (`RegisterDriver`/`GetDriver`) mirroring the discovery and
  resilience driver idioms; empty-name, nil, or duplicate registrations panic
  at init.
- A config-driven module: under `spring.cache`, each entry's
  `driver = "<driver>:<beanID>"` selects a registered driver and the backend
  bean it wraps.

## Import

```
go get go-spring.org/spring
```

```go
import "go-spring.org/spring/data/cache"
```

## Configure

A starter creates the backend client beans **and** registers its cache driver.
With starter-go-redis:

```properties
# a redis client bean named "main"
spring.go-redis.main.addr=127.0.0.1:6379
# expose it as a cache.Cache bean named "main"
spring.cache.primary.driver=go-redis:main
```

Other drivers: `redigo:<pool>`, `bigcache:<instance>`, `memcached:<client>`.
The beanID after the colon names the backend client bean to wrap; the cache
bean is registered under that same name, so inject it by that name.

## Use

```go
type User struct{ Name string }

// typed — val must be a pointer; JSON is the default codec.
err := c.Get(ctx, "user:42", &user)        // cache.ErrMiss if absent
_  = c.Set(ctx, "user:42", user, 5*time.Minute)

// raw bytes — bypass the codec.
b, err := c.GetBytes(ctx, "icon:42")       // (nil, cache.ErrMiss) if absent
_  = c.SetBytes(ctx, "icon:42", png, 0)    // non-positive ttl = no expiry
```

`ttl` semantics differ by backend: go-redis / redigo / memcached honor per-entry
ttl (non-positive = no expiry); bigcache ignores it in favor of a global
`LifeWindow` set at construction.

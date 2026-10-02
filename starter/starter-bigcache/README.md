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

Refer to the [main.go](example/main.go) file.

```go
import _ "go-spring.org/starter-bigcache"
```

### 2. Configure the BigCache Instance

Add BigCache configuration in your project's [configuration file](example/conf/app.properties), for example:

```properties
spring.bigcache.instances.main.life-window=10m
```

### 3. Inject the BigCache Instance

Refer to the [main.go](example/main.go) file.

```go
import StarterBigCache "go-spring.org/starter-bigcache"

type Service struct {
    Cache *StarterBigCache.Cache `autowire:"main"`
}
```

The bean is the starter's `*Cache` wrapper. The raw `*bigcache.BigCache` is held in an
unexported field with no accessor, so every operation stays behind the wrapper: Get/Set/Delete
emit the operation's span and metrics themselves, and the remaining raw methods
(Stats, Len, Reset, Close, …) are re-exposed as plain delegations. Nothing rate-limits or breaks
these calls, and no access log is written: an in-process cache has no external dependency to
protect and no external call to record. See [Design Notes](README.md#design-notes).

### 4. Use the BigCache Instance

Refer to the [main.go](example/main.go) file.

```go
err := s.Cache.Set("key", []byte("value"))
value, err := s.Cache.Get("key")
```

## Core Features

The [example](example/) self-asserts the wiring and the whole command surface; run it with
`example/check.sh`. It covers, in order:

* **SET/GET** — write a value with `Set(...)` and read it back with `Get(...)`.
* **DELETE + miss** — remove a key with `Delete(...)` and confirm a subsequent `Get(...)` returns `ErrEntryNotFound`.
* **Instance isolation** — a key written to one named instance is not visible through another, proving multi-instance wiring.
* **Beyond Get/Set/Delete** — `Len`, `Iterator` and `Reset`.
* **`life-window` semantics** — two instances share a 1s window and differ only in `clean-window`; one stopped
  serving the stale entry, the other still serves it.
* **A custom Driver** — one Driver bean, selected by name, that reaches `bigcache.Config.OnRemove`.
* **Cache abstraction** — the same instance through `cloud/cache`, including the ignored per-call TTL.
* **HTTP handlers** — driven and asserted, not merely documented.

See [example/README.md](example/README.md) for the list, and
[example-otel/](example-otel/) for the observability side (gauges, per-operation metrics and spans).

## Advanced Features

* **Supports multiple BigCache instances**: You can define multiple BigCache instances in the configuration file and
  reference them by name in your project.
* **Support BigCache extensions**: You can extend BigCache creation by implementing the `Driver` interface.
  When several Driver beans coexist, an instance selects one by name: `spring.bigcache.instances.<name>.driver = <bean-name>`
  (empty = fall back to the family-wide `spring.<family>.default.driver`, then to the single Driver bean by type; naming a missing bean fails startup).
* **Observability**: Get/Set/Delete emit a `get`/`set`/`delete` span and the `bigcache.operation.total`
  counter and `bigcache.operation.duration` histogram, labelled `operation` × `status` × `cache.name`.
  The key rides the span as `bigcache.key`, never a metric label.
* **Hit/miss statistics**: on by default (`stats-enabled`) — read `cache.Stats()` for the
  hit/miss/collision counters, or scrape the OTel observable gauges the starter exports. Set
  `stats-enabled=false` to drop the per-key bookkeeping bigcache keeps while it is on.
* **Eviction/expiry callback**: register `bigcache.Config.OnRemove` by providing a
  custom `Driver` (implement `CreateClient` and set the callback on the
  `bigcache.Config` you build).
* **Graceful shutdown**: the destroy callback calls `Close()`, stopping the background cleaner goroutine.
* **Cache abstraction backend**: alongside the wrapper, each instance is also provided as a
  `cloud/cache.Cache` bean named `bigcache:<instance>` — inject `*cache.Cache` with the
  autowire tag `bigcache:<instance>` to use it through the cache abstraction. The bean is
  lazy: un-injected, it never instantiates, so there is no config switch. Note BigCache
  expires by a single global
  `life-window`, so the per-call TTL is ignored; when used purely as a local
  level, `cache.Memory` (which keeps concrete types without serialization) is
  often the better fit.

## Design Notes

* **Process-local, not a cache cluster.** Entries live in this process's heap: two
  replicas hold independent copies, and nothing is invalidated across them. That is the
  trade-off for zero-hop reads — if you need coherence, use a networked backend.
* **Sizing is fixed at startup.** `shards` must be a power of two, and
  `max-entries-in-window` × `max-entry-size` roughly bounds the memory the instance
  pre-allocates. There is no runtime resize, so pick the instance's shape up front.
* **Entries are `[]byte`.** The cache treats values as opaque bytes; encode and decode in
  your own code. The `cloud/cache` path serializes with JSON.
* **`life-window` is one TTL for the whole instance**, not per entry: every entry in an
  instance shares it. Several TTL classes means several named instances — that is what
  the multi-instance bucket is for.
* **`life-window` is not a read-side TTL.** It marks an entry stale; it does not hide it. `Get`
  returns an entry whatever its age, and removal is the cleaner's job, every `clean-window`. With
  `clean-window=0` a value outlives its `life-window` until capacity eviction takes it — a
  practical detail the startup-time sizing rules assume you know.
* **This starter is the store; policy lives one layer up.** Loader/refresh semantics
  belong to `cloud/cache`, not to a backend starter: this module contributes the cache,
  the abstraction contributes the reload behaviour.
* **Self-observed, never governed.** This starter emits its own span and metrics and
  takes no part in the framework's executor chain: no governance bean, no
  `ClientParams`, nothing that a rule could arm. A rate limit or a breaker would be
  wrong here — the dependency is in this process, so "the downstream is down" cannot
  happen, while a breaker tripped by a transient `ErrEntryTooLarge` would reject
  `Get`s that would have hit. It writes no access log either: a cache call is
  high-frequency and reaches nothing outside the process. The one thing it shares with
  the rest of the framework is the span-attribute carrier — its span picks up whatever
  a layer above contributed, with no cooperation from either side.

# cache Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`cache` is the caching abstraction of the data layer: the single `Cache`
interface every backend implements, plus a driver registry and a config-driven
module that wires a named backend into a bean.

## 1. Responsibilities & Boundaries

- **Does:** define the `Cache` interface; provide the `Codec`/`JSONCodec`
  serialization seam and the `ResolveCodec` default; run a `spring.cache`
  module that reads config and dispatches to registered drivers; expose the
  `RegisterDriver`/`GetDriver` registry.
- **Refuses:**
  - No concrete backend. Redis / redigo / bigcache / memcached adapters live
    in their starters; each registers a driver and wraps its own client bean.
  - No in-process `Memory`, no `MultiLevel`, no aspect bridge here. (An older
    experimental revision had them; they were dropped when the interface was
    simplified to bytes-native backends — see §4.)
  - No stampede protection, async refresh, or negative caching — policies for
    the caller, not a general interface.

## 2. Key Abstractions / Seams

- **`Cache` is bytes-native with a codec on top.** `GetBytes`/`SetBytes` are
  the raw primitive a remote backend (Redis, memcached, bigcache) maps 1:1 to
  its client; `Get`/`Set` wrap a `Codec` (default `JSONCodec`) around them for
  typed callers. `ResolveCodec` centralizes the default so every backend
  applies it identically instead of re-deriving it.
- **`ErrMiss` separates miss from failure.** A backend maps its native
  "key absent" (redis.Nil, memcache.ErrCacheMiss, bigcache.ErrEntryNotFound)
  to `ErrMiss`; any other error is a real failure. Callers fall through to the
  source of truth only on `ErrMiss`.
- **A Driver is a bean-builder factory.** `Driver func(beanID string)
  gs.ModuleFunc` takes the backend client's bean name and returns the module
  that provides the `Cache` bean — so this package never imports a concrete
  client type.
- **Config dispatch.** The package's own `init` module reads `spring.cache`,
  parses each `driver = "<driver>:<beanID>"`, looks up the driver, and invokes
  `driver(beanID)`. The beanID both selects the backend client and names the
  resulting cache bean.

## 3. Constraints

- The registry is safe to populate at init and read concurrently at runtime;
  one `sync.RWMutex` guards it.
- `Cache` implementations must be safe for concurrent use.
- A backend that cannot honor per-entry TTL must ignore the `ttl` argument
  silently and document it (bigcache uses a global `LifeWindow`) — never panic.
- `driver` parsing requires a non-empty driver name and beanID; anything else
  is a config error returned from the module, not a panic.

## 4. Trade-offs / Alternatives Rejected

- **Bytes + codec on one interface, not a separate `ByteStore`.** The earlier
  design had a narrow `ByteStore` lifted to `Cache` via `FromByteStore`. Every
  backend shipped is bytes-native, so collapsing both onto `Cache` removes a
  type and an adapter with no loss of generality. The cost — a future
  live-value in-process backend would have to serialize — is accepted; use
  bigcache for an in-process tier.
- **Per-call codec, not per-instance.** `Get`/`Set` take an optional codec so
  one cache can serve mixed types; the default is JSON and a mismatched codec
  fails loudly on decode rather than corrupting silently. A per-instance codec
  would need its own registry, rejected as overkill for the 1% case.
- **Cache bean named by backend beanID.** The config slot (`spring.cache.X`)
  is only an iteration key; the bean name is the beanID suffix, coupling cache
  identity to the backend client. Intentional: one client → one cache name,
  and duplicate wiring panics at init rather than silently overwriting.
- **No aspect bridge in this package.** The old `AsStore` coupled this package
  to `aspect` and its `Store` contract. Dropping it keeps the package focused;
  an aspect adapter, if needed, belongs with `aspect` or the caller.

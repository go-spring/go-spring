# starter-bigcache Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-bigcache` is a Client-archetype starter (`starter/DESIGN.md` §2.2)
that provisions in-process caches backed by `github.com/allegro/bigcache`.
Because bigcache is a pure Go, GC-friendly heap cache, there is no network
lifecycle — but there is still a background eviction goroutine that must
be released on shutdown.

## 1. Responsibilities & Boundaries

- Binds each `spring.bigcache.instances.<name>` entry to a
  `*bigcache.BigCache` bean via `gs.Group`. No single-instance default
  (see `project_client_starter_multiinstance`).
- Registers a `stdlib/cache` driver named `bigcache` so callers using
  the `cache.Cache` abstraction — including `stdlib/cache`'s
  MultiLevel — can pick this backend by name without importing bigcache
  directly.
- No cross-process coherence: the cache lives in the process's own
  heap. Two replicas hold independent copies; that is the trade-off for
  zero-hop reads.

## 2. Key Abstractions & Seams

- **`gs.Group` per instance.** Distinct configs (LRU/expiring, small
  fast-path for hot data plus larger slow-path) coexist as separate
  bigcache instances tuned independently.
- **`destroy = Close`.** bigcache launches a background eviction
  goroutine when `LifeWindow > 0` or `CleanWindow > 0`; failing to call
  `Close` leaks that goroutine per instance. The starter wires
  `destroy` for exactly this reason (`project_starter_bigcache`).
- **`AsCache` adapter registers with the driver registry.** The
  starter's contribution to `stdlib/cache` is via the driver registry
  (see `project_stdlib_cache`) so callers write `cache: bigcache` in a
  cache config without importing bigcache directly.
- **`check.sh` needs no docker.** In-process cache has no service
  container — smoke is a plain `go test`.

## 3. Constraints

- **Sizing is up-front and static.** `Shards` must be a power of two;
  `MaxEntriesInWindow` × `MaxEntrySize` roughly bounds the pre-allocated
  backing memory. Runtime resize is not supported.
- **Entries are `[]byte`.** Application types must be
  encoded/decoded by callers; the `stdlib/cache` MultiLevel path uses
  JSON via the shared `ByteStore` seam.
- **`LifeWindow` is TTL, not per-entry.** All entries in one instance
  share the TTL. Different TTL classes = different named instances.

## 4. Trade-offs / Alternatives Rejected

- **freecache / ristretto — not built in.** Both share the same niche
  (in-process cache) so the choice is process-wide anyway; picking one
  keeps the codebase small. bigcache was chosen for stability and
  known-good GC behavior (`project_starter_bigcache`).
- **Auto-loader / refresh cache — rejected.** Loader semantics belong to
  the cache abstraction layer (`stdlib/cache`), not to a backend
  starter. The starter contributes the store; the abstraction
  contributes loader/refresh policies.

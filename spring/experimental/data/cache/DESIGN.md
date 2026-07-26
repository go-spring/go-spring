# cache Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`cache` is the caching abstraction of the stdlib (zero-dependency foundation)
layer: it defines the interface every backend implements and the bridge into
`aspect`, so a caching concern is declared once and served by any backend.

## 1. Responsibilities & Boundaries

- **Does:** define the `Cache` interface, provide an in-process default
  (`Memory`), offer a byte-oriented seam (`ByteStore` + `Codec`) that remote
  backends share, compose caches into a `MultiLevel` hierarchy, expose a
  package-level driver registry, and bridge to `aspect.Store` for the
  `@Cacheable` equivalent.
- **Refuses:**
  - No concrete remote backend. Redis / memcached / bigcache adapters live
    in their respective starters and register themselves under a name.
  - No cache-stampede protection, no async refresh, no negative caching.
    These are policies best expressed at the aspect layer or in the caller.

## 2. Key Abstractions / Seams

- **`Cache` values are `any`.** The interface is symmetrical with
  `aspect.Joinpoint.Result` so a cache lookup on hit can short-circuit an
  aspect chain without a type conversion at the boundary.
- **`ByteStore` + `Codec` is the single serialization seam.** Every remote
  backend natively stores bytes, so lifting a `ByteStore` to a `Cache` via
  `FromByteStore` gives all remote backends one shared serialization path.
  This is the only way remote backends should adapt — do not re-derive
  encoding in a starter.
- **Driver registry** (`Register` / `Get` / `MustGet`) mirrors
  `discovery.Register` and `resilience.RegisterDriver`: panic on empty name,
  nil backend, or duplicate registration, so wiring errors fail loudly at
  init.
- **`AsStore` bridges to `aspect`.** A backend error is treated as a miss and
  a failed `Set` is swallowed, matching aspect's fail-open contract (a broken
  cache must never fail the business call).
- **`MultiLevel` reads near-to-far, writes to every level.** A per-level read
  error does not abort the scan (a nearer failure must not hide a far hit);
  write/delete errors are joined so a partial outage is visible.

## 3. Constraints

- The registry must remain safe to populate during package init and read
  concurrently at runtime; a single `sync.RWMutex` is sufficient.
- Backends must be safe for concurrent use.
- `Memory` and the `ByteStore` codec path are the only two flavours. Any new
  concrete backend that is not a byte store either implements `Cache`
  directly (rare — only makes sense for another in-process layer) or is
  rejected in favour of a `ByteStore`.
- `NewMultiLevel` requires at least one level; an empty hierarchy is a bug,
  not a valid degraded state.

## 4. Trade-offs / Alternatives Rejected

- **`any` values, JSON default codec.** The tension between typed `any`
  values and byte-oriented backends is resolved by the single JSON
  round-trip through `ByteStore + Codec`. This is lossy for concrete struct
  types (a cached struct comes back as `map[string]any`) — a known trade-off.
  The near `Memory` level is unaffected because it stores live values with
  their type, so multi-level deployments keep local reads fast and typed
  while the far level is JSON-friendly.
- **No sliding-window / stampede protection.** Best expressed by the caller
  (e.g. single-flight in the aspect chain, or Redis-native primitives when
  needed) rather than baked into a general interface.
- **No global default backend.** A caller either passes the `Cache` it wants
  or looks it up by name; there is no implicit "the cache". This keeps
  multi-tenant / multi-domain wiring explicit.

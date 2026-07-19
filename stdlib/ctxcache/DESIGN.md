# ctxcache Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `ctxcache` piggybacks on
`context.Context` to give short-lived, request-scoped values a typed,
write-once home that doesn't leak into function signatures.

## 1. Responsibilities & Boundaries

- Own the pattern "one request, one bag of typed values, cleared at the
  boundary". Everything is anchored to a specific context; there is no
  process-global map.
- Not a cache in the "TTL / eviction / hit rate" sense. It is a small,
  guaranteed-scoped, guaranteed-typed key/value store for values whose
  lifecycle is exactly the enclosing request.

## 2. Key Abstractions

- **`Cache`**: mutex-protected `map[any]any` attached to a context via a
  private `cacheKey`. Exactly one Cache per context; a second `Init` on the
  same context is a no-op.
- **`TypedKey[T]`**: keys are `(string, type)` pairs, produced by generics.
  Two different `T`s with the same name are disjoint slots — this is why
  `Get`/`Set` are generic instead of taking a `any` value.
- **Explicit lifecycle**: `Init` returns a `cancel` function. Calling it
  clears the map and permanently marks the Cache as cleared. Subsequent
  `Get`/`Set` return `ErrCacheAlreadyCleared`. There is deliberately no
  "second life" — reuse would silently share state across requests.

## 3. Constraints & Trade-offs

- Write-once per key. This makes lookups a total contract: a value that is
  present cannot mutate under the caller. Callers that want mutability must
  store a pointer or a `sync.Map` under a stable key.
- The cancel function must be called — usually via `defer` at the middleware
  boundary. Missing calls leak the map (small) but do not leak goroutines.
- Concurrent access is safe. Contention is unlikely because request-scoped
  data usually flows sequentially; the single mutex is the simplest correct
  choice.
- Errors are sentinel variables built with `errutil.Explain(nil, ...)`; use
  `errors.Is` to test for them.

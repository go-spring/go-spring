# aspect Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`aspect` supplies the AOP-equivalent capability for Go-Spring in the stdlib
(zero-dependency foundation) layer, so both applications and the framework
itself can attach cross-cutting concerns without dragging in framework wiring.

## 1. Responsibilities & Boundaries

- **Does:** compose ordered interceptors around a target function or HTTP
  handler, expose typed `Chain.Run` / `RunE` / `Around[T]` entry points, and
  ship a small set of general-purpose interceptors that cover the common
  concerns (transaction, cache, timing, recover, pointcut).
- **Refuses:**
  - No bytecode weaving, no reflection-based dynamic proxy, no code
    generation. Go has no runtime metaobject protocol; simulating Java AOP
    would either compromise type safety or the DI container's simplicity.
  - No change to `spring/`, `gs/` or the container. There is no
    `BeanPostProcessor`-equivalent hook. Concerns are attached by writing a
    decorator bean that shares an interface with the concrete implementation.
  - Not a concrete transaction manager or cache backend. `TxManager` and
    `Store` are interfaces; real backends (gorm, redis, ...) live in starters.

## 2. Key Abstractions / Seams

- **`Chain` boundary is `any`**, not a generic parameter. Interceptors such
  as `Cache` need to read or replace the target's return value regardless of
  its type; a strongly typed chain would prevent that. `Around[T]` restores
  type safety at the call site with a single type assertion and no
  reflection.
- **`Joinpoint.Proceed` carries the context.** An interceptor derives a new
  context (e.g. a transaction handle in `Transactional`) and passes it to
  `Proceed`; downstream code discovers the transaction through the context,
  matching Go's idiomatic propagation.
- **`Store` and `TxManager` are the pluggable seams.** Every backend implements
  the same tiny interface; starters register beans that satisfy them (see
  `go-spring.org/spring/cache.AsStore` for the cache bridge). The interceptor
  never imports a concrete backend.
- **`NewHandler` is the HTTP seam.** It mirrors
  `resilience.NewHandler`: 5xx responses count as errors for interceptors
  like `Timing`, and the request is served exactly once even under a retry
  policy — inbound serving is not idempotent.

## 3. Constraints

- Nil / empty chain must be a transparent pass-through (same contract as
  `resilience.Executor`); wiring stays a no-op until an interceptor is
  configured.
- Interceptor at index 0 is outermost; `Chain.With` appends inward without
  mutating the receiver.
- `Chain` and every bundled interceptor must be safe for concurrent use.
- No dependency outside the Go standard library. New built-in interceptors
  that need a concrete backend must accept the backend through an interface,
  not import it.

## 4. Trade-offs / Alternatives Rejected

- **`any` at the boundary vs generic chain.** Chose `any` so heterogeneous
  interceptors (Cache reads results, Transactional does not) can share a
  chain; the loss of static typing is confined to the boundary and repaired
  at call sites by `Around[T]`.
- **Decorator + DI vs container-side proxy.** Chose the decorator convention
  because it is plain Go composition — callers depend only on the interface
  and cannot tell whether they got the plain or decorated bean. A
  container-side proxy would either require reflection wrapping (giving up
  type safety) or codegen (a build-time dependency the project does not
  want).
- **In-process `MemoryStore` bundled.** The framework must cache out of the
  box and in tests. A remote cache (Redis, memcached) plugs in via the same
  `Store` interface without changing the interceptor.

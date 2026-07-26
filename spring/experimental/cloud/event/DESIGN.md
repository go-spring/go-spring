# event Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`event` is the zero-dependency in-process publish/subscribe bus at the stdlib
foundation layer. It closes the Spring `ApplicationEventPublisher` /
`@EventListener` gap using Go idioms (generics + reflect type key) rather than
reproducing annotation scanning or handler-signature reflection.

## 1. Responsibilities & Boundaries

- In-process only. Cross-replica broadcast is a separate concern (a "config
  bus" on top of a message queue) and does not belong here.
- Delivers a published value to handlers subscribed for its **exact dynamic
  type**. Interface subscriptions are deliberately not resolved against
  concrete implementations — routing stays predictable and reflection-free at
  the call site, and Go-idiomatic code uses a concrete struct per event anyway.
- No marker interface, no annotation scanning, no reflect-based handler
  signatures. The only reflection is `reflect.TypeFor[T]()` / `reflect.TypeOf`
  used as a map key.

## 2. Key Abstractions & Seams

- `Bus` interface exposes just `Publish(ctx, any) error` and `Close() error`.
  The concrete `*bus` satisfies an unexported `subscribable` interface that
  the generic `Subscribe[T]` / `SubscribeAsync[T]` functions assert to, so
  callers stay type-safe with zero reflection at the call site.
- `Listener` is the non-generic collection seam for the container: a bean
  exported as `event.Listener` is collected the same way `health.Indicator`
  beans are collected, and its `Register(bus)` internally calls the generic
  `Subscribe[T]` where types are known.
- `SubOption` (`WithOrder` / `WithBuffer` / `WithErrorHandler`) mutates a
  normalized `subOptions`; options are applied per subscription, not per bus.

## 3. Constraints (do not break)

- **Sync errors never short-circuit**: every synchronous handler runs, and the
  errors are combined with `errors.Join`. This mirrors the aspect chain's
  pass-through spirit — one faulty subscriber must not silently suppress the
  rest.
- **Ordering**: lower `WithOrder` runs first (index 0 is outermost, matching
  the aspect chain); ties break by registration sequence for determinism.
- **Async worker channel**: send uses a three-way `select { ch<- / done /
  ctx.Done() }`. Once `done` is closed a send never blocks, so a cancelled
  subscription can never cause a send-on-closed panic or leak the publisher.
- **Graceful drain**: `Close` closes each worker's `done` and then drains any
  events already buffered on the channel before returning — accepted events
  are not silently dropped.
- **Close is a hard error, not empty transparency**: publishing after `Close`
  returns `ErrClosed`. Publishing to a bus with no subscribers is the benign
  no-op case; a closed bus is a programming mistake and is reported.
- **nil transparency at the edges**: `Publish(nil)` is a no-op; `Subscribe`
  on a nil bus or with a nil handler returns a no-op cancel.

## 4. Trade-offs / Alternatives Rejected

- **No interface-based routing**: subscribing to `io.Reader` would not receive
  a concrete `*bytes.Buffer` publish. Adding it would require walking the
  method set on every publish or maintaining a second index; both hurt the
  predictable, cheap type-key routing without a compelling use case in Go.
- **No implicit synchronous drain on Close**: sync handlers run on the
  publisher's goroutine and are already done when Publish returns, so there is
  nothing to wait for. Close is a wait for async workers only.
- **No global bus**: `New()` is per-application. A process-wide singleton
  makes tests flaky and blurs shutdown semantics; container wiring hands the
  bus around explicitly.

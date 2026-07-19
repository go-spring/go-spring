# aspect
[English](README.md) | [中文](README_CN.md)

`aspect` is the Go-idiomatic equivalent of Spring AOP: an explicit, type-safe
interceptor chain plus a decorator convention wired through the DI container.
It lets a cross-cutting concern (transaction, cache, timing, panic recovery,
audit) be declared once and applied around many operations without tangling
into business code — no bytecode weaving, no runtime dynamic proxy.

## Features

- `Chain` of `Interceptor`s wrapped around a target function; index 0 is the
  outermost. A nil or empty chain is a transparent pass-through.
- `Joinpoint{Context, Method, Result, Proceed}` — an interceptor calls
  `Proceed` to continue the chain, or returns a value to short-circuit it.
- `Around[T]` restores static typing at the call site with no reflection.
- Built-in interceptors: `Recover`, `Timing`, `Cache` (with pluggable `Store`
  and a zero-dep `MemoryStore`), `Transactional` (with pluggable `TxManager`),
  `Only` (pointcut on method name).
- `NewHandler` wraps an `http.Handler` so each request flows through the chain
  as a joinpoint; a 5xx response is reported to the chain as an error.

## Installation

```
go get go-spring.org/stdlib
```

## Usage

```go
import (
    "context"

    "go-spring.org/stdlib/aspect"
)

type OrderService interface {
    Place(ctx context.Context, o *Order) error
}

// Business implementation.
type orderService struct{ /* deps */ }

func (s *orderService) Place(ctx context.Context, o *Order) error { /* ... */ return nil }

// Decorator sharing the same interface; the chain is invisible to callers.
type orderServiceAspect struct {
    inner OrderService
    chain *aspect.Chain
}

func (a *orderServiceAspect) Place(ctx context.Context, o *Order) error {
    return a.chain.RunE(ctx, "OrderService.Place",
        func(ctx context.Context) error { return a.inner.Place(ctx, o) })
}

func newChain(tm aspect.TxManager) *aspect.Chain {
    return aspect.NewChain(
        aspect.Recover(),
        aspect.Timing(func(m string, d time.Duration, err error) { /* metric */ }),
        aspect.Transactional(tm),
    )
}
```

For a strongly-typed return value use `Around`:

```go
order, err := aspect.Around(chain, ctx, "PlaceOrder",
    func(ctx context.Context) (*Order, error) { return repo.Insert(ctx, o) })
```

For HTTP servers, wrap the mux instead of writing a decorator:

```go
h := aspect.NewHandler(mux, chain, func(r *http.Request) string { return r.URL.Path })
```

# transaction
[English](README.md) | [中文](README_CN.md)

`transaction` is the zero-dependency Saga abstraction for cross-resource and
cross-service consistency — the Go-idiomatic equivalent of Seata Saga and
Spring's `@GlobalTransactional`. A long-running business operation is expressed
as an ordered list of compensable `Step`s; when a step fails, the already-
succeeded steps are undone in reverse order by their `Compensate` functions.

For TCC and AT patterns see the subpackages
[`transaction/tcc`](tcc/README.md) and [`transaction/at`](at/README.md).

## Features

- `Saga` + `Step{Action, Compensate}` — plain Go, no annotations.
- `Coordinator` interface with a bundled in-process implementation
  (`NewCoordinator`).
- `Store` seam for the saga log; bundled `MemoryStore`; durable backend is a
  starter-supplied bean.
- `Recover(ctx, s)` — backward recovery: replays compensation for whatever the
  crashed process might have effected.
- `Observer` seam for otel spans without stdlib depending on otel.
- `StepRegistry` + `GlobalTransactional(coord, reg)` — the aspect-level
  `@GlobalTransactional` equivalent, keyed by method name.
- Step-level `RetryPolicy` (aliased to `resilience.Policy`) reuses the same
  knob set as outbound resilience.

## Usage

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/stdlib/transaction"
)

func main() {
    coord := transaction.NewCoordinator(transaction.WithStore(&transaction.MemoryStore{}))

    saga := transaction.Saga{
        ID:     "order-42",
        Method: "PlaceOrder",
        Steps: []transaction.Step{
            {
                Name:       "reserve-stock",
                Action:     func(ctx context.Context) (any, error) { return "res-1", nil },
                Compensate: func(ctx context.Context, r any) error { fmt.Println("undo stock", r); return nil },
            },
            {
                Name:       "charge-card",
                Action:     func(ctx context.Context) (any, error) { return nil, fmt.Errorf("card declined") },
                Compensate: func(ctx context.Context, r any) error { return nil },
            },
        },
    }

    res, err := coord.Execute(context.Background(), saga)
    fmt.Println(res.Status, err) // Compensated, card declined
}
```

## Aspect (`@GlobalTransactional`) form

```go
reg := transaction.NewStepRegistry()
reg.Register("PlaceOrder",
    transaction.Step{Name: "reserve-stock", Action: reserveStock, Compensate: releaseStock},
    transaction.Step{Name: "charge-card",   Action: charge,       Compensate: refund},
)
gtx := transaction.GlobalTransactional(coord, reg)
// Compose gtx into an aspect chain (see stdlib/aspect); the aspect drives the
// saga when the joinpoint's method name matches a registration, and proceeds
// transparently otherwise.
```

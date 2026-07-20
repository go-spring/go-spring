# tcc
[English](README.md) | [中文](README_CN.md)

`tcc` is the zero-dependency abstraction for the TCC (Try / Confirm / Cancel)
distributed-transaction pattern — the Go-idiomatic equivalent of Seata TCC. It
targets short, strongly-consistent transactions where a Try phase reserves
resources without exposing final results, and a global Confirm or Cancel
resolves them.

Sibling patterns Saga and AT live in [`spring/transaction`](../README.md) and
[`spring/transaction/at`](../at/README.md).

## Features

- `Transaction` + `Participant{Try, Confirm, Cancel}` — plain Go, no
  annotations, no proxy magic.
- `Coordinator.Execute(ctx, t)` — try all, decide, then confirm-all-in-order
  or cancel-tried-in-reverse.
- `Coordinator.Recover(ctx, t)` — forward-recovery from `StatusConfirming`,
  backward from `StatusTrying` / `StatusCancelling`.
- `Store` seam persists the decision log; bundled `MemoryStore` for tests;
  durable backend is a starter-supplied bean.
- `Observer` seam for otel spans without stdlib depending on otel.
- `ParticipantRegistry` + `GlobalTCC(coord, reg)` aspect — the AOP form,
  keyed by method name.
- `RetryPolicy = resilience.Policy` alias reuses the outbound resilience
  knob set; recommended non-zero for Confirm/Cancel since the TCC contract
  requires them to eventually succeed.

## Usage

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/spring/transaction/tcc"
)

func main() {
    coord := tcc.NewCoordinator(tcc.WithStore(&tcc.MemoryStore{}))

    tx := tcc.Transaction{
        ID:     "order-42",
        Method: "PlaceOrder",
        Participants: []tcc.Participant{
            {
                Name:    "stock",
                Try:     func(ctx context.Context) (any, error) { return "hold-1", nil },
                Confirm: func(ctx context.Context, r any) error { fmt.Println("commit hold", r); return nil },
                Cancel:  func(ctx context.Context, r any) error { fmt.Println("release hold", r); return nil },
            },
            {
                Name:    "balance",
                Try:     func(ctx context.Context) (any, error) { return "freeze-9", nil },
                Confirm: func(ctx context.Context, r any) error { return nil },
                Cancel:  func(ctx context.Context, r any) error { return nil },
            },
        },
    }

    res, err := coord.Execute(context.Background(), tx)
    fmt.Println(res.Status, err) // Committed <nil>
}
```

## Participant Obligations

- **Idempotent Confirm/Cancel** — a crash-driven retry may replay them.
- **Empty rollback** — Cancel must tolerate a nil result (Try never
  recorded one).
- **Anti-hanging** — a delayed Try that arrives after a Cancel must not
  re-reserve; key reservations by the transaction id.

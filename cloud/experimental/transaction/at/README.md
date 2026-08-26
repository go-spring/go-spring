# at
[English](README.md) | [中文](README_CN.md)

`at` is the zero-dependency abstraction for AT (Automatic Transaction) — the
Go-idiomatic equivalent of Seata AT. Unlike Saga or TCC, business code writes
only the forward SQL; a resource-side interceptor captures a before-image for
each statement, and rollback restores rows from those images automatically.

## Features

- `Coordinator` with `Begin` / `Register` / `Commit` / `Rollback`; XID rides
  the context via `WithXID` / `XIDFromContext`.
- `Branch` interface — implemented by a backend starter that persists undo
  logs and restores rows (see `starter-transaction-at-gorm`).
- `GlobalLock` interface + bundled `MemoryGlobalLock` for write-write isolation
  (`ErrLockConflict` on conflict); a distributed deployment supplies a shared
  backend.
- `Observer` seam for otel spans (nil disables observation).
- `RetryPolicy = resilience.Policy` alias for second-phase retries.
- `GlobalAT(coord)` decorator — the AT `@GlobalTransactional` equivalent; no
  per-method registry needed.

## Usage

Wire the coordinator with a global lock and wrap business methods with the
decorator:

```go
package main

import (
    "context"

    "go-spring.org/cloud/experimental/transaction/at"
)

var coord = at.NewCoordinator(
    at.WithGlobalLock(&at.MemoryGlobalLock{}),
)

// global is the AT @GlobalTransactional equivalent: wrap the business method
// by hand (there is no shared interceptor protocol — each call site nests its
// own decorator). The GORM backend registers each branch when it sees an XID
// on the context; business code just runs plain SQL.
var global = at.GlobalAT(coord)

func PlaceOrder(ctx context.Context) error {
    // Any writes made via the AT-aware ORM below are captured and, on error
    // return, undone automatically.
    return global(ctx, "PlaceOrder", func(ctx context.Context) error {
        return nil // real business logic here
    })
}
```

The coordinator begins a global transaction on entry, injects the XID into
`ctx`, and resolves the transaction on return: commit on nil error, rollback
otherwise. A nested `GlobalAT` reuses the outer XID (no nested global
transactions).

# starter-transaction-saga

[English](README.md) | [中文](README_CN.md)

`starter-transaction-saga` contributes the **Saga** distributed-transaction
capability defined in
[`go-spring.org/stdlib/transaction`](../../stdlib/transaction) to a Go-Spring
application. It is the Go-idiomatic equivalent of `@GlobalTransactional(SAGA)`,
reached with an in-process coordinator and an aspect chain rather than by
replicating Seata's TC/TM/RM roles or requiring bytecode magic.

It is a **Contributor**-archetype starter (see [DESIGN.md](../DESIGN.md) §2.3):
it opens no port and starts no server, only registers beans.

## Saga vs. TCC — which one?

Both patterns are provided; pick by consistency need:

| | **Saga** (`starter-transaction-saga`) | **TCC** (`starter-transaction-tcc`) |
|---|---|---|
| Forward step | takes real effect immediately | *reserves* a resource (invisible as committed) |
| On failure | compensating business function undoes the effect | second phase cancels the reservation |
| Isolation | none — an undone value is briefly visible | reservation is not business-visible between Try and Confirm |
| Best for | long, cross-service flows (MQ, HTTP, cache) | short, strongly-consistent flows (freeze balance / hold stock) |

## Installation

```bash
go get go-spring.org/starter-transaction-saga
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-transaction-saga"
```

The container now holds three beans:

- a `transaction.Coordinator` — the in-process orchestrator that runs each
  step and compensates in reverse on failure;
- a `*transaction.StepRegistry` — where application code declares each
  method's steps;
- a `transaction.Store` — the in-memory default, replaceable by a durable
  Store (see below).

### 2. Declare steps

Each step has a forward `Action` and a `Compensate`. Both must be
**idempotent**: a crash can retry the action, and recovery can replay the
compensation. Return values from earlier steps are visible via
`StepResults` on later ones.

```go
deductInventory := transaction.Step{
    Name:       "DeductInventory",
    Action:     func(ctx context.Context, r *transaction.StepResults) (any, error) { ... },
    Compensate: func(ctx context.Context, r *transaction.StepResults) error { ... },
}
```

### 3. Run a saga

Inject the coordinator and execute directly:

```go
type OrderService struct {
    Coord transaction.Coordinator `autowire:""`
}

func (s *OrderService) Place(ctx context.Context, id string) (transaction.Result, error) {
    return s.Coord.Execute(ctx, transaction.Saga{
        ID:     id,
        Method: "OrderService.Place",
        Steps:  []transaction.Step{deductInventory, chargePayment, publishEvent},
    })
}
```

### 4. Or declare it as `@GlobalTransactional`

Register the same steps under a method name and wire
`transaction.GlobalTransactional` into an aspect chain — the no-reflection
equivalent of `@GlobalTransactional(SAGA)`:

```go
func RegisterOrder(reg *transaction.StepRegistry) {
    reg.Register("OrderService.Place", deductInventory, chargePayment, publishEvent)
}

chain := aspect.NewChain(transaction.GlobalTransactional(coord, reg))
```

Steps must be registered at **wiring time** (bean construction), never from a
custom `Runner`, so the registry is populated before the startup recovery
scan runs.

## Configuration

Bound under `${spring.transaction.saga}` (the `spring.transaction` capability
namespace is shared with `starter-transaction-tcc`).

| Key | Default | Description |
|---|---|---|
| `spring.transaction.saga.enabled` | `true` | Turn the starter's beans on/off. |
| `spring.transaction.saga.tracing` | `true` | Emit an otel child span per step phase on the globals `starter-otel` installs. No-op without it. |
| `spring.transaction.saga.recover-on-start` | `true` | At startup, scan the Store and compensate any saga a crash left in flight. |

## Crash recovery

By default the saga log is kept **in memory only** — enough for the common
single-process case and for tests, but not crash-recoverable. When
`recover-on-start` is true, a `gs.Runner` scans the `transaction.Store` at
startup and, for each `StatusRunning` snapshot, rebuilds the saga's steps
from the `StepRegistry` (keyed by method name) and hands it to the
coordinator for backward recovery. A saga whose steps are no longer
registered is logged and skipped — recovery cannot fabricate business
logic. The scan is a harmless no-op with the in-memory Store, whose
`Pending` is always empty after a restart.

To make recovery meaningful, import a durable Store — currently
[`starter-transaction-saga-gorm`](../starter-transaction-saga-gorm). The
default in-memory Store is registered with `gs.OnMissingBean`, so a
contributed `transaction.Store` takes over both the coordinator and the
recovery scan without any change to business code.

## Observability

When `tracing` is true, the coordinator emits an otel child span per step
phase (`saga.action <step>`, `saga.compensate <step>`) tagged with
`saga.id`, `saga.step`, and `saga.phase`. Failures are recorded on the span.
The `Observer` seam lives in the coordinator so `stdlib/transaction` stays
free of an otel dependency.

## License

Apache 2.0. See [LICENSE](../../LICENSE).

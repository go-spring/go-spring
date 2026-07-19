# starter-transaction-tcc

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-transaction-tcc` contributes the **TCC (Try / Confirm / Cancel)**
distributed-transaction capability defined in
[`go-spring.org/stdlib/transaction/tcc`](../../stdlib/transaction/tcc) to a
Go-Spring application. It is the Go-idiomatic equivalent of Seata TCC, reached
without replicating Seata's TC/TM/RM roles or requiring bytecode/proxy magic.

It is a **Contributor**-archetype starter (see [DESIGN.md](../DESIGN.md) §2.3):
it opens no port and starts no server, only registers beans.

## TCC vs. Saga — which one?

Go-Spring ships two distributed-transaction patterns; pick by consistency need:

| | **Saga** (`starter-transaction-saga`) | **TCC** (`starter-transaction-tcc`) |
|---|---|---|
| Forward step | takes real effect immediately | **reserves** a resource (tentative, invisible as committed) |
| On failure | compensating business function undoes the effect | second phase **cancels** the reservation |
| Isolation | none — an undone value is briefly visible | reservation is not business-visible between Try and Confirm |
| Best for | long, cross-service flows (MQ, HTTP, cache) | short, strongly-consistent flows (freeze balance / hold stock) |

Reach for TCC when a resource must be *held* between the try and the commit;
reach for Saga when each step's effect is real and undone by compensation.

## Installation

```bash
go get go-spring.org/starter-transaction-tcc
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-transaction-tcc"
```

The container now holds two beans:

- a `tcc.Coordinator` — the in-process orchestrator;
- a `*tcc.ParticipantRegistry` — where you declare each method's participants.

### 2. Define participants

Each participant splits its work into three **idempotent** phases. `Try` reserves
the resource and returns a token; `Confirm` commits the reservation; `Cancel`
releases it. All three are required.

```go
reserveStock := tcc.Participant{
    Name:    "ReserveStock",
    Try:     func(ctx context.Context) (any, error) { return stock.reserve(txID, qty) },
    Confirm: func(ctx context.Context, tried any) error { return stock.commit(txID) },
    Cancel:  func(ctx context.Context, tried any) error { return stock.release(txID) },
}
```

### 3. Run a transaction

Inject the coordinator and execute directly:

```go
type OrderService struct {
    Coord tcc.Coordinator `autowire:""`
}

func (s *OrderService) Place(ctx context.Context, txID string) (tcc.Result, error) {
    return s.Coord.Execute(ctx, tcc.Transaction{
        ID:           txID,
        Participants: []tcc.Participant{reserveStock, freezeBalance},
    })
}
```

The coordinator tries every participant in order. If all succeed it confirms
them all; if any `Try` fails it cancels the tried ones in reverse. See
[example/example.go](example/example.go) for a runnable stock + balance demo
covering both the commit and rollback paths.

### 4. Or declare it as `@GlobalTransactional`

Register participants under a method name and wire `GlobalTCC` into an aspect
chain — the no-reflection equivalent of `@GlobalTransactional(TCC)`:

```go
func RegisterOrder(reg *tcc.ParticipantRegistry) {
    reg.Register("OrderService.Place", reserveStock, freezeBalance)
}

chain := aspect.NewChain(tcc.GlobalTCC(coord, reg))
```

Set the transaction id at the edge with `tcc.WithTransactionID(ctx, id)` so it
aligns with your idempotency key.

## Participant obligations

Because a crash can interrupt any phase and recovery replays the second phase,
your participants must handle the three classic TCC hazards — this starter
cannot enforce them across process boundaries:

- **Idempotence** — `Confirm`/`Cancel` may be called more than once; the second
  call must be a no-op.
- **Empty rollback** — `Cancel` may run for a participant whose `Try` never
  recorded a result (its value is then `nil`); it must do nothing in that case.
- **Anti-hanging** — a delayed `Try` arriving after a `Cancel` must not
  re-reserve. Keying reservations by the transaction id lets you detect this.

## Configuration

Bound under `${spring.transaction.tcc}` (shared with Saga under the
`spring.transaction` capability namespace).

| Key | Default | Description |
|---|---|---|
| `spring.transaction.tcc.enabled` | `true` | Turn the starter's beans on/off. |
| `spring.transaction.tcc.tracing` | `true` | Emit an otel child span per participant phase on the globals `starter-otel` installs. No-op without it. |
| `spring.transaction.tcc.recover-on-start` | `true` | Scan the Store at startup and drive interrupted transactions to their decided outcome. |

## Crash recovery

By default the TCC log is kept **in memory only** — enough for the common
single-process case and for tests, but not crash-recoverable. When
`recover-on-start` is true, a `gs.Runner` scans the `tcc.Store` at startup and,
for each interrupted transaction, **confirms** it forward if a commit decision
was recorded or **cancels** it backward otherwise. Recovery rebuilds the
participants from the `ParticipantRegistry` keyed by the persisted method name,
so you MUST register participants at wiring time (bean construction), not from a
custom `Runner`.

To make recovery meaningful, import a durable-`Store` starter
(`spring.transaction.tcc.store=...`); because the in-memory default is registered
with `gs.OnMissingBean`, the durable Store then takes over both the coordinator
and the startup recovery scan.

## License

Apache 2.0. See [LICENSE](../../LICENSE).

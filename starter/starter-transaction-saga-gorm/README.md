# starter-transaction-saga-gorm

[English](README.md) | [中文](README_CN.md)

`starter-transaction-saga-gorm` contributes a **durable, gorm-backed**
[`transaction.Store`](../../spring/transaction) so a Go-Spring application can
recover Sagas a crash left in flight. It is the persistence side-car for
[`starter-transaction-saga`](../starter-transaction-saga): the coordinator
writes its saga log here, and the startup recovery `Runner` reads back
in-flight snapshots.

It is a **Contributor**-archetype starter (see [DESIGN.md](../DESIGN.md)
§2.3): it opens no port. The saga starter registers its in-memory default
Store with `gs.OnMissingBean`, so contributing this Store makes the default
step aside — crash recovery is switched on with **no code change**.

## Installation

```bash
go get go-spring.org/starter-transaction-saga-gorm
```

## Quick Start

### 1. Import a `*gorm.DB` and this Store

The Store autowires an existing `*gorm.DB`, so pair it with any gorm-driver
starter you already use (mysql, postgres, sqlserver, clickhouse).

```go
import (
    _ "go-spring.org/starter-gorm-mysql"        // provides *gorm.DB
    _ "go-spring.org/starter-transaction-saga"  // Saga capability
    _ "go-spring.org/starter-transaction-saga-gorm"
)
```

### 2. Select this Store

```properties
spring.transaction.saga.store=gorm
```

At construction the Store calls `db.AutoMigrate(&sagaSnapshot{})` and fails
fast if the table cannot be created. The table is pinned as
`saga_snapshots` regardless of gorm's pluralization rules.

## Schema

| Column         | Type    | Notes                                          |
| -------------- | ------- | ---------------------------------------------- |
| `id`           | pk      | saga id                                        |
| `method`       | string  | rebuilds steps via `StepRegistry.Lookup`       |
| `status`       | int     | indexed; `Pending` scans `StatusRunning`       |
| `in_progress`  | string  | step currently executing                       |
| `completed`    | text    | JSON-encoded `[]string`                        |
| `step_results` | text    | JSON-encoded `map[string]any`                  |
| `updated_at`   | time    | last write                                     |

Slice and map fields are stored **JSON-encoded** in text columns so the
schema stays backend-agnostic (no dialect-specific array/JSON types).

## JSON round-trip caveat

`Step.Action` results are stored as JSON, so on recovery a value comes back
in its JSON form — a number becomes `float64`, a struct becomes
`map[string]any`, and so on, not its original Go type. Sagas that must
survive a crash should keep Action results JSON-friendly (ids, tokens, and
other scalars) and avoid relying on rich Go types in `Compensate`. The
in-progress step is always recovered with a nil result, so it sidesteps
this entirely.

## Configuration

Bound under `${spring.transaction.saga.gorm}`.

| Key | Default | Description |
|---|---|---|
| `spring.transaction.saga.store` | (unset) | Must be `gorm` for this Store to register. |
| `spring.transaction.saga.gorm.db` | `` | Named `*gorm.DB` instance to select when the app registers several. The first version always autowires the default instance; the field is currently informational. |

## License

Apache 2.0. See [LICENSE](../../LICENSE).

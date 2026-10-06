# starter-transaction-tcc-gorm

[English](README.md) | [中文](README_CN.md)

`starter-transaction-tcc-gorm` contributes a **durable, gorm-backed**
[`tcc.Store`](../../../cloud/experimental/transaction/tcc) so a Go-Spring application can
recover TCC transactions a crash left in flight. It is the persistence
side-car for [`starter-transaction-tcc`](../starter-transaction-tcc): the
coordinator writes its TCC log here, and the startup recovery `Runner` reads
back in-flight snapshots.

It is a **Contributor**-archetype starter (see [DESIGN.md](../../DESIGN.md)
§2.3): it opens no port. The tcc starter registers its in-memory default
Store with `gs.OnMissingBean`, so contributing this Store makes the default
step aside — crash recovery is switched on with **no code change**.

## Installation

```bash
go get go-spring.org/starter-transaction-tcc-gorm
```

## Quick Start

### 1. Import a `*gorm.DB` and this Store

The Store autowires an existing `*gorm.DB`, so pair it with any gorm-driver
starter you already use (mysql, postgres, sqlserver, clickhouse).

```go
import (
    _ "go-spring.org/starter-gorm-mysql"       // provides *gorm.DB
    _ "go-spring.org/starter-transaction-tcc"  // TCC capability
    _ "go-spring.org/starter-transaction-tcc-gorm"
)
```

### 2. Select this Store

```properties
spring.transaction.tcc.store=gorm
```

At construction the Store calls `db.AutoMigrate(&tccSnapshot{})` and fails
fast if the table cannot be created. The table is pinned as
`tcc_snapshots` regardless of gorm's pluralization rules.

## Schema

| Column         | Type    | Notes                                          |
| -------------- | ------- | ---------------------------------------------- |
| `id`           | pk      | transaction id                                 |
| `method`       | string  | rebuilds participants via `ParticipantRegistry.Lookup` |
| `status`       | int     | indexed; `Pending` scans the non-terminal statuses |
| `tried`        | text    | JSON-encoded `[]string`                        |
| `in_progress`  | string  | participant currently trying                   |
| `try_results`  | text    | JSON-encoded `map[string]any`                  |
| `updated_at`   | time    | last write                                     |

Slice and map fields are stored **JSON-encoded** in text columns so the
schema stays backend-agnostic (no dialect-specific array/JSON types).

## JSON round-trip caveat

`Participant.Try` results are stored as JSON, so on recovery a value comes
back in its JSON form — a number becomes `float64`, a struct becomes
`map[string]any`, and so on, not its original Go type. Transactions that must
survive a crash should keep Try results JSON-friendly (ids, tokens, and other
scalars) and avoid relying on rich Go types in `Confirm`/`Cancel`. The
in-progress participant is always recovered with a nil result, so it sidesteps
this entirely.

## Configuration

Bound under `${spring.transaction.tcc.gorm}`.

| Key | Default | Description |
|---|---|---|
| `spring.transaction.tcc.store` | (unset) | Must be `gorm` for this Store to register. |

The `${spring.transaction.tcc.gorm}` prefix currently carries no keys of its
own: the `*gorm.DB` is autowired from the container (the default instance when
several are registered).

## License

Apache 2.0. See [LICENSE](../../../LICENSE).

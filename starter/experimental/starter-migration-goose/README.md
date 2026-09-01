# starter-migration-goose

[English](README.md) | [中文](README_CN.md)

`starter-migration-goose` runs [goose](https://github.com/pressly/goose) schema
migrations at startup over any gorm `*gorm.DB`. Goose owns the algorithm
(versioning, checksums, transactions, dialect quirks); this starter owns the
wiring: config keys, the IoC lifecycle and the gorm→`*sql.DB` bridge.

## Installation

```bash
go get go-spring.org/starter-migration-goose
```

## Quick Start

```go
import _ "go-spring.org/starter-migration-goose"
```

```properties
spring.migration.app.db-ref=app       # *gorm.DB bean name; optional when only one exists
spring.migration.app.dir=./sql        # directory of goose SQL migrations
```

The starter registers a `gs.Runner` that runs after every bean is wired but
before any server serves: it applies every pending migration
(`00001_init.sql`, `00002_seed.sql`, ...) forward-only, each in its own
transaction, recording them in `goose_db_version`. A failure aborts startup —
a broken schema never serves traffic. Multiple databases are multiple entries
under `spring.migration`.

Migration files use goose's SQL format (`-- +goose Up` / `-- +goose Down`
directives); the Down halves are never executed (forward-only).

## Configuration

| Key | Default | Description |
| --- | --- | --- |
| `spring.migration.<name>.enabled` | `true` | Set `false` to keep the config but skip the run. |
| `spring.migration.<name>.db-ref` | (only bean) | The `*gorm.DB` bean to migrate. Required when several exist. |
| `spring.migration.<name>.dir` | — | Directory of goose SQL migrations (required). |
| `spring.migration.<name>.table` | `goose_db_version` | Version-table name. |
| `spring.migration.<name>.allow-missing` | `false` | Permit gap-filling migrations below the highest applied version. |

## Notes

- Dialects: mysql, postgres, sqlite, sqlserver, clickhouse (mapped from the
  gorm dialector; anything else fails fast).
- For a self-contained single binary, copy the embedded migrations to a temp
  dir at startup and point `dir` there.
- See [example/](example/) for a runnable end-to-end demo (in-memory sqlite,
  startup apply + idempotency self-assertions, `check.sh` smoke).

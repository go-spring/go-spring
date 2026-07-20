# starter-repository-gorm

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-repository-gorm` is the [gorm](https://gorm.io)-backed implementation of
the framework-neutral [`go-spring.org/spring/repository`](../../spring/repository)
abstraction. It translates a `repository.Query` into gorm's chained builder and
returns a ready-to-use generic `repository.Repository[T, ID]` over any
`*gorm.DB` — the Go-idiomatic equivalent of a Spring Data JPA repository, without
JPA or method-name query parsing.

It is a **library-first integration module**, not a blank-import starter: a
repository is parameterised over a domain type the application owns, so there is
nothing to auto-register. It is **driver-agnostic** — the concrete database
(MySQL, Postgres, SQL Server, ClickHouse, sqlite, ...) is whichever
`starter-gorm-*` published the `*gorm.DB` bean.

## Installation

```bash
go get go-spring.org/starter-repository-gorm
```

## Usage

### Inline over a `*gorm.DB` you already hold

The natural shape inside a service constructor:

```go
import (
    reposgorm "go-spring.org/starter-repository-gorm"
    "go-spring.org/spring/repository"
    "gorm.io/gorm"
)

type UserService struct {
    repo repository.Repository[User, int64]
}

func newUserService(db *gorm.DB) *UserService {
    return &UserService{repo: reposgorm.For[User, int64](db, "users")}
}
```

`For` resolves the primary-key column from `T`'s gorm schema (falling back to
`"id"`).

### As a named IoC bean

Register `For` through a plain `gs.Provide` constructor so other beans autowire
the repository by interface, never learning it is gorm-backed:

```go
gs.Provide(func(db *gorm.DB) repository.Repository[User, int64] {
    return reposgorm.For[User, int64](db, "users",
        repository.WithPrincipal(currentUser)) // enable audit CreatedBy
}).Name("userRepo")
```

### CRUD, paging, composite conditions

```go
_ = repo.Create(ctx, &User{Name: "Ann"})
u, found, _ := repo.FindByID(ctx, 1)

page, _ := repo.FindPage(ctx, repository.NewQuery().
    Where("age", repository.Ge, 18).
    Where("name", repository.Like, "A%").
    OrderByDesc("created_at").
    Slice(0, 20))
// page.Items is the window; page.Total is the count of all matches.
```

### Audit fields

Make the entity implement `repository.Auditable` and enable it with
`repository.WithPrincipal`; `CreatedAt`/`UpdatedAt` fill on write and `CreatedBy`
comes from the request context:

```go
func (u *User) SetCreatedAt(t time.Time) { u.CreatedAt = t }
func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
func (u *User) SetCreatedBy(who string)  { u.CreatedBy = who }
```

## Query translation

| `repository.Op` | SQL |
|---|---|
| `Eq` / `Ne` | `field = ?` / `field <> ?` |
| `Gt` / `Ge` / `Lt` / `Le` | `field > ?` / `>= ?` / `< ?` / `<= ?` |
| `In` | `field IN ?` (binds a slice) |
| `Like` | `field LIKE ?` (caller supplies wildcards) |

`Sort` becomes `ORDER BY field ASC|DESC`; `Page` becomes `OFFSET`/`LIMIT`.
`FindPage` runs one windowed `Find` for the items and one filter-only `Count` for
the total. Field names are validated as identifiers before interpolation; values
always ride gorm's parameter binding.

## Design

This is a database-integration module built on the shared starter conventions —
see [starter/DESIGN.md](../DESIGN.md). A second store (e.g. Mongo) is a separate
`repository.Backend` implementation and does not touch this module or the
abstraction.

## Example

A runnable smoke test lives in [`example/`](example): it wires the repository as
an IoC bean over in-memory sqlite and drives CRUD, paging, composite conditions
and audit population. Run it with `example/check.sh`.

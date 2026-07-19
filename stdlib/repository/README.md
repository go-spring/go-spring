# repository
[English](README.md) | [中文](README_CN.md)

`repository` is a zero-dependency, framework-agnostic generic data-access
abstraction — the Go-idiomatic equivalent of Spring Data's `CrudRepository` +
`PagingAndSortingRepository`. A single [`Repository[T, ID]`](repository.go)
interface gives a domain type CRUD, sorted/paged reads, and automatic audit
fields, and any store can serve it by implementing the [`Backend[T, ID]`](repository.go)
seam. There is no query DSL and no derived-query parsing: a read is expressed
with a plain [`Query`](query.go) value.

## Features

- Generic `Repository[T, ID]` — `Create`, `Save`, `FindByID`, `ExistsByID`,
  `Delete`, `Count`, `FindAll`, `FindPage`. No reflection-based method-name
  parsing; the type parameters carry the entity and its key.
- `Query` = `Filters []Cond` + `Sort []Order` + `Page Pageable` — a small,
  data-store-neutral Specification. `Op` is a closed set (`Eq/Ne/Gt/Ge/Lt/Le/In/Like`)
  so every backend can cover all of it. Build one with the fluent
  `NewQuery().Where(...).OrderBy(...).Slice(...)`.
- `Page[T]{ Items, Total, Offset, Limit }` with `HasNext()` — `FindPage`
  composes it from a list plus an independent count, so a paginator renders in
  one call.
- `Auditable` optional interface — implement `SetCreatedAt/SetUpdatedAt/SetCreatedBy`
  and the repository fills them on write. `CreatedBy` comes from the context via
  the `PrincipalFunc` seam (`WithPrincipal`), aligning with the security layer's
  current subject. Timestamps use an injectable `Clock` (`WithClock`).
- `Backend[T, ID]` seam — the single interface a store implements (translate a
  `Query` into its own query). It is a bean-type swap, not a global driver
  registry, because a backend is bound to a live client.

## Usage

```go
package main

import (
    "context"
    "time"

    "go-spring.org/stdlib/repository"
)

type User struct {
    ID        int64
    Name      string
    CreatedAt time.Time
    UpdatedAt time.Time
    CreatedBy string
}

// Opt in to auto-audit by implementing Auditable.
func (u *User) SetCreatedAt(t time.Time) { u.CreatedAt = t }
func (u *User) SetUpdatedAt(t time.Time) { u.UpdatedAt = t }
func (u *User) SetCreatedBy(who string)  { u.CreatedBy = who }

func demo(repo repository.Repository[User, int64]) error {
    ctx := context.Background()

    // Create + audit
    if err := repo.Create(ctx, &User{ID: 1, Name: "Ann"}); err != nil {
        return err
    }

    // Composite conditions + sort + paging
    page, err := repo.FindPage(ctx, repository.NewQuery().
        Where("name", repository.Like, "A%").
        OrderByDesc("created_at").
        Slice(0, 20))
    if err != nil {
        return err
    }
    _ = page.Items
    _ = page.Total // count of all matches, not just this window
    return nil
}
```

`Repository` is obtained from `repository.New(backend, opts...)` over a
`Backend`. The gorm implementation and its `For[T, ID](db, table)` factory live
in [`starter-repository-gorm`](../../starter/starter-repository-gorm); a second
store (Mongo) is just another `Backend` and leaves this package untouched.

## Design

See [DESIGN.md](DESIGN.md).

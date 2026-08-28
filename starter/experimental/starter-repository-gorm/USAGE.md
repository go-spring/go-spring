# starter-repository-gorm Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against this module (`starter.go`, `backend.go`), the abstraction it implements
(`cloud/data/repository`: `repository.go`, `query.go`, `audit.go`) and the runnable,
self-asserting [example/](example/) (in-memory sqlite, `check.sh` smoke). **GORM's own
chained-builder semantics are [gorm's documentation](https://gorm.io/docs/)** — everything
below is the go-spring increment.

**Activation**: none. This is a library-first integration module, not a blank-import starter —
a `repository.Repository` is parameterised over a domain type the application owns, so nothing
can usefully auto-register (`starter.go:22-25`). The single entry point is `For`.

---

## 1. Complete worked project

A person-service with audit fields, paging and a smoke-verifiable behavior contract:

```
demo/
├── go.mod
├── main.go
├── person.go          # entity + Auditable implementation
├── service.go         # business service depending on the Repository interface only
└── conf/app.properties
```

**go.mod** (deps that matter):

```
require (
    gorm.io/gorm                      latest
    gorm.io/driver/sqlite             latest   // or mysql/postgres via a starter-gorm-* starter
    go-spring.org/spring              v1.3.x
    go-spring.org/starter-gorm-sqlite latest   // publishes the *gorm.DB bean (any dialect)
    go-spring.org/starter-repository-gorm latest
)
```

**person.go** — the entity owns its audit plumbing:

```go
package main

import (
    "context"
    "time"

    "go-spring.org/cloud/data/repository"
)

type principalKey struct{}

func withUser(ctx context.Context, user string) context.Context {
    return context.WithValue(ctx, principalKey{}, user)
}

func currentUser(ctx context.Context) string {
    s, _ := ctx.Value(principalKey{}).(string)
    return s
}

// Person implements repository.Auditable: three setters, no per-field code.
type Person struct {
    ID        int64     `gorm:"primaryKey;column:id"`
    Name      string    `gorm:"column:name"`
    Age       int       `gorm:"column:age"`
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
    CreatedBy string    `gorm:"column:created_by"`
}

func (Person) TableName() string { return "people" }
func (p *Person) SetCreatedAt(t time.Time) { p.CreatedAt = t }
func (p *Person) SetUpdatedAt(t time.Time) { p.UpdatedAt = t }
func (p *Person) SetCreatedBy(who string)  { p.CreatedBy = who }
```

**service.go** — business code never learns the backend is gorm:

```go
package main

import (
    "context"

    "go-spring.org/cloud/data/repository"
    "go-spring.org/spring/gs"
    reposgorm "go-spring.org/starter-repository-gorm"
    "gorm.io/gorm"
)

type PersonService struct {
    Repo repository.Repository[Person, int64] `autowire:""`
}

func newPersonService(repo repository.Repository[Person, int64]) *PersonService {
    return &PersonService{Repo: repo}
}

func init() {
    gs.Provide(func(db *gorm.DB) repository.Repository[Person, int64] {
        return reposgorm.For[Person, int64](db, "people",
            repository.WithPrincipal(currentUser)) // CreatedBy from the request context
    })
    gs.Provide(newPersonService).Export(gs.As[gs.Rooter]())
}
```

**main.go**:

```go
package main

import "go-spring.org/spring/gs"

func main() { gs.Run() }
```

**conf/app.properties**: nothing to configure — the module has **zero config keys**
(`schema.json` declares an empty property set; the example's config file exists only so gs
has a directory to load). The concrete database comes from whichever `starter-gorm-*`
dialect starter you import.

**Verify** (mirror of the example smoke):

```bash
cd demo && CGO_ENABLED=1 go run .
# create + audit OK: 4 rows, createdBy=alice, timestamps set
# findByID / existsByID OK
# composite condition + sort OK: [Cate Bob Dan]
# paging OK: window=[Dan Bob] total=3 hasNext=true
# save OK: updatedAt refreshed, createdBy preserved
# delete OK: 3 rows remain
```

Or run the shipped example directly:
`cd starter/experimental/starter-repository-gorm/example && ./check.sh` (exits non-zero on the
first deviation; `example/example.go:143-245` is the assertion list).

---

## 2. Assembly & timing

There is no lifecycle to time: `For` is a plain constructor. The interesting sequence is what
happens **per call**, because the generic wrapper layers store-neutral concerns above the gorm
backend (`repository.go:150-162`):

```
For[T, ID](db, "people", opts...)
  ├─ resolvePrimaryKey[T](db)          parse T's gorm schema → PK column (fallback "id")
  └─ repository.New(backend, opts...)  wraps backend; panics on nil backend (wiring-time)

repo.Create(ctx, entity)
  ├─ applyCreateAudit                  SetCreatedAt/SetUpdatedAt/SetCreatedBy  (audit.go:55-65)
  └─ backend.Create                    INSERT ... (audit already applied)

repo.Save(ctx, entity)
  ├─ applyUpdateAudit                  SetUpdatedAt only; created-by/at immutable (audit.go:70-74)
  └─ backend.Save                      gorm Save = upsert

repo.FindPage(ctx, q)
  ├─ backend.FindAll(q)                filters → sort → offset/limit
  └─ backend.CountBy(q)                filters only — Total ignores sort/window by design
```

Design reason (from source comments): audit lives in the generic layer so the same entity
carries correct timestamps whether persisted to SQL, Mongo or an in-memory store
(`repository.go:42-44`); `CountBy` ignores sort and window because a paginator needs the
count of all rows the *filters* match (`backend.go:113-115`).

When registered via `gs.Provide`, the repository bean instantiates lazily like any Provide
bean — export it or have it injected, or (in prod) it is never built.

## 3. API reference

### 3.1 Factory

```go
func For[T any, ID comparable](db *gorm.DB, table string, opts ...repository.Option) repository.Repository[T, ID]
```

- `resolvePrimaryKey` parses T's gorm schema and takes `PrioritizedPrimaryField.DBName`,
  falling back to `"id"` — so `FindByID`/`ExistsByID`/`Delete` build an explicit
  `WHERE <pk> = ?` even under an overridden table name (`backend.go:47-59`).
- Returned value is safe for concurrent use exactly as the underlying `*gorm.DB` is.
- Options: `repository.WithPrincipal(fn PrincipalFunc)` (fills CreatedBy from ctx; without it
  CreatedBy is left empty but timestamps still populate), `repository.WithClock(Clock)`
  (deterministic tests; production uses `time.Now`).

### 3.2 Repository methods (what you get)

| Method | Behavior |
|--------|----------|
| `Create(ctx, *T) error` | INSERT; fills all three audit fields first; pointer arg lets gorm write back generated keys |
| `Save(ctx, *T) error` | upsert; refreshes UpdatedAt only |
| `FindByID(ctx, ID) (T, bool, error)` | miss = `found=false, err=nil`; uses explicit PK WHERE + `Take` |
| `ExistsByID(ctx, ID) (bool, error)` | `Count ... Limit 1`, no materialisation |
| `Delete(ctx, ID) error` | deleting an absent id is **not** an error |
| `Count(ctx) (int64, error)` | total rows (CountBy over empty Query) |
| `FindAll(ctx, Query) ([]T, error)` | zero Query = every row; no total computed |
| `FindPage(ctx, Query) (Page[T], error)` | windowed items + filter-matching Total; `Page.HasNext()` false when unbounded |

Query building is fluent: `repository.NewQuery().Where(field, op, value).OrderBy(f) /
.OrderByDesc(f).Slice(offset, limit)`. Operators: `Eq Ne Gt Ge Lt Le In Like` — `In` binds a
slice value; `Like` expects the caller's value to already carry wildcards (`backend.go:143-167`).

### 3.3 Identifier validation (what is rejected, when)

Every field name that reaches SQL text — a `Cond.Field` or an `Order.Field` — must match
`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$` (`backend.go:34`); a violation fails the
call with `repository-gorm: invalid filter field %q` / `invalid sort field %q` **before any
query runs**. Field names come from developer code, not end-user input — the check is cheap
insurance; **values always ride gorm's `?` parameter binding** and are never interpolated.
An unsupported `Op` is likewise rejected (`unsupported operator`). Table names are passed to
gorm's `Table()` as-is — validate them yourself if they are dynamic.

### 3.4 Extension points

- **`repository.Backend[T, ID]`** — the single seam a second store implements; `New` layers
  audit + paging on top. Not a driver registry: a backend is a bean-type choice.
- **`repository.Auditable`** — per-entity opt-in; discard a value in a setter to not track
  that field.
- **`PrincipalFunc`** — wire to the same subject the security layer puts on the context.
- Maturity note from the abstraction: only this gorm backend exists today; the multi-store
  contract is unverified against a second store.

## 4. Verification & fault drills

1. **Audit population**: create with `withUser(ctx, "alice")`, assert `CreatedBy == "alice"`
   and `CreatedAt` non-zero (example does exactly this, `example/example.go:159-163`).
2. **Immutability of create audit**: `Save` an existing row, reload, assert `CreatedBy`
   unchanged and `UpdatedAt` advanced (example `example.go:216-229`).
3. **Paging contract**: `Where("age", Ge, 30).OrderBy("age").Slice(0, 2)` over 3 matches →
   2 items, `Total == 3`, `HasNext() == true`.
4. **Injection guard (negative)**: `repo.FindAll(ctx, repository.NewQuery().Where("age = 1; DROP TABLE people", repository.Eq, 1))`
   → error `invalid filter field`, table intact. Same for `OrderBy("name; DROP ...")`.
5. **PK fallback**: entity without a gorm primaryKey tag → `FindByID` queries `WHERE id = ?`
   (Spring Data convention).

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `invalid filter field` / `invalid sort field` | Field name fails the identifier regex (includes any attempted injection) | Use a bare column name; qualify as `table.column` if needed — the pattern allows one dot |
| `unsupported operator` | `repository.Op` value outside the closed set | Stick to Eq/Ne/Gt/Gt/Ge/Lt/Le/In/Like |
| `CreatedBy` stays empty | No `WithPrincipal`, or ctx carries no principal | Wire `WithPrincipal(currentUser)`; put the subject on the ctx the write uses |
| `UpdatedAt` not refreshed | Entity does not implement `Auditable` (all three setters required) | Implement the full interface |
| `FindByID` misses rows that exist | PK column resolved as `"id"` fallback while the real PK is named differently | Tag the field `gorm:"primaryKey"` so schema parsing finds it |
| Repository bean never constructed | Provide bean neither injected nor exported | `.Export(gs.As[gs.Rooter]())` or inject it (gs wires only root-reachable beans) |
| `LIKE` matches nothing | Value lacks wildcards — the backend adds none | Pass `"%bob%"` yourself |
| `FindPage` Total ≠ len(Items) | Working as designed — Total ignores the window | That is the contract; page with `HasNext()` |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 0 (verified: no `value:` tags in the module; `schema.json` empty) |
| Required | 0 |
| Quickstart external deps | 0 (in-memory sqlite) |
| "Watch out" entries | 4 |

Design suspects: `For` takes `*gorm.DB` while dialect starters publish the gormcore wrapper,
forcing a `.DB` unwrap at every call site (a gormcore-level overload would hide it); only one
backend exists so the `Backend` seam's multi-store contract is unverified; no gorm
soft-delete / hook / preload surface is reachable through the Repository (by scope — escape
hatch is the underlying `*gorm.DB`).

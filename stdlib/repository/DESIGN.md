# repository Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`repository` is the zero-dependency generic data-access abstraction in the
stdlib layer. It gives Go the *effect* of Spring Data's `CrudRepository` +
`PagingAndSortingRepository` — a ready-made set of persistence operations over a
domain type — reached with Go generics instead of proxy-generated method-name
parsing. A store (gorm, Mongo) is contributed by implementing the `Backend`
seam; the gorm one lives in `starter-repository-gorm`.

## 1. Responsibilities and Boundaries

- Give a domain type `T` (keyed by `ID`) CRUD, sorted/paged reads, and automatic
  audit fields through one generic `Repository[T, ID]` interface, so business
  code depends on an abstraction, not a specific store.
- Express reads as a data-store-neutral `Query` (filter/sort/window) — a simple
  Specification, not an expression language.
- Populate audit fields (`Auditable`) in a backend-neutral place, so timestamps
  and `CreatedBy` are correct regardless of store.
- Refuse to be an ORM, a query builder, or a method-name query parser. There is
  no derived-query magic and no relationship/lazy-loading model; complex or
  store-specific queries stay in the store's own client.

## 2. Key Abstractions and Seams

- **`Backend[T, ID]` interface as the store seam.** There is no global driver
  registry. A backend is bound to a live client (a `*gorm.DB`, a Mongo
  collection), so selecting one is a bean-type swap — the same choice
  `stdlib/batch`.`JobRepository` and `stdlib/lock` make. `Backend` is
  deliberately the same shape as `Repository` minus the store-neutral concerns,
  so an implementation is a thin `Query` translation and nothing more.
- **`New` layers the store-neutral concerns.** Auditing and `FindPage`
  composition (list + count) live above any backend, in `New`, so a new backend
  never re-implements them.
- **`Query` is a closed, small Specification.** `Op` is `Eq/Ne/Gt/Ge/Lt/Le/In/Like`
  — deliberately finite so every backend covers all of it, avoiding the
  partial-support trap of an open expression language. Fluent builders
  (`Where/OrderBy/Slice`) keep call sites readable.
- **`Page` carries an independent `Total`.** `FindPage` runs the backend's
  `FindAll` (windowed) and `CountBy` (filters only, no window) separately, so a
  paginator gets both the page and the full count in one call.
- **Audit `who` comes through `PrincipalFunc`.** The `CreatedBy` source is a
  context-reading seam, so it aligns with whatever the security layer put on the
  context, without this package importing security.

## 3. Constraints

- **A create fills all three audit fields; an update refreshes only
  `UpdatedAt`.** `CreatedAt`/`CreatedBy` are immutable after creation, so `Save`
  leaves them untouched.
- **`New` panics on a nil backend.** A nil backend can never serve a request;
  failing at wiring time is safer than on the first call.
- **`FindByID` miss is not an error.** It returns `found=false` with a nil
  error, so a lookup that finds nothing reads cleanly.
- **Field names are trusted developer input, values are always parameter-bound.**
  A `Cond.Field`/`Order.Field` comes from code, not end users; a backend still
  validates it as an identifier before interpolating, and every `Value` rides
  through the store's parameter binding.

## 4. Extending to another store

Implement `Backend[T, ID]` over the store's client and expose a `For` factory
that wraps it with `New`. The abstraction, the `Query` model, and audit handling
are untouched — a Mongo backend only writes the `Query`→filter-document
translation.

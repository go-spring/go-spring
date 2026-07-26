# validation Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`validation` is the zero-dependency stdlib abstraction for struct validation.
The recommended production driver (`go-playground/validator`) lives in
`starter-validation` and registers itself on blank import.

## 1. Responsibilities & Boundaries

- Answer "is this struct valid?" and, on failure, produce a **flat**
  `ValidationErrors` list of neutral `FieldError` values that any driver can
  emit.
- Provide two entry points: the config-binding one-shot (`Validate(ctx, name,
  v)`) and the Web request one (`Handle[T]`). Everything else is either
  driver-side or i18n-side.
- **Not** an i18n library. Message templates and localization stay out of
  this package — `Localize` takes a `func(key, args...) string`.

## 2. Key Abstractions & Seams

- `Validator` — `Validate(ctx, v) error`. Returning `nil` on success (not an
  empty `ValidationErrors`) lets callers keep the `err != nil` idiom.
- `Driver.NewValidator()` — factory registered in the package-level
  registry. `starter-validation` maps `validator.ValidationErrors` fields
  onto `FieldError` (tag→Rule, Namespace→Field, Param→Param).
- `FieldError.MessageKey()` — deterministic i18n key convention:
  `"validation." + Rule` (e.g. `validation.email`). Templates use positional
  args `{0}` = field name, `{1}` = param.
- `ValidationErrors.Localize(msg)` — the seam that keeps validation from
  importing i18n. Missing translations (`msg` returns `""`) fall back to
  `FieldError.Default()` so output is never blank.
- Web seam: `Handle[T](v, decode, render, next)` is the transport-level
  equivalent of `aspect.NewHandler` / `resilience.NewHandler`. `WriteError`
  is exported so adapters that do their own binding can reuse the exact
  400 body shape (`{"errors":[...]}`).
- `Decoder[T] = func(*http.Request, *T) error`. `JSONDecoder` is the
  default; gin/echo/hertz adapters can supply their own binder without
  losing the validation shell.

## 3. Constraints (do not break)

- **Driver returns `nil` on success, `ValidationErrors` on failure**. Any
  other error type from `Validate` is a driver bug — `WriteError` writes it
  as plain 400 text, which is a deliberately visible degradation.
- **`FieldError.Field` must be the struct-field path**, not the JSON tag.
  Struct path is the stable identifier; message rendering can rewrite it if
  needed.
- **`Localize` must never return a blank string**. When the message lookup
  yields `""` (missing key or nil lookup), fall back to `FieldError.Default`.
- **`Handle[T]`: nil validator = pass-through** (with decoding still done).
  The seam stays a no-op until a validator is wired.

## 4. Trade-offs / Alternatives Rejected

- **No i18n import**. Adding one would drag translation bundles into every
  downstream stdlib consumer. `Localize` accepts any lookup function; i18n
  is optional.
- **No struct-tag DSL of our own**. The driver owns the tags
  (`validator.v10` uses `validate:"..."`); stdlib only owns the neutral
  reporting shape.
- **`Validate` convenience is one-shot**. It resolves the driver, builds a
  validator and validates in one call — handy for the config path where a
  struct is validated once at startup, not a hot-loop primitive.
- **Registry mirrors `discovery.Register`** with panic-on-empty / nil /
  duplicate — duplicate wiring is a wiring bug, fail loudly at init.

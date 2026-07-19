# i18n Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`i18n` is the zero-dependency stdlib abstraction for resolving localized
messages. It backs both business text and `validation.ValidationErrors`
rendering without dragging any third-party i18n library into the foundation
layer.

## 1. Responsibilities & Boundaries

- Resolve `(ctx, key, args...) -> string`. The locale is taken from `ctx` via
  `LocaleFrom`, which downstream callers rarely wire explicitly — the
  application middleware sets it once from `Accept-Language`.
- Interpolate positional `{0}`, `{1}`, ... placeholders. Any placeholder whose
  index does not correspond to an arg is left intact so template drift is
  visible.
- Store templates. `MapSource` is the bundled backend; interfaces stay open so
  callers can plug alternate backends (database, remote config, ...) via
  their own `MessageSource` implementation.
- Explicitly refuses to read files. stdlib may not import `spring/conf`
  parsers (that would reverse the dependency direction), so `MapSource`
  accepts already-parsed maps and lets the outer layer choose the reader.

## 2. Key Abstractions & Seams

- `MessageSource` is the single interface — `Message(ctx, key, args...)`.
- `MapSource.AddParsed` accepts nested maps in the shape a yaml/json reader
  produces. **Both** `map[string]any` and `map[any]any` are flattened, because
  yaml.v2 yields the latter for nested maps; treating only one shape means
  entire subtrees collapse to a single leaf via `fmt.Sprint`.
- `Localizer(src, ctx)` curries a `MessageSource` to the
  `func(key, args...) string` shape `validation.ValidationErrors.Localize`
  requires. This keeps validation from importing i18n directly and lets any
  other lookup function plug in.
- `ErrMessageNotFound` is the sentinel wrapped by the "not found" error. When
  a caller wants graceful degradation, ignoring the error and using the
  returned string (which is the key) is safe; `Localizer` swallows the error
  and returns `""` so `Localize` falls back to `FieldError.Default()`.

## 3. Constraints (do not break)

- **Zero dependencies**. stdlib is the foundation layer; adding `x/text` or
  ICU would leak that dependency into every downstream consumer.
- **No file/URL reading here**. If it ever seems easy to just parse a
  `messages.yaml`, that reverses the dependency (stdlib → spring/conf). The
  parser lives at the wiring layer; `MapSource` only receives parsed maps.
- **Lookup order** is fixed: request-locale → default-locale → key. The
  default locale is the "safety net" for partially translated bundles; treat
  it as an invariant callers rely on.
- **Locale key type is unexported** (`localeKey struct{}`), so no accidental
  collision with keys from other packages.
- **Interpolator preserves unmatched placeholders**. Do not silently drop
  them; visibility is the diagnostic path.

## 4. Trade-offs / Alternatives Rejected

- **Not ICU MessageFormat**. Plural rules, ordinal handling, and
  gender-agreement templates are out of scope. Positional interpolation is
  enough for validation messages and typical business text.
- **No pluralization DSL**. If needed later, `MessageSource` can be extended
  through a second interface; the current one stays lean.
- **No auto-scan of message files at init**. That would need a parser and a
  file-system convention, both of which reverse the dependency direction.
  Wiring lives in the outer layer.

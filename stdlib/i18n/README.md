# i18n
[English](README.md) | [中文](README_CN.md)

`i18n` resolves localized messages: a `MessageSource` turns a key plus
arguments into a string in the caller's language. It serves user-facing
business text and the rendering of `validation.ValidationErrors`, with zero
third-party dependencies.

## Usage

### 1. Resolve messages per request locale

```go
src := i18n.NewMapSource(i18n.WithDefaultLocale("en")).
    AddMessage("en", "hello", "Hello, {0}!").
    AddMessage("zh", "hello", "你好, {0}!")

ctx := i18n.WithLocale(context.Background(), "zh")
msg, _ := src.Message(ctx, "hello", "Go-Spring")
fmt.Println(msg) // 你好, Go-Spring!
```

Lookup order is fixed: request locale → default locale → the key itself with
an error wrapping `ErrMessageNotFound`. Ignore the error for graceful
degradation, or `errors.Is` it to fail loud.

### 2. Feed bundles parsed elsewhere

The package reads no files — the parser belongs to the wiring layer (spring/conf
reader, remote config center). Hand the parsed map in; keys are whatever the
map carries (dot-joined names are a convention, not a requirement). `MapSource`
is the bundled in-memory backend: to serve messages from a database or config
center instead, implement the single-method `MessageSource` —
`Message(ctx, key, args...) (string, error)`.

```go
src.AddBundle("en", map[string]string{
    "validation.email": "{0} must be a valid email",
})
```

### 3. Pair with validation errors

`Localizer` produces exactly the lookup signature
`validation.ValidationErrors.Localize` consumes; on a missing key it returns
`""` so `Localize` falls back to `FieldError.Default()`:

```go
msgs := errs.Localize(i18n.Localizer(src, ctx))
```

## Design

- The locale travels on the context (`WithLocale` / `LocaleFrom`): middleware
  sets it once from `Accept-Language`, and every downstream `Message` call
  picks it up — the same way trace context flows.
- Positional interpolation `{0}`, `{1}`, ...; a placeholder with no matching
  arg is left intact, so template drift stays visible instead of silently
  disappearing.
- Not ICU MessageFormat: no plural/gender DSL. Positional interpolation covers
  validation messages and typical business text.
- Zero dependencies — the locale context key is unexported, so it cannot
  collide with keys from other packages.

## License

`i18n` is distributed under the Apache License 2.0. See [LICENSE](../../LICENSE) for details.

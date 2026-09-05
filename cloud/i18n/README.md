# i18n
[English](README.md) | [中文](README_CN.md)

`i18n` resolves localized messages: a `MessageSource` turns a key plus
arguments into a string in the caller's language. It serves user-facing
business text and the rendering of `validation.ValidationErrors`, with zero
third-party dependencies.

## The API

| API | What it does |
| --- | --- |
| `MessageSource` | The single interface: `Message(ctx, key, args...) (string, error)`. Implement your own for a database or config-center backend. |
| `WithLocale(ctx, locale)` / `LocaleFrom(ctx)` | The locale travels on the context — set it once in middleware from `Accept-Language`, every downstream call picks it up. |
| `NewMapSource(WithDefaultLocale)` | The bundled in-memory backend: locale → key → template. `AddMessage` registers one template, `AddBundle` registers a whole locale's bundle (chainable). |
| `Localizer(src, ctx)` | Curries a `MessageSource` into `func(key, args...) string` — the shape `validation.ValidationErrors.Localize` expects. |
| `ErrMessageNotFound` | Wrapped by the "key not found" error. Missing keys also yield the key itself as the string. |

## Usage

### 1. Resolve messages per request locale

```go
src := i18n.NewMapSource(WithDefaultLocale("en")).
    Add("en", "hello", "Hello, {0}!").
    Add("zh", "hello", "你好, {0}!")

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
map carries (dot-joined names are a convention, not a requirement):

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

A runnable end-to-end demo lives in [example/](example/).

## Rules the model guarantees

- Positional interpolation `{0}`, `{1}`, ...; a placeholder with no matching
  arg is left intact, so template drift stays visible instead of silently
  disappearing.
- Not ICU MessageFormat: no plural/gender DSL. Positional interpolation covers
  validation messages and typical business text.
- Zero dependencies — the locale context key is unexported, so it cannot
  collide with keys from other packages.

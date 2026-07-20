# i18n
[English](README.md) | [中文](README_CN.md)

`i18n` is a zero-dependency abstraction for localized messages. A
`MessageSource` resolves a key plus arguments into a string in the caller's
language; it backs both user-facing business text and the rendering of
`validation.ValidationErrors` without pulling any third-party i18n library
into the foundation layer.

## Features

- Zero third-party dependencies.
- Locale travels on `context.Context` (`WithLocale` / `LocaleFrom`), the same
  way trace context flows through a request.
- Bundled `MapSource` holds messages in memory keyed by locale then key.
- Fallback lookup order: request locale → default locale → the key itself
  paired with `ErrMessageNotFound` (fail-loud or graceful render is the
  caller's choice).
- Positional interpolation `{0}`, `{1}`, ... — unmatched placeholders are left
  intact so template mismatches surface instead of being silently dropped.
- Accepts already-parsed maps (`AddMap` for flat, `AddParsed` for nested);
  both `map[string]any` (json) and `map[any]any` (yaml.v2) nests are
  flattened with dot-joined keys.
- `Localizer(src, ctx)` produces the `func(key, args...) string` signature
  `validation.ValidationErrors.Localize` expects; missing keys yield `""`
  so the caller's default message is used.

## Quick Start

Import path: `go-spring.org/spring/i18n`.

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/spring/i18n"
)

func main() {
    src := i18n.NewMapSource("en").
        Add("en", "hello", "Hello, {0}!").
        Add("zh", "hello", "你好, {0}!")

    ctx := i18n.WithLocale(context.Background(), "zh")
    msg, _ := src.Message(ctx, "hello", "Go-Spring")
    fmt.Println(msg) // "你好, Go-Spring!"
}
```

For nested bundles loaded by an outer parser (`spring/conf` reader, remote
config-center provider, ...) hand the parsed map to `AddParsed`:

```go
src.AddParsed("en", map[string]any{
    "validation": map[string]any{
        "email": "{0} must be a valid email",
    },
})
```

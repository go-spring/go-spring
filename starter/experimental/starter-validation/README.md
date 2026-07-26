# starter-validation

[English](README.md) | [中文](README_CN.md)

`starter-validation` registers [go-playground/validator][gpv] as the `default`
driver for the abstraction defined in
[`spring/validation`](../../spring/validation). Blank-import it and every
`validation.Validate(ctx, "default", v)` call — whether from `conf.Bind`
post-validation or a Web request handler — validates structs tagged with
`validate:"..."`.

It follows the *global / infrastructure* archetype (see
[starter/DESIGN.md](../DESIGN.md) §2.4): it registers no bean and opens no
port. The third-party validator is pulled in here — never in `stdlib` — so
the foundation layer keeps its zero-dependency guarantee.

[gpv]: https://github.com/go-playground/validator

## Installation

```bash
go get go-spring.org/starter-validation
```

## Quick Start

### 1. Import the starter

```go
import _ "go-spring.org/starter-validation"
```

### 2. Tag your struct

Any tag `go-playground/validator` supports (`required`, `email`, `min=...`,
`oneof=a b c`, ...) works unchanged.

```go
type SignupRequest struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=18"`
}
```

### 3. Validate

```go
import "go-spring.org/spring/web/validation"

if err := validation.Validate(ctx, "default", &req); err != nil {
    // err is validation.ValidationErrors — one FieldError per failing rule
}
```

`ValidationErrors` is neutral: each entry carries `Field` (the struct
namespace), `Rule` (the validator tag, e.g. `email`), `Param` (the tag
parameter, e.g. `18` for `min=18`), and `Value` (the offending value). The
tag becomes the i18n key `validation.<tag>`.

## i18n

Combine with [`spring/i18n`](../../spring/i18n) to render localized messages
without hard-coding strings:

```properties
# messages_en.yaml
validation.required: "{{.Field}} is required"
validation.email:    "{{.Field}} must be a valid email"
validation.min:      "{{.Field}} must be at least {{.Param}}"
```

```go
lines := err.(validation.ValidationErrors).Localize(func(fe validation.FieldError) string {
    s, _ := src.Message(ctx, fe.MessageKey(), fe.Field, fe.Param)
    return s
})
```

See [`example/`](example) for a runnable demo exercising both a
`conf.Bind`-style config path and an HTTP handler path, with English and
Chinese message bundles.

## Non-tag errors

`InvalidValidationError` (e.g. calling `Validate` with a nil pointer) is a
programming error, not a field failure, so it is returned verbatim rather
than wrapped into `ValidationErrors`.

# validation
[English](README.md) | [中文](README_CN.md)

`validation` is a framework-agnostic, zero-dependency abstraction for struct
validation, split from its implementations the same way `spring/resilience`
and `spring/discovery` are. It answers "is this struct well-formed?" for both
configuration binding and inbound Web requests.

## Features

- Zero third-party dependencies in the abstraction.
- Neutral `FieldError{Field, Rule, Param, Value}` and `ValidationErrors` list
  every driver must produce.
- `Validator` interface + `Driver` factory + registry (`RegisterDriver` /
  `GetDriver` / `MustGetDriver`); `starter-validation` registers a
  `go-playground/validator` driver as `"default"` on blank import.
- `ValidationErrors.Localize(msg func(key, args...) string)` renders per-field
  messages through any lookup function — typically bound to `spring/i18n`
  with `i18n.Localizer(src, ctx)`. This package never imports i18n directly.
- Web seam: generic `Handle[T](v, decode, render, next)` returns an
  `http.Handler` that decodes, validates and rejects with structured JSON
  400 before business code runs. Default decoder is `JSONDecoder`.
- Convenience `Validate(ctx, name, v)` for the config-binding path (resolve
  driver, build validator, run once); reuse a `Validator` on hot paths.

## Quick Start

Import path: `go-spring.org/spring/validation`.

```go
package main

import (
    "context"
    "log"
    "net/http"

    "go-spring.org/spring/web/validation"
    _ "go-spring.org/starter/starter-validation" // registers "default" driver
)

type SignUp struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

func main() {
    d, _ := validation.MustGetDriver("default")
    v, _ := d.NewValidator()

    h := validation.Handle[SignUp](v, nil, nil, func(w http.ResponseWriter, r *http.Request, in *SignUp) {
        _, _ = w.Write([]byte("ok"))
    })
    log.Fatal(http.ListenAndServe(":8080", h))

    _ = context.Background // hint: pair with i18n.Localizer for localized errors
}
```

To render errors in the caller's language, pass a `render` that uses
`i18n.Localizer`:

```go
render := func(fe validation.FieldError) string {
    // fe.MessageKey() == "validation." + fe.Rule
    return i18n.Localizer(src, ctx)(fe.MessageKey(), fe.Field, fe.Param)
}
```

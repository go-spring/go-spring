# validation

[English](README.md) | [中文](README_CN.md)

`validation` is the neutral error model for struct validation: a flat
`ValidationErrors` list of `FieldError{Field, Rule, Param, Value}` values.
Run whichever validator you like (typically go-playground/validator); when it
fails, map its errors onto this shape so they render uniformly and localize.

## When to use it

- You validate with go-playground/validator (or any validator) and want the
  failures rendered as user-facing messages, possibly localized.
- Multiple parts of your app must report validation failures in one stable
  shape (field path + rule + param).

You don't need it if you only check `err != nil` and log the validator's own
error.

## The API

| API | What it does |
| --- | --- |
| `FieldError{Field, Rule, Param, Kind, Value}` | One failed rule. `Field` is the struct-field path (e.g. `User.Email`) — the stable identifier, not the JSON tag. `Kind` is the field's kind when the mapper knows it (`"string"`, `"int"`). |
| `FieldError.MessageKey()` | The i18n key convention: `"validation." + Rule` → `validation.email`. |
| `FieldError.Default()` | Plain English fallback message, used when no translation exists. |
| `ValidationErrors` | `[]FieldError` implementing `error` (joins the default messages). |
| `ValidationErrors.Localize(msg, opts...)` | Resolves each field through up to three steps, first non-empty wins: `WithCustom(cb)` → `msg(key, args...)` (`{0}` = field, `{1}` = param) → `FieldError.Default()`. Never returns a blank string. |

## Usage

### 1. Validate with your validator, map failures on error

```go
import (
    "github.com/go-playground/validator/v10"
    "go-spring.org/stdlib/validation"
)

v := validator.New(validator.WithRequiredStructEnabled())

err := v.Struct(&SignUp{Email: "not-an-email"})
verrs, ok := err.(validator.ValidationErrors)
if !ok {
    return err // a real error, not a field failure
}
out := make(validation.ValidationErrors, len(verrs))
for i, fe := range verrs {
    out[i] = validation.FieldError{
        Field: fe.Namespace(), // struct path → stable identifier
        Rule:  fe.Tag(),       // "email" → key "validation.email"
        Param: fe.Param(),     // "3" for min=3
        Kind:  fe.Kind().String(), // "string" / "int" / ...
    }
}
```

### 2. Render in the caller's language with stdlib/i18n

Prepare messages keyed by rule:

```go
src := i18n.NewMapSource(WithFallbackLocale("en")).
    Add("zh-CN", "validation.email", "{0} 不是合法邮箱").
    Add("zh-CN", "validation.min", "{0} 至少为 {1}")
```

Then localize per request (locale travels on the context):

```go
msgs := out.Localize(i18n.Localizer(src, ctx))
// []string{"SignUp.Email 不是合法邮箱", ...}
```

Missing translations fall back to `FieldError.Default()` — output is never
blank.

### 3. Customize: per-field phrasing and kind-dependent rules

`Localize`'s optional callback receives the whole `FieldError`. Return a string
to override, `""` to fall through to the generic template. Use it to give one
field dedicated phrasing, or to disambiguate rules that mean different things on
different types — `min` is a length on strings but a value on numbers:

```go
msgs := out.Localize(i18n.Localizer(src, ctx), validation.WithCustom(func(fe validation.FieldError) string {
    if fe.Field == "SignUp.Password" {
        return "密码至少 8 位"
    }
    if fe.Rule == "min" && fe.Kind == "string" {
        return fe.Field + " 长度至少为 " + fe.Param
    }
    return "" // generic template
}))
```

### 4. Or just use it as a plain error

`ValidationErrors` implements `error`, joining the default messages:

```go
return out // "validation: field \"SignUp.Email\" failed rule \"email\""
```

## Rules the model guarantees

- `Field` is the struct-field path, not the JSON tag; rendering can rewrite it.
- `Localize` never yields a blank string.
- This package imports nothing third-party — mapping from your validator's
  error type is always a few caller-side lines.

## License

`validation` is distributed under the Apache License 2.0. See [LICENSE](../../LICENSE) for details.

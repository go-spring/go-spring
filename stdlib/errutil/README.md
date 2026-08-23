# errutil

[English](README.md) | [中文](README_CN.md)

`errutil` is a lightweight utility package for structured error handling; every helper is a pure function over
Go's built-in `error` type. It provides two orthogonal wrapping verbs:
**`Explain`** adds human-readable meaning — *what* went wrong in business
terms, user-facing and semantic — while **`Stack`** adds call-path context —
*where* the error passed through, developer-facing and structural. It also
ships common sentinel errors and fail-fast precondition helpers; separating
interpretation (`:`) from trace path (`>>`) makes wrapping more expressive
than ad-hoc `fmt.Errorf` prefixes.

## Usage

### Explanatory wrapping

Use `Explain` to add semantic meaning to an existing error:

```go
err := errors.New("connection refused")
return errutil.Explain(err, "failed to connect to database")
// Output: "failed to connect to database: connection refused"
```

### Stack wrapping

Use `Stack` to add call-path context for debugging or tracing:

```go
err := errors.New("file not found")
return errutil.Stack(err, "LoadConfig")
// Output: "LoadConfig >> file not found"
```

### Combined usage

`Explain` and `Stack` can be combined — first add a semantic explanation,
then attach call-path context, preserving both meaning and trace:

```go
baseErr := errors.New("file not found")
baseErr = errutil.Explain(baseErr, "failed to load configuration")
err := errutil.Stack(baseErr, "InitService")
// Output: "InitService >> failed to load configuration: file not found"
```

### API

- `Explain(err, format, args...) error` — wraps with `":"`.
- `Stack(err, format, args...) error` — wraps with `" >> "`.
- `ErrForbiddenMethod`, `ErrUnimplementedMethod` — sentinel errors for the
  forbidden / not-implemented cases.

### Precondition helpers

- `Field` — a named string value passed to `RequireAny`; `Name` is the
  human-readable property name used in the error message, `Value` is the
  field's current value.
- `RequireField(component, field, value string) error` — returns a fail-fast
  error when a required value is empty (whitespace-only counts as empty):

```go
if err := errutil.RequireField("mail", "host", cfg.Host); err != nil {
    return nil, err
}
```

- `RequireAny(component string, fields ...Field) error` — returns a fail-fast
  error when every one of the alternative fields is empty, for cross-field
  rules like "addr OR service-name" that declarative per-field validation
  cannot express:

```go
if err := errutil.RequireAny("http-client",
    errutil.Field{Name: "addr", Value: cfg.Addr},
    errutil.Field{Name: "service-name", Value: cfg.ServiceName},
); err != nil {
    return nil, err
}
```

## Design

- **Two verbs, two separators.** `:` marks semantic interpretation, `>>` marks
  call-path propagation. Splitting the two removes the "is this prefix a cause
  or a location?" ambiguity that plagues single-verb wrapping.
- **`%w` everywhere, no custom error type.** Every wrapper uses
  `fmt.Errorf("... %w", err)` internally, so `errors.Is` / `errors.As` keep
  working across chains. The package deliberately defines no error type of its
  own; interoperability with the standard library is the point.
- **Uniform `nil`-in behaviour.** When the inner error is `nil`, both wrappers
  fall back to `fmt.Errorf(format, args...)`, so callers can write
  `errutil.Explain(nil, "reason: %s", x)` without a `nil` guard.
- **Deliberately not a stack-trace library.** `Stack` records a name for each
  step; it does not capture `runtime.Callers` frames. Anything richer belongs
  in a dedicated tracing package.

## License

`errutil` is distributed under the Apache License 2.0. See
[LICENSE](../../LICENSE) for details.

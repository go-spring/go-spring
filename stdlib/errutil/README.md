# errutil

[English](README.md) | [中文](README_CN.md)

`errutil` is a lightweight Go utility package for structured error handling.
It provides **two distinct semantic styles** for wrapping errors:

1. **Explanatory wrapping** — Adds human-readable meaning or interpretation to an error, clarifying *what* went wrong in
   business or logical terms.
2. **Stack wrapping** — Adds contextual call-path information, showing *where* in the call chain the error was
   propagated.

These two patterns serve different purposes:

* **Explanatory errors** are *user-facing* and semantic:
  e.g. `"failed to load configuration: file not found"`
* **Stack errors** are *developer-facing* and structural:
  e.g. `"InitService >> LoadConfig >> file not found"`

The goal is to make error wrapping more expressive by clearly separating **interpretation (`:`)** from **trace
path (`>>`)**.

## Usage

### Explanatory Wrapping

Use `Explain` to add semantic meaning to an existing error:

```go
err := errors.New("connection refused")
return errutil.Explain(err, "failed to connect to database")
// Output: "failed to connect to database: connection refused"
```

### Stack Wrapping

Use `Stack` to add call-path context for debugging or tracing:

```go
err := errors.New("file not found")
return errutil.Stack(err, "LoadConfig")
// Output: "LoadConfig >> file not found"
```

### Combined Usage

`Explain` and `Stack` can be combined — first add a semantic explanation,  
then attach call-path context:

```go
baseErr := errors.New("file not found")
baseErr = errutil.Explain(baseErr, "failed to load configuration")
err := errutil.Stack(baseErr, "InitService")
// Output: "InitService >> failed to load configuration: file not found"
````

This pattern preserves both semantic meaning and call-path trace,
making it ideal for large or layered systems.

## Public API

- `Explain(err, format, args...) error` — wraps with `":"`.
- `Stack(err, format, args...) error` — wraps with `" >> "`.
- `ErrForbiddenMethod`, `ErrUnimplementedMethod` — sentinel errors for the
  forbidden / not-implemented cases.

Both wrappers use `fmt.Errorf("... %w", err)`, so `errors.Is` / `errors.As`
keep working across chains.

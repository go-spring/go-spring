# funcutil
[English](README.md) | [中文](README_CN.md)

`funcutil` returns runtime metadata (file, line, name) about a Go function
value. Used by the container / aspect framework to build human-readable
diagnostics. Part of the zero-dependency `stdlib` layer.

## API

- `FuncName(fn any) string` — package-qualified function name, without the
  full module path prefix. Method values printed by the runtime as `T.m-fm`
  have the `-fm` suffix trimmed.
- `FileLine(fn any) (file string, line int, fnName string)` — the source
  location plus the cleaned-up name.

## Usage

```go
import "go-spring.org/stdlib/funcutil"

func Handle() {}

name := funcutil.FuncName(Handle)
file, line, _ := funcutil.FileLine(Handle)
```

`fn` must be a function or method value. Passing anything else will panic
inside `reflect`.

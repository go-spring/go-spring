# funcutil

[English](README.md) | [中文](README_CN.md)

`funcutil` returns runtime metadata (file, line, name) about a Go function
value, used by the container / aspect framework for human-readable diagnostics.

## Usage

```go
import "go-spring.org/stdlib/funcutil"

func Handle() {}

name := funcutil.FuncName(Handle)
file, line, _ := funcutil.FileLine(Handle)
```

`fn` must be a function or method value. Passing anything else will panic
inside `reflect`.

### API

- `FuncName(fn any) string` — package-qualified function name, without the
  full module path prefix. Method values printed by the runtime as `T.m-fm`
  have the exact `-fm` suffix trimmed; names that merely end in `-`, `f` or
  `m` are left intact.
- `FileLine(fn any) (file string, line int, fnName string)` — the source
  location plus the cleaned-up name.

## License

Apache License 2.0. See [LICENSE](../../LICENSE).

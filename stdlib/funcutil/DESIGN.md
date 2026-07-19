# funcutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `funcutil` is a two-function
wrapper over `reflect` + `runtime.FuncForPC`.

## 1. Responsibilities & Boundaries

- Extract file, line and cleaned-up function name from a function value —
  enough for the container / aspect layer to produce actionable error and
  log messages ("bean registered at file:line by funcX").
- Not a stack walker. `runtime.Callers` and friends belong elsewhere; this
  package always operates on a value passed in by the caller.

## 2. Design Notes

- The runtime prints method values as `T.m-fm`; `funcutil` strips the `-fm`
  suffix so the returned name is what humans would write. The trim uses
  `strings.TrimRight`, which drops any trailing `-`, `f`, or `m` characters;
  callers should not depend on the suffix being present.
- The last `/`-separated segment is preserved (`pkg.Fn`), the module path
  prefix is dropped. This is a display choice and can bite comparisons if a
  caller assumes uniqueness across packages with the same short name.
- No caching. Reflecting a `*runtime.Func` on every call is cheap enough for
  the current uses (registration time, diagnostics), and skipping the cache
  keeps this package a two-function file.

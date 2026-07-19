# errutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`errutil` is part of `stdlib`, the zero-dependency foundation layer of
Go-Spring. It carries no framework state; every helper is a pure function over
Go's built-in `error` type.

## 1. Responsibilities & Boundaries

- Provide two orthogonal wrapping verbs — `Explain` and `Stack` — so callers
  can encode intent explicitly instead of using ad-hoc `fmt.Errorf` prefixes.
- Ship a couple of common sentinel errors (`ErrForbiddenMethod`,
  `ErrUnimplementedMethod`) that other stdlib packages reuse.
- Deliberately NOT a stack-trace library. `Stack` records a name for each
  step; it does not capture runtime.Callers frames. Anything richer belongs in
  a dedicated tracing package.

## 2. Key Design Decisions

- Two verbs, two separators. `:` marks semantic interpretation, `>>` marks
  call-path propagation. Splitting the two removes the "is this a cause or a
  location?" ambiguity that plagues single-verb wrapping.
- Every wrapper uses `%w` internally, so the underlying error stays reachable
  via `errors.Is` / `errors.As`. The package does not define its own error
  type; interoperability with the standard library is the point.
- `nil`-in behaviour is uniform: when the inner error is `nil`, both helpers
  fall back to `fmt.Errorf(format, args...)`. This lets callers write
  `errutil.Explain(nil, "reason: %s", x)` without a `nil` guard.

## 3. Constraints

- No third-party imports. errutil sits at the very bottom of the stdlib layer
  and is imported by most other stdlib packages.
- Format strings follow `fmt` semantics; passing a `%w` verb explicitly would
  wrap twice — callers should not.

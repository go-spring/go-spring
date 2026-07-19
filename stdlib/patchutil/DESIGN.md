# patchutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `patchutil` is a single escape
hatch — everything about it is a trade-off between "the alternative is
worse" and "please do not use this widely".

## 1. Responsibilities & Boundaries

- Provide **one** unsafe primitive: clear the read-only flag on a
  `reflect.Value` so assignment to unexported fields becomes possible.
- Not a reflection library. Every other reflection operation lives in
  `stdlib/typeutil` or in the standard `reflect` package.
- Not a general-purpose "modify anything" helper — only the RO flag is
  cleared; addressability, kind, and other invariants remain the caller's
  responsibility.

## 2. Constraints

- Depends on the exact memory layout of `reflect.Value` and the values of
  the private `flagStickyRO` / `flagEmbedRO` bits. This is stable across
  Go releases in practice, but is not covered by the Go 1 compatibility
  promise.
- Uses `unsafe.Pointer`. Requires `-gcflags=all=-d=checkptr=0` to be off (it
  is by default) and does not currently work under strict `unsafeptr`
  analyzers.
- Not thread-safe in any meaningful sense — the returned `Value` shares the
  underlying flag word with the input.

## 3. Trade-offs

- The whole package could be avoided by exporting the fields callers need.
  It exists because framework-internal seams (e.g. binding into
  legacy structs during tests) sometimes cannot change the target type. That
  scope is small and audit-friendly, which is why the file is only ~40 lines.

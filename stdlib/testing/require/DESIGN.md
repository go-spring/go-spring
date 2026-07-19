# require Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`require` is a **thin wrapper** over the shared assertion engine in
`stdlib/testing/internal`, differing from [`assert`](../assert/) only in
that a failing check calls `t.Fatalf` instead of `t.Errorf`. It exists so
the fail-fast choice is visible at the import statement.

## 1. Responsibilities and Boundaries

- Expose the same fluent API as `assert` with `fatalOnFailure = true`.
- Refuse to hold assertion logic of its own; every check lives in
  `internal` and is shared with `assert`. Splitting by wrapper guarantees
  the two modes cannot diverge in checks.

## 2. Key Abstractions and Seams

- **Const `fatalOnFailure = true`** — the one line that makes this package
  "require" instead of "assert". Every entry point passes it through.
- **`internal.TestingT`** — the minimal `*testing.T` surface accepted by
  every entry point.
- **Fluent value objects** (`*internal.Assertion`, ...) — identical to
  `assert`'s, so the two modes are drop-in from a signature perspective.

## 3. Constraints

- **No dependency beyond `internal`.**
- **Behavioural parity with `assert`.** The `testcase` suite runs the same
  scenarios through both entry points to enforce this.
- **`t.Fatalf` stops the current test only.** A `t.Run` subtest that fails
  through `require` stops that subtest; a parent test can decide whether
  to continue.

## 4. Trade-offs and Alternatives Rejected

- **Two thin packages over a runtime flag.** Making the fail-fast mode a
  compile-time choice keeps the reader from having to trace whether some
  earlier `SetMode(...)` call flipped the behaviour.
- **Own the API rather than depend on testify.** Kept the API surface close
  for muscle memory; the implementation is stdlib-only.

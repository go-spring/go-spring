# assert Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`assert` is a **thin wrapper** over the shared assertion engine in
`stdlib/testing/internal`. It exists to make the "fail-continue" mode a
compile-time choice at the import statement rather than a runtime flag on
every call.

## 1. Responsibilities and Boundaries

- Expose the same fluent API as [`require`](../require/) but with
  `fatalOnFailure = false` — so a failing assertion calls `t.Errorf`
  (continue) rather than `t.Fatalf` (stop).
- Refuse to hold assertion logic of its own; every check lives in
  `internal` and is shared with `require`. Two identical wrappers guarantee
  the two modes cannot diverge in behaviour.

## 2. Key Abstractions and Seams

- **Const `fatalOnFailure = false`** — the one line that makes this package
  "assert" instead of "require". Every entry point passes it through to the
  internal constructor.
- **`internal.TestingT`** — the minimal `*testing.T` surface. Accepting the
  interface lets the same assertion run under a real test, a subtest, or the
  test harness the `testcase` suite uses to introspect failures.
- **Fluent value objects** (`*internal.Assertion`, `*internal.ErrorAssertion`,
  ...) are returned by the entry points; check methods chain off them and
  ultimately call the shared engine with the mode's `fatalOnFailure`.

## 3. Constraints

- **No dependency beyond `internal`.** Any new dependency would ripple into
  every module's test binary.
- **Behavioural parity with `require`.** A method that exists here must
  exist there with the same signature; the `testcase` suite runs the same
  scenarios through both to enforce it.
- **`Panic` is a top-level function**, not a fluent chain — the target is a
  callback, not a value.

## 4. Trade-offs and Alternatives Rejected

- **Two thin packages over one package + a runtime flag.** A flag would let
  a test accidentally mix modes in one function; splitting by import makes
  the choice visible at every call site.
- **Own the API rather than depend on testify.** The API is intentionally
  close for muscle memory, but the implementation is stdlib-only.

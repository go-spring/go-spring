# testcase Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`testcase` is the guardrail against `assert` and `require` diverging. It is a
test-only package — `package testcase_test` — that runs the shared assertion
engine through both mode wrappers so any silent behavioural drift shows up
immediately.

## 1. Responsibilities and Boundaries

- Exercise every assertion family (`That`, `Error`, `Number`, `String`,
  `Slice`, `Map`, `Panic`) through **both** `assert` and `require`.
- Verify that the same input yields the same message and the same overall
  fluent shape from both entry points, differing only in whether the test
  stops.
- Refuse to hold production code. Nothing is exported; there is no `.go`
  file, only `*_test.go` files.

## 2. Key Abstractions and Seams

- **`internal.TestingT` fake** — the suite records failure output rather
  than actually failing the outer test, so it can assert on the *content* of
  a failure message.
- **One test file per assertion family** — matches the `internal/*.go`
  breakdown so a change in one family maps to one test file.

## 3. Constraints

- **No exported symbols.** The suite is discovered only by `go test`. A tool
  that scans the module for public API sees nothing here.
- **Runs against `stdlib/testing/internal` and both wrappers only.** Any
  third-party dependency would leak into the wrapper packages via
  build-time coupling.

## 4. Trade-offs and Alternatives Rejected

- **One shared suite over per-package duplication.** Duplicating the test
  bodies in `assert` and `require` would drift as the two modes are
  maintained separately. Running one suite through both entry points forces
  parity.
- **Test-only package, not a helper library.** Making the fake `TestingT`
  and the scenario tables public would tempt callers to embed them in their
  own tests, which the two-mode contract is not designed for.

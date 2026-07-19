# testcase
[English](README.md) | [中文](README_CN.md)

`testcase` is the shared assertion test suite that exercises the checks in
`stdlib/testing/internal` through **both** [`assert`](../assert/) and
[`require`](../require/). It is a test-only package (`package testcase_test`)
and exports no code. If you are looking for assertion helpers, use `assert`
or `require`; this package exists so their behaviour cannot drift apart.

## Purpose

Every check — equality, error matching, number comparisons, string matchers,
slice / map operations, panic detection — lives in one place (`internal`). By
running the same scenarios through both `assert` (fail-continue) and
`require` (fail-fast), the suite guarantees:

- Both entry points expose the same methods with the same signatures.
- Both entry points report failures with the same message shape.
- Only the "does the test stop?" behaviour differs.

## Layout

Six files, one per assertion family:

| File | Covers |
|------|--------|
| `assert_test.go` | Generic `That` and `Panic` |
| `error_test.go`  | `Error` (`Is`, `Matches`, `String`, ...) |
| `number_test.go` | `Number[T]` |
| `string_test.go` | `String` |
| `slice_test.go`  | `Slice[T]` |
| `map_test.go`    | `Map[K,V]` |

## Running the suite

```
go test ./stdlib/testing/...
```

There is no public API to import.

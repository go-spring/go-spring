# iterutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `iterutil` is a set of callback-
driven loops that give `defer` per-iteration semantics.

## 1. Responsibilities & Boundaries

- Provide `Times`, `Ranges`, `StepRanges` — each hands the loop body to a
  callback so `defer` inside the body fires when that callback returns,
  not when the enclosing function returns.
- Not a full iteration DSL. Go's own `for` loop is still the tool for the
  vast majority of loops; use these helpers only when per-iteration cleanup
  matters.

## 2. Design Notes

- Direction is inferred from the arguments. `Ranges(2, 5, fn)` counts up,
  `Ranges(5, 2, fn)` counts down. This removes a Boolean parameter at the
  cost of "no-op" when `start == end`.
- `StepRanges` requires the sign of `step` to match the direction of the
  range; mismatched inputs produce no calls (rather than an infinite loop).
- No `error`-returning variant. Callers that need early exit should use a
  plain `for` loop.

# mathutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `mathutil` currently holds only
the overflow checks that JSON / form binding need.

## 1. Responsibilities & Boundaries

- Answer "does this `int64` / `uint64` / `float64` fit `T`?" without the
  caller pulling in `math` and repeating a switch per type.
- Not a general-purpose numeric library. Bounded / saturating conversions
  are not offered; callers get a bool and pick their own error behaviour.

## 2. Design Notes

- Type dispatch uses `switch any(z).(type)` on the zero value of `T`.
  Compile-time dispatch would require a per-type function set, which pushes
  the switch out to every call site — the current shape is tolerated because
  the check is only used at decode boundaries.
- `OverflowUint` never checks `uint64` and never inspects negativity — the
  parameter is already `uint64`. `OverflowInt[int64]` is likewise a no-op.
  This is intentional: the callers pass `int64` / `uint64` produced by
  `strconv`, and the "no truncation needed" case must be cheap.
- `OverflowFloat[float64]` always returns `false`; `OverflowFloat[float32]`
  compares against `±math.MaxFloat32`. Subnormals and NaN are not treated as
  overflow.

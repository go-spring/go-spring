# listutil Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `listutil` is a very thin
generic wrapper around `container/list` plus a handful of slice/writer
helpers.

## 1. Responsibilities & Boundaries

- Restore compile-time typing to `container/list` without reimplementing the
  data structure. Every method is a one-liner over the embedded stdlib type.
- Provide small helpers (`SliceOf`, `ListOf`, `AllOfList`, `WriteStrings`)
  that show up repeatedly in framework code.
- Not a linked-list rewrite, not a functional collections library.

## 2. Design Notes

- `Element[T]` embeds `*list.Element` by pointer so `Valid()` can safely
  compare it to `nil`. The zero value of `Element[T]` is a valid "nil"
  marker for iteration end.
- `AllOfList` uses `e.Value.(T)` and will panic if the caller mixes types.
  This is a deliberate trade-off — a type-checked variant would need an
  `ok, err` shape that most callers do not want.
- The generic wrapper does **not** protect against passing an `Element[T]`
  from a foreign list — `container/list` itself panics in that case and the
  wrapper does not add a check.

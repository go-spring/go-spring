# ordered Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

Part of the zero-dependency `stdlib` layer. `ordered` is intentionally a
single-function file today; it exists as a named location where any future
"deterministic iteration order" helpers can accumulate.

## 1. Responsibilities & Boundaries

- Give the framework a one-call way to iterate a map in a stable order,
  without every caller allocating a slice + `sort.Strings`.
- Not an ordered-map data structure. There is no insertion-order container
  here; Go's built-in map plus this helper is enough for the current use
  cases.

## 2. Design Notes

- Uses the `cmp.Ordered` constraint (Go 1.21+) so numeric and string keys
  alike are covered without duplicated helpers.
- The returned slice is a copy; the caller may mutate it freely.
- Uses `slices.Sort`, not `sort.Strings`, to keep the implementation generic.

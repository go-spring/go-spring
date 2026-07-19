# mathutil
[English](README.md) | [中文](README_CN.md)

`mathutil` provides generic overflow checks used when narrowing an
`int64` / `uint64` / `float64` into a smaller numeric type. Part of
Go-Spring's zero-dependency `stdlib` layer.

## API

- `OverflowInt[T ~int|~int8|~int16|~int32|~int64](v int64) bool`
- `OverflowUint[T ~uint|~uint8|~uint16|~uint32|~uint64](v uint64) bool`
- `OverflowFloat[T ~float32|~float64](v float64) bool`

Each returns `true` when the value cannot be represented in `T`.

## Usage

```go
import "go-spring.org/stdlib/mathutil"

if mathutil.OverflowInt[int16](v) {
    return errors.New("out of range")
}
```

Used internally by `stdlib/formutil` and `stdlib/jsonflow` when decoding
numbers into narrower target types.

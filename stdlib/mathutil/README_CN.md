# mathutil
[English](README.md) | [中文](README_CN.md)

`mathutil` 提供泛型的溢出检查，用于把 `int64` / `uint64` / `float64` 收敛到
更窄的数值类型时判断是否越界。属于 Go-Spring 零依赖的 `stdlib` 层。

## API

- `OverflowInt[T ~int|~int8|~int16|~int32|~int64](v int64) bool`
- `OverflowUint[T ~uint|~uint8|~uint16|~uint32|~uint64](v uint64) bool`
- `OverflowFloat[T ~float32|~float64](v float64) bool`

值无法安全转换到 `T` 时返回 `true`。

## 用法

```go
import "go-spring.org/stdlib/mathutil"

if mathutil.OverflowInt[int16](v) {
    return errors.New("out of range")
}
```

被 `stdlib/formutil` 与 `stdlib/jsonflow` 用于把数值解码到更窄目标类型时
做范围校验。

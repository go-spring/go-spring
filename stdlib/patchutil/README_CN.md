# patchutil
[English](README.md) | [中文](README_CN.md)

`patchutil` 提供一个反射工具，用于清掉 `reflect.Value` 内部的 read-only 标记，
从而给未导出字段赋值。**仅供框架内部工具与测试使用**。属于 Go-Spring 零依赖的
`stdlib` 层。

## API

- `PatchValue(v reflect.Value) reflect.Value` —— 返回同一个 `Value`，
  但已清掉 `flagRO` 位，之后 `Set` 即便原本指向未导出字段也能成功。

## 用法

```go
import (
    "reflect"
    "go-spring.org/stdlib/patchutil"
)

f := patchutil.PatchValue(reflect.ValueOf(&obj).Elem().FieldByName("secret"))
f.SetString("new value")
```

## 警告

- 使用 `unsafe` 修改 `reflect.Value` 的内部 `flag` 字段。
- 版本敏感，未来 Go 升级可能失效。
- 生产业务代码请**不要**使用；仅在框架内部工具、测试等替代成本更高的场景下
  使用。

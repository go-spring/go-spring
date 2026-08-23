# patchutil

[English](README.md) | [中文](README_CN.md)

`patchutil` 提供一个反射工具，用于清掉 `reflect.Value` 内部的 read-only
标记，从而给未导出字段赋值。它为框架内部无法修改目标类型的接缝处而生，
**仅供框架内部工具与测试使用**。

## 使用方式

```go
import (
    "reflect"
    "go-spring.org/stdlib/patchutil"
)

f := patchutil.PatchValue(reflect.ValueOf(&obj).Elem().FieldByName("secret"))
f.SetString("new value")
```

### API 列表

- `PatchValue(v reflect.Value) reflect.Value` —— 返回同一个 `Value`，
  但已清掉 `flagRO` 位，之后 `Set` 即便原本指向未导出字段也能成功。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。

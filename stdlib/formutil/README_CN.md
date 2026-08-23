# formutil

[English](README.md) | [中文](README_CN.md)

`formutil` 提供 Go 值与表单键值（`url.Values`、`[]string`）之间的泛型编解码
工具，供 Go-Spring 的 HTTP 客户端 / 服务端绑定代码使用。

## 使用方式

```go
import (
    "net/url"
    "go-spring.org/stdlib/formutil"
)

// 解码单值
n, err := formutil.DecodeInt[int]("page", []string{"3"})

// 解码重复字段
ids, err := formutil.DecodeList("ids",
    []string{"1", "2", "3"}, formutil.DecodeInt[int64])

// 编码到 url.Values
v := url.Values{}
_ = formutil.EncodeString(v, "name", "alice")
_ = formutil.EncodeIntPtr[int64](v, "opt", nil) // 缺省
```

### API 列表

#### 解码（`[]string` → Go 值）

| 函数 | 说明 |
|---|---|
| `DecodeBool(key string, values []string) (bool, error)` | 解码 `bool` |
| `DecodeBoolPtr(key string, values []string) (*bool, error)` | 解码为 `*bool` |
| `DecodeInt[T ~int64\|~int32\|~int16\|~int8\|~int](key, values) (T, error)` | 解码有符号整数，溢出检查 |
| `DecodeIntPtr[...](...) (*T, error)` | 解码为 `*T` |
| `DecodeUint[T ~uint64\|~uint32\|~uint16\|~uint8\|~uint](...) (T, error)` | 解码无符号整数，溢出检查 |
| `DecodeUintPtr[...](...) (*T, error)` | 解码为 `*T` |
| `DecodeFloat[T ~float64\|~float32](...) (T, error)` | 解码浮点数，溢出检查 |
| `DecodeFloatPtr[...](...) (*T, error)` | 解码为 `*T` |
| `DecodeString(key, values) (string, error)` | 解码 `string` |
| `DecodeStringPtr(key, values) (*string, error)` | 解码为 `*string` |
| `DecodeBytes(key, values) ([]byte, error)` | 解码字节切片（标准 base64） |
| `DecodeJSON[T any](key, values) (T, error)` | 解码任意 JSON（委托 `stdlib/jsonflow`） |
| `DecodeList[T any](key string, values []string, fn Decoder[T]) ([]T, error)` | 用 `fn` 逐个解码重复字段 |

#### 编码（Go 值 → `url.Values`）

| 函数 | 说明 |
|---|---|
| `EncodeBool(m url.Values, key string, val bool) error` | 编码 `bool` |
| `EncodeBoolPtr(m, key, val *bool) error` | `nil` 时省略该字段 |
| `EncodeInt[T ~int64\|~int32\|~int16\|~int8\|~int](m, key, val T) error` | 编码有符号整数 |
| `EncodeIntPtr[...](m, key, val *T) error` | `nil` 时省略该字段 |
| `EncodeUint[T ~uint64\|~uint32\|~uint16\|~uint8\|~uint](m, key, val T) error` | 编码无符号整数 |
| `EncodeUintPtr[...](m, key, val *T) error` | `nil` 时省略该字段 |
| `EncodeFloat[T ~float64\|~float32](m, key, val T) error` | 编码浮点数 |
| `EncodeFloatPtr[...](m, key, val *T) error` | `nil` 时省略该字段 |
| `EncodeString(m, key, val string) error` | 编码 `string` |
| `EncodeStringPtr(m, key, val *string) error` | `nil` 时省略该字段 |
| `EncodeBytes(m, key, val []byte) error` | 编码字节切片（标准 base64） |
| `EncodeJSON[T any](m, key, val T) error` | 编码任意 JSON（委托 `stdlib/jsonflow`） |
| `EncodeList[T any](m, key string, values []T, fn Encoder[T]) error` | 用 `fn` 逐个编码切片 |

### 规则

- 所有非列表 Decoder 遇到多个原始值时会报错（"too many values for form
  field ..."），遇到空值列表时同样报错（"missing value for form field ..."）。
- 整数 / 无符号 / 浮点解码在结果不适合目标 `T` 时返回溢出错误。
- `DecodeBytes` / `EncodeBytes` 使用标准 base64。
- `EncodeXxxPtr` 在指针为 `nil` 时完全省略该字段。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。

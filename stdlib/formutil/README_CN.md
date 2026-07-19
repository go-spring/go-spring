# formutil
[English](README.md) | [中文](README_CN.md)

`formutil` 提供 Go 值与表单键值（`url.Values`、`[]string`）之间的泛型编解码
工具，供 Go-Spring 的 HTTP 客户端 / 服务端绑定代码使用。属于零依赖的
`stdlib` 层。

## 功能

- 对 `bool`、有 / 无符号整数、浮点数、`string`、字节切片以及任意 JSON 提供
  对称的 `Decode<Type>` / `Encode<Type>` 对。
- 每种类型都配套 `<Type>Ptr` 变体：编码时 `nil` 表示"字段缺省"，解码时返回
  `*T`。
- 通过泛型 `DecodeList` / `EncodeList` 处理重复字段。
- 整数 / 浮点数解码借助 `stdlib/mathutil` 做溢出检查。
- JSON 编解码委托给 `stdlib/jsonflow`。

## 用法

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

## 规则

- 所有非列表 Decoder 遇到多个原始值时会报错（"too many values for form
  field ..."）。
- 整数 / 无符号 / 浮点解码在结果不适合目标 `T` 时返回溢出错误。
- `DecodeBytes` / `EncodeBytes` 使用标准 base64。
- `EncodeXxxPtr` 在指针为 `nil` 时完全省略该字段。

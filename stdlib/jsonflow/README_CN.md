# jsonflow
[English](README.md) | [中文](README_CN.md)

`jsonflow` 是 Go-Spring 的流式 JSON 层。它构建在 Go 1.26 的
`encoding/json/v2` + `encoding/json/jsontext` 之上，提供：

- 带合理默认值的 `Marshal` / `Unmarshal` / `MarshalWrite` /
  `UnmarshalRead` API（map key 稳定排序、nil map / slice 输出为 `null`）。
- 一组泛型 `Encode<T>` / `Decode<T>` 工具，用于手写流式编解码、代码生成
  以及自定义 `JSONEncoder` / `JSONDecoder` 实现。

属于零依赖的 `stdlib` 层（依赖 `encoding/json/v2` 与 stdlib 内部包）。

## 顶层 API

- `Marshal(v, opts...) ([]byte, error)`
- `MarshalIndent(v, prefix, indent string) ([]byte, error)`
- `MarshalWrite(w io.Writer, v, opts...) error`
- `Unmarshal(b []byte, v) error`
- `UnmarshalRead(r io.Reader, v) error`

若 `v` 实现 `JSONEncoder` / `JSONDecoder`，则优先走这两个接口；否则走标准
`encoding/json/v2` 路径。

### 选项

选项实现被密封的 `MarshalOptions` 接口。内置选项：

- `Indent` / `IndentPrefix` —— 美化输出的缩进。
- `NilSliceAsNull` / `NilMapAsNull` —— nil 集合是否输出为 `null`
  （默认 `true`）。
- `Deterministic` —— map key 稳定排序（默认 `true`）。

## 流式工具

对于实现 `JSONEncoder` / `JSONDecoder` 的值，`jsonflow` 提供逐值 / 结构化
工具：

Encoders（`Encoder = json.Encoder`）：

- `EncodeNull`、`EncodeBool[T]`、`EncodeInt[T]`、`EncodeUint[T]`、
  `EncodeFloat[T]`、`EncodeString[T]`、`EncodeBytes`（base64）、
  `EncodeAny[T]`、`EncodeObject`。
- `EncodeArrayBegin` / `EncodeArrayEnd` / `EncodeArray` 与
  `EncodeObjectBegin` / `EncodeObjectEnd` / `EncodeMap`。
- Map key 工具：`EncodeIntKey`、`EncodeUintKey`、`EncodeStringKey`。
- 每个标量都有 `Ptr` 变体：nil 指针输出 `null`。

Decoders（`Decoder = json.Decoder`）：

- `DecodeBool`、`DecodeInt[T]`、`DecodeUint[T]`、`DecodeFloat[T]`、
  `DecodeString`、`DecodeBytes`（base64）、`DecodeAny[T]`、`DecodeObject`。
- `DecodeArray`、`DecodeMap` 为高阶组合子。
- `DecodeObjectBegin` / `DecodeObjectEnd` / `DecodeEOF` 用于框架化。
- 对应的 `Parse*` 变体供自定义 `parseFn` 回调复用。
- 每个标量都有 `Ptr` 变体：JSON 为 `null` 时返回 `nil`。

## 示例

```go
import "go-spring.org/stdlib/jsonflow"

type User struct {
    Name string
    Age  int
}

func (u *User) EncodeJSON(e jsonflow.Encoder) error {
    if err := jsonflow.EncodeObjectBegin(e); err != nil { return err }
    if err := jsonflow.EncodeStringKey(e, "name"); err != nil { return err }
    if err := jsonflow.EncodeString(e, u.Name); err != nil { return err }
    if err := jsonflow.EncodeStringKey(e, "age"); err != nil { return err }
    if err := jsonflow.EncodeInt(e, u.Age); err != nil { return err }
    return jsonflow.EncodeObjectEnd(e)
}

b, _ := jsonflow.Marshal(&User{Name: "alice", Age: 30})
```

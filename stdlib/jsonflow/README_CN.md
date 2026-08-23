# jsonflow

[English](README.md) | [中文](README_CN.md)

`jsonflow` 是 Go-Spring 的流式 JSON 层，构建在 Go 1.26 的 `encoding/json/v2` +
`encoding/json/jsontext` 之上。它是框架统一的 JSON 边界：所有序列化 / 反序列化都要
经过它，因此整个代码库共享同一套默认行为（map key 稳定排序、nil map / slice 输出为
`null`）。它在带合理默认值的 `Marshal` / `Unmarshal` / `MarshalWrite` /
`UnmarshalRead` API 之上，还提供泛型 `Encode<T>` / `Decode<T>` 工具，用于手写流式
编解码、代码生成以及自定义 `JSONEncoder` / `JSONDecoder` 实现。

## 使用方式

导入路径：

```go
import "go-spring.org/stdlib/jsonflow"
```

### API 列表

- `Marshal(v, opts...) ([]byte, error)`
- `MarshalIndent(v, prefix, indent string) ([]byte, error)`
- `MarshalWrite(w io.Writer, v, opts...) error`
- `Unmarshal(b []byte, v) error`
- `UnmarshalRead(r io.Reader, v) error`
- `NewEncoder(w io.Writer) Encoder` / `NewDecoder(r io.Reader) Decoder`——
  面向手写 `JSONEncoder` / `JSONDecoder` 实现的流式编码器 / 解码器。

若 `v` 实现 `JSONEncoder` / `JSONDecoder`，则优先走这两个接口——这是值想自己控制
线上格式的可选钩子，代码生成器主要用它；否则走标准 `encoding/json/v2` 路径。

### 选项

选项实现被密封的 `MarshalOptions` 接口。内置选项：

- `Indent` / `IndentPrefix` —— 美化输出的缩进。
- `NilSliceAsNull` / `NilMapAsNull` —— nil 集合是否输出为 `null`
  （默认 `true`）。
- `Deterministic` —— map key 稳定排序（默认 `true`）。

### 流式工具

对于实现 `JSONEncoder` / `JSONDecoder` 的值，`jsonflow` 提供逐值 / 结构化工具：

Encoders（`Encoder = json.Encoder`）：

- `EncodeNull`、`EncodeBool[T]`、`EncodeInt[T]`、`EncodeUint[T]`、
  `EncodeFloat[T]`、`EncodeString[T]`、`EncodeBytes`（base64）、
  `EncodeAny[T]`、`EncodeObject`。
- `EncodeArrayBegin` / `EncodeArrayEnd` / `EncodeArray` 与
  `EncodeObjectBegin` / `EncodeObjectEnd` / `EncodeMap`。
- Map key 工具：`EncodeIntKey`、`EncodeUintKey`、`EncodeStringKey`。
- 除 `Bytes` 外每个标量都有 `Ptr` 变体：nil 指针输出 `null`。

Decoders（`Decoder = json.Decoder`）：

- `DecodeBool`、`DecodeInt[T]`、`DecodeUint[T]`、`DecodeFloat[T]`、
  `DecodeString`、`DecodeBytes`（base64）、`DecodeAny[T]`、`DecodeObject`。
- `DecodeArray`、`DecodeMap` 为高阶组合子：`DecodeArray[T](parseFn)`、
  `DecodeMap[K,V](parseKey, parseVal)` 组合出按类型的解码器，无需捕获框架级状态。
- `DecodeObjectBegin` / `DecodeObjectEnd` / `DecodeEOF` 用于框架化。
- Map key 解码器：`DecodeIntKey`、`DecodeUintKey`。
- 对应的 `Parse*` 变体供自定义 `parseFn` 回调复用。
- 除 `Bytes` 外每个标量都有 `Ptr` 变体：JSON 为 `null` 时返回 `nil`；
  `DecodeBytes` 自身处理 `null`。

标量工具是泛型的：`EncodeInt[T ~int|...]` 等在叶子层避免反射；配合
`mathutil.Overflow*`，Decoder 会在数值静默扩大前拒绝越界。

几个边界形态值得注意：

- `EncodeFloat` 把 `NaN`、`+Inf`、`-Inf` 分别映射为字符串 `"NaN"`、
  `"Infinity"`、`"-Infinity"`。这是为了让输出仍是合法 JSON，代价是往返时解码方
  必须理解这种约定。
- `DecodeBytes` 把 `null` 处理为"返回 nil、无错误"，而 `DecodeString` 把
  `null` 视为错误。字节切片常见可选，字符串通常不可选，形态反映了这一点。
- 数值 map key 在 `ParseIntKey` / `ParseUintKey` 中同时接受 `"..."` 与 `0`
  两种 token —— `encoding/json/v2` 会把数值 map key 序列化成字符串。

### 示例

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

## 依赖与兼容

依赖 `encoding/json/v2`，只支持 Go 1.26+，没有 v1 兼容层。流式工具对
`internal/json` 编程——它是厂商中立的 token 接口缝隙（Encoder、Decoder、
Kind）；`internal/jsonv2` 是它唯一的适配器，基于 `encoding/json/v2` 实现。

## 许可证

Apache License 2.0，详见 [LICENSE](../../LICENSE)。

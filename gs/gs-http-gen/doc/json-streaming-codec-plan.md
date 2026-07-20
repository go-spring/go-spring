# JSON 流式编解码改造计划

本文档用于记录 `gs-http-gen` 的 JSON 流式编解码改造任务。内容待确认后作为后续执行计划使用。

## 背景

当前 `gs-http-gen` 生成代码已经较完整地支持了流式 JSON 解码：

- 普通结构体会生成 `DecodeJSON(d jsonflow.Decoder) error`。
- JSON 请求体会生成对应的 `DecodeJSON`。
- 字段分发使用 `hashutil.FNV1a64`，未知字段通过 `d.SkipValue()` 跳过。
- `stdlib/jsonflow` 已提供 Decoder 抽象和 bool/int/uint/float/string/bytes/object/list/map 等解码辅助函数。

流式 JSON 编码仍未完善：

- `stdlib/jsonflow.Encoder` 当前只有 `WriteValue([]byte)` 低层接口。
- `jsonflow.EncodeInt` 为空实现，其他基础类型和容器类型编码辅助函数缺失。
- `gs-http-gen` 未生成 `EncodeJSON(e jsonflow.Encoder) error`。
- HTTP client/server/SSE 当前仍依赖 `jsonflow.MarshalWrite` 做反射编码，没有利用生成代码做逐字段编码。

## 改造目标

1. 建立与流式解码对称的流式编码 API。
2. 为生成结构体和 JSON 请求体生成 `EncodeJSON(e jsonflow.Encoder) error`。
3. 让 HTTP 请求体、普通响应、SSE `data:` 响应可以自动使用生成的流式编码实现。
4. 保持编码结果与现有 `jsonflow.MarshalWrite` 在默认配置下语义一致。
5. 顺手补齐当前解码方案中已发现的几个明显缺口。

## 非目标

- 不改变 IDL 语法。
- 不改变公开 HTTP handler/client 的主要调用方式。
- 不在第一阶段重写所有 JSON marshal/unmarshal 选项体系。
- 不把 OpenAPI 文档生成纳入本次改造。

## 第一阶段：完善 `stdlib/jsonflow` 流式编码能力

修改范围建议放在 `/Users/didi/Go-Spring/stdlib/jsonflow/`。

### 1.1 扩展 Encoder 抽象

当前内部接口只有：

```go
type Encoder interface {
    WriteValue(v []byte) error
}
```

建议扩展为能表达 token 级写入：

```go
type Encoder interface {
    WriteValue(v []byte) error
    WriteNull() error
    WriteBool(v bool) error
    WriteInt(v int64) error
    WriteUint(v uint64) error
    WriteFloat(v float64) error
    WriteString(v string) error
    WriteObjectBegin() error
    WriteObjectEnd() error
    WriteArrayBegin() error
    WriteArrayEnd() error
}
```

底层 `internal/jsonv2.Encoder` 可以基于 `encoding/json/jsontext.Encoder.WriteToken` 和 `WriteValue` 实现。

### 1.2 增加公开编码辅助函数

与 decode 函数对齐，补齐：

- `EncodeNull`
- `EncodeBool` / `EncodeBoolPtr`
- `EncodeInt` / `EncodeIntPtr`
- `EncodeUint` / `EncodeUintPtr`
- `EncodeFloat` / `EncodeFloatPtr`
- `EncodeString` / `EncodeStringPtr`
- `EncodeBytes`
- `EncodeAny`
- `EncodeObject`
- `EncodeArray`
- `EncodeMap`
- map key 编码函数，如 `EncodeStringKey`、`EncodeIntKey`、`EncodeUintKey`

建议函数形态与 decode 对称：

```go
type ObjectEncoder interface {
    EncodeJSON(e Encoder) error
}

func EncodeObject[T ObjectEncoder](e Encoder, v T) error
func EncodeArray[T any](fn func(Encoder, T) error) func(Encoder, []T) error
func EncodeMap[K comparable, V any](
    encodeKey func(Encoder, K) error,
    encodeVal func(Encoder, V) error,
) func(Encoder, map[K]V) error
```

### 1.3 处理 nil 和 omitempty 语义

基础策略：

- 指针为 nil 时编码为 `null`，是否省略由生成代码决定。
- nil slice/map 默认编码为 `null`，保持当前 `jsonflow.Marshal` 默认 `NilSliceAsNull(true)`、`NilMapAsNull(true)` 的语义。
- 空 slice/map 编码为 `[]` / `{}`。
- `bytes` 按 JSON 字符串输出 base64。
- `any` 可以先用 `jsonflow.Marshal` 得到完整 JSON 值，再 `WriteValue`。

## 第二阶段：在 `gs-http-gen` 生成流式编码代码

修改范围建议放在 `/Users/didi/Go-Spring/gs-http-gen/gen/generator/golang/type.go`。

### 2.1 增加 `genEncodeJSON`

新增与 `genDecodeJSON` 对称的生成函数，根据 `TypeKind` 返回编码函数名或表达式：

- bool -> `jsonflow.EncodeBool`
- *bool -> `jsonflow.EncodeBoolPtr`
- int/enum -> `jsonflow.EncodeInt[...]`
- *int/*enum -> `jsonflow.EncodeIntPtr[...]`
- uint -> `jsonflow.EncodeUint[...]`
- float -> `jsonflow.EncodeFloat[...]`
- string -> `jsonflow.EncodeString`
- *string -> `jsonflow.EncodeStringPtr`
- bytes -> `jsonflow.EncodeBytes`
- enum_as_string -> 可走 `jsonflow.EncodeAny[...]`，或生成专用 enum string encoder
- struct pointer -> `jsonflow.EncodeObject`
- list -> `jsonflow.EncodeArray(...)`
- map -> `jsonflow.EncodeMap(keyEncoder, valueEncoder)`
- any -> `jsonflow.EncodeAny[...]`

map key 需要特殊处理：

- `map[string]T` 使用 string key encoder。
- `map[int64]T` 使用 int key encoder，将 key 编码成 JSON object name 字符串。

### 2.2 增加字段级省略判断

生成代码需要尊重 `JSONTag.OmitEmpty`：

- `omitempty` 字段在空值时跳过。
- `json=",non-omitempty"` 字段即使为空也必须输出。
- required 字段通常不带 omitempty，必须输出。

建议生成一个内部 helper 表达式，例如：

```go
func genIsEmptyJSON(fieldName string, typeKind []TypeKind) string
```

不同类型的空值判断：

- 指针：`field == nil`
- slice/map/bytes：`len(field) == 0`
- string：`field == ""`
- bool：`!field`
- int/uint/float/enum：`field == 0`
- struct pointer：`field == nil`
- any：`field == nil`

需要注意：当前 IDL 设计里非 required 基础类型通常会生成指针，因此基础零值判断主要用于自定义 `go.type` 或特殊场景。

### 2.3 生成普通结构体 `EncodeJSON`

对非 request struct，在现有 `DecodeJSON` 之后生成：

```go
func (x *TypeName) EncodeJSON(e jsonflow.Encoder) error {
    if x == nil {
        return jsonflow.EncodeNull(e)
    }
    if err := jsonflow.EncodeObjectBegin(e); err != nil {
        return err
    }
    // 逐字段写 key/value
    if err := jsonflow.EncodeObjectEnd(e); err != nil {
        return err
    }
    return nil
}
```

字段写入顺序使用 IDL 字段顺序，以保证输出稳定。

### 2.4 生成 request body `EncodeJSON`

对 JSON encoded request body，在 `DecodeJSON` 附近生成：

```go
func (x *RequestNameBody) EncodeJSON(e jsonflow.Encoder) error
```

只编码非 path/query binding 的 body 字段，和现有 `DecodeJSON` 保持一致。

### 2.5 处理 receiver 和 client body 值传递

当前 client 模板中 body 设置为：

```go
Body: req.RequestNameBody,
```

如果 `EncodeJSON` 使用指针 receiver，建议改成：

```go
Body: &req.RequestNameBody,
```

或者在生成代码中使用值 receiver。推荐指针 receiver，因为和 `DecodeJSON`、`Validate` 风格一致，也便于 nil 判断。

## 第三阶段：让运行时优先使用流式编码

修改范围建议放在 `/Users/didi/Go-Spring/spring/httpclt/`、`/Users/didi/Go-Spring/spring/httpsvr/` 和 `/Users/didi/Go-Spring/stdlib/jsonflow/`。

### 3.1 封装统一入口

建议在 `jsonflow` 中新增：

```go
type EncodeJSONer interface {
    EncodeJSON(e Encoder) error
}

func MarshalWrite(w io.Writer, i any, opts ...MarshalOptions) error {
    if v, ok := i.(EncodeJSONer); ok && len(opts) == 0 {
        return v.EncodeJSON(NewEncoder(w))
    }
    return stdjsonv2.MarshalWrite(w, i, toJSONv2Options(opts)...)
}
```

这样 `httpsvr.HandleJSON`、`httpsvr.HandleStream`、`httpclt.doRequest` 可以尽量不改或少改。

### 3.2 SSE 输出注意点

SSE `data:` 当前直接调用 `jsonflow.MarshalWrite(w, res.GetData())`。如果 MarshalWrite 会自动识别 `EncodeJSON`，SSE 也能获得流式编码。

需要确认 `jsontext.Encoder` 写顶层值时是否追加换行。如果追加换行，SSE 当前后续又写 `\n`，可能产生多余空行。必要时需要在 `jsonflow.NewEncoder` 或 SSE 输出处使用不自动追加换行的封装策略。

## 第四阶段：补齐解码缺口

这些问题不属于“编码”本身，但会影响编解码方案完整性。

### 4.1 map key 解码

当前 `genDecodeJSON` 对 `map[int,T]` 会使用 `jsonflow.DecodeInt` 作为 key decoder。JSON object key 是字符串 token，应使用 `DecodeIntKey` / `DecodeUintKey`。

建议拆分：

```go
func genDecodeJSONKey(typeName string, typeKind []TypeKind) string
func genDecodeJSONValue(typeName string, typeKind []TypeKind) string
```

map key 调用 key 版本，普通值调用 value 版本。

### 4.2 any 类型解码

`getTypeKind` 已识别 `any` / `interface{}`，但 `genDecodeJSON` 缺少 `TypeKindAny` 分支。应补：

```go
case TypeKindAny:
    return "jsonflow.DecodeAny[" + typeName + "]"
```

### 4.3 required 容器 null 校验

文档说明 required 字段不得为 null。当前 `DecodeArray` / `DecodeMap` 遇到 null 返回 nil，如果 required list/map 字段存在但值为 null，生成代码会认为字段已存在。

可选方案：

- 在生成 required 容器字段时，decode 后检查 nil。
- 或新增 non-null 版本：`DecodeArrayRequired` / `DecodeMapRequired`。

建议第一阶段在生成代码中做检查，侵入最小。

### 4.4 oneof 约束校验

文档要求 oneof：

- 必须且只能有一个成员字段被赋值。
- `FieldType` 必须与实际成员字段一致。

当前生成代码只校验 `FieldType` 是否存在。需要在 oneof 类型的 `DecodeJSON`、`EncodeJSON`、`Validate` 中补齐一致性检查。

## 第五阶段：测试计划

### 5.1 `stdlib/jsonflow`

新增或补充测试：

- 基础类型编码。
- 指针 nil 和非 nil 编码。
- bytes base64 编码。
- any 编码。
- struct object 编码。
- list/map 嵌套编码。
- map int/uint key 编码。
- nil slice/map 和空 slice/map 的差异。
- streaming encode 和 `jsonflow.Marshal` 默认输出语义一致。

运行命令：

```sh
GOEXPERIMENT=jsonv2 go test ./jsonflow
```

### 5.2 `gs-http-gen`

新增生成器测试或 golden case：

- 普通 struct 生成 `EncodeJSON`。
- JSON request body 生成 `EncodeJSON`。
- `omitempty` 与 `non-omitempty` 输出差异。
- `enum_as_string` 输出。
- `bytes` 输出。
- `list` / `map` / 嵌套结构输出。
- `map<int,T>` 解码与编码。
- `any` 解码与编码。
- oneof 编解码约束。

运行命令：

```sh
GOEXPERIMENT=jsonv2 go test -vet=off ./...
```

说明：当前 `gs-http-gen` 存在 `errutil.Explain(... "%w" ...)` 被 vet 拦截的问题，因此暂时需要 `-vet=off`。这不是本次流式编码改造的核心问题，但后续最好单独修复。

### 5.3 examples

重新生成 examples 后运行：

```sh
GOEXPERIMENT=jsonv2 go test -vet=off ./...
```

并建议增加一个 request/response roundtrip 测试，覆盖 assistant SSE payload 和 manager CRUD 响应对象。

## 建议执行顺序

1. 在 `stdlib/jsonflow` 补全 Encoder 抽象和基础编码函数。
2. 给 `stdlib/jsonflow` 增加编码单测，先不动生成器。
3. 在 `gs-http-gen` 增加 `genEncodeJSON` 和普通 struct `EncodeJSON` 生成。
4. 增加 JSON request body `EncodeJSON` 生成，并调整 client body 传指针。
5. 修改 `jsonflow.MarshalWrite` 优先使用 `EncodeJSON`。
6. 补齐 map key、any、required 容器 null、oneof 校验等解码缺口。
7. 重新生成 examples，并补充 roundtrip/golden 测试。
8. 跑完整测试并记录结果。

## 风险和确认点

执行前需要确认以下决策：

1. `jsonflow.Encoder` 是否允许扩展为 token 级接口，还是保持只暴露 `WriteValue` 并在 helper 内自行构造 JSON 片段。
2. `EncodeJSON` 使用指针 receiver 还是值 receiver。建议使用指针 receiver。
3. `MarshalWrite` 是否应该在有 options 时继续走反射路径。建议第一阶段仅 `len(opts) == 0` 时走生成流式编码。
4. SSE `data:` 是否允许 encoder 自动追加换行。如果不允许，需要为 SSE 使用无额外换行的 writer/encoder 策略。
5. 是否把 oneof 校验放入本次改造。建议纳入，因为编码必须知道应输出哪个成员字段。

## 完成标准

- `stdlib/jsonflow` 编码辅助函数完整覆盖当前 IDL 类型系统。
- 生成的所有 struct/request body 都能编译出 `EncodeJSON`。
- HTTP client/server/SSE 默认路径能自动优先使用生成的流式编码。
- 现有 decode 测试保持通过。
- 新增 encode 和 roundtrip 测试通过。
- `GOEXPERIMENT=jsonv2 go test ./jsonflow` 通过。
- `GOEXPERIMENT=jsonv2 go test -vet=off ./...` 在 `gs-http-gen` 通过。

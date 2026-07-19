# jsonflow 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

属于零依赖的 `stdlib` 层。`jsonflow` 是 Go-Spring 的 JSON 边界：框架中所有
的序列化 / 反序列化都要经过它，因此它拥有默认值与流式缝隙的定义权。

## 1. 职责与边界

- 提供统一的 JSON 入口（`Marshal` / `Unmarshal` 及其流式变体），让整个代码库
  共享同一套默认行为 —— key 稳定排序、nil 集合输出 `null`。
- 暴露带类型的逐 token 编解码工具，供手写代码或生成代码在实现 `JSONEncoder` /
  `JSONDecoder` 时使用，避免走反射。
- 不是 schema 库。字段顺序、发现、校验都归调用方，`jsonflow` 只理解原始
  token 和上述两个接口。

## 2. 关键抽象与缝隙

- **`JSONEncoder` / `JSONDecoder`**：值想自己控制线上格式的可选钩子。
  `Marshal` / `UnmarshalRead` 会先做类型断言，再退回 `encoding/json/v2`。
  代码生成器主要用它。
- **密封的 `MarshalOptions`**：`JSONOptions` 上带有未导出的
  `NotForPublicUse{}` 参数，把选项集封闭。新增能力都以新的包级类型出现
  （`Indent`、`NilSliceAsNull` 等），牺牲了外部扩展性换来 API 稳定。
- **稳定默认**：`NilSliceAsNull(true)`、`NilMapAsNull(true)`、
  `Deterministic(true)` 恒先追加，然后再让用户选项覆盖。这样 golden 测试
  与缓存 key 在跨进程运行时都稳定。
- **泛型标量工具**：`EncodeInt[T ~int|...]` 在叶子层避免反射；配合
  `mathutil.Overflow*`，Decoder 会在数值静默扩大前拒绝越界。
- **高阶组合子**：`DecodeArray[T](parseFn)`、`DecodeMap[K,V](parseKey, parseVal)`
  让生成代码组合出按类型的解码器，无需捕获框架级状态。

## 3. 约束与取舍

- 依赖 `encoding/json/v2`，只支持 Go 1.26+。v1 兼容层位于 `internal/json`，
  流式工具都对它编程。
- `EncodeFloat` 把 `NaN`、`+Inf`、`-Inf` 分别映射为字符串 `"NaN"`、
  `"Infinity"`、`"-Infinity"`。这是为了让输出仍是合法 JSON，代价是往返时
  解码方必须理解这种约定。
- `DecodeBytes` 把 `null` 处理为"返回 nil、无错误"，而 `DecodeString`
  把 `null` 视为错误。字节切片常见可选，字符串通常不可选，形态反映了这
  一点。
- 数值 map key 在 `ParseIntKey` / `ParseUintKey` 中同时接受 `"..."` 与 `0`
  两种 token —— `encoding/json/v2` 会把数值 map key 序列化成字符串。

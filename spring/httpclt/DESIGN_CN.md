# httpclt Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`httpclt` 是代码生成器(`gs-http-gen`)所面向的运行时。位于 `stdlib`,只 import
`net/http` / `stdlib/jsonflow` 与少量标准库,让生成客户端保持 stdlib-only,不把
starter 依赖泄给调用方。

## 1. 职责与边界

- 承载生成器所输出的每次请求的 `Metadata`,推动请求构建、发送与流式解码。
- 提供两个 hook(`Metadata.Client` 与包级 `DoRequest` 变量),让调用方——starter、
  测试、或普通应用代码——不动生成的调用点即可整段替换 HTTP 调用。
- 拒绝关心服务发现、负载均衡、resilience、trace 透传。这些都由注入到
  `Metadata.Client` 的 `*http.Client` 负责(见 `spring/httpx`)。

## 2. 关键抽象与缝隙

- **`Metadata`**——声明式调用描述。刻意保留 `Client *http.Client` 字段:这是
  生成客户端接入 discovery + LB + resilience + otel 的唯一缝隙,生成代码无需改动。
- **`DoRequest` 包级变量**——测试/观测缝隙。替换它可短路整段 HTTP 调用。
- **`QueryStringer` / `EncodeForm` / `ResponseObject` 接口**——两个编码可插拔
  点(`QueryForm`、`EncodeForm`)与一个解码可插拔点(`DecodeJSON`),生成类型实
  现之,`httpclt` 无需对业务 struct 做运行时反射。

## 3. 约束

- 运行时由生成器驱动:`Metadata` 上的字段是与 `gs-http-gen` 输出的契约,改字段
  名即破坏契约。
- `Metadata.Client == nil` 必须静默回落到 `http.DefaultClient`——生成器输出这个
  字段,应用在测试或脚本里可能不填。
- 默认流式 JSON:`ObjectResponse` / `JSONResponse` 经 `jsonflow` 增量解码,不
  在内存中缓冲整个响应体。

## 4. 取舍与被否决方案

- **httpclt 内不构造 client。** 传输层、超时、Cookie jar、TLS 配置都在调用方注
  入的 `*http.Client` 里。这正是 client 侧集成(`starter-http-client`、测试、
  contract 桩)可插拔而不改生成代码的原因。
- **不对业务类型做反射。** 用接口方法(`QueryForm` / `EncodeForm` /
  `DecodeJSON`)让运行时保持快 + 零依赖,代价是让代码生成器承担对偶职责。

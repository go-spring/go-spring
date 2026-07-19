# gs-http-gen 设计说明
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`gs-http-gen` 是四层栈（stdlib → spring → starter → gs）工具层里 `gs`
的外部工具。它基于项目 IDL（接口定义语言）文件生成 HTTP 服务端代码、
多语言客户端代码，以及 OpenAPI / Swagger 文档。

## 1. 职责与边界

- 读 `idl/http/` 下的 IDL（`gs gen` 对该协议子目录派发过来），产出：
  - Go 数据模型 + 校验
  - HTTP 路由绑定（普通 + 流式 / SSE）
  - 服务端 handler 脚手架
  - Go / PHP / Java 客户端桩（`--language`）
  - Swagger 2.0（`--swagger`）或 OpenAPI 3.0（`--openapi`）文档
- 只管 HTTP。其它协议（gRPC / Thrift / …）各有各的生成器；按 layout
  约定，每种协议的 IDL 独立放在 `idl/<framework>[-<protocol>]/` 目录。

## 2. 关键抽象与接缝

- **外部工具协议**。二进制名 `gs-http-gen`，与 `gs` 并排放置。
  `gs-http-gen --version` 打印 description + version；`gs gen` 遇到
  `idl/http/` 时通过 `gs/gs/cmd/proto` 派发过来。
- **无子命令的 cobra root**。所有模式都是根命令上的 flag：`--server`、
  `--client`、`--swagger`、`--openapi`；doc-gen 与 code-gen 互斥
  （`--swagger`/`--openapi` 不能与 `--server`/`--client` 一起用）。
- **`--go_package`** 控制生成的 Go 包名（默认 `proto`）；`--output`
  指定输出目录（默认 `.`）。
- **IDL 表面**。工具自带 IDL 语法（`standard@v1.idl` 内置）。支持常
  量、枚举、结构体、`oneof`、泛型、字段嵌入复用——比裸 JSON schema 表
  达力更强，让服务端与客户端共用一份源。

## 3. 约束

- 单次调用里文档生成与代码生成互斥。同时请求两者会直接报错。
- 一个生成器一种协议：`gs-http-gen` 不处理 Thrift / gRPC IDL。扩展它
  去处理更多协议违反 layout 的"每协议独立 IDL + 生成器 + 原生类型"约
  定。

## 4. 权衡与被否决的方案

- **不复用 `protoc` 做 HTTP**。HTTP 语义（path、query、header、流）没
  法干净地映射进 `.proto`；`gs-http-gen` 的 IDL 是专为 HTTP 定制，让
  产出的 handler 保持 idiomatic Go。
- **客户端多语言、服务端只 Go**。跨团队消费方常需要 PHP/Java 客户端
  但服务用 Go 实现；这个表面是有意做成不对称的。
- **文档作为独立模式，非常态开启**。若每次 `--server` 构建都产出
  `.yaml`，会污染 PR diff；由用户显式开启。

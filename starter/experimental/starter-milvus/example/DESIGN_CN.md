# starter-milvus 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

Milvus（向量数据库）的 Client 原型 starter，走 gRPC。

## 1. 职责与边界

- **负责**：每实例装配、逐 RPC 治理守卫（装在 SDK dial options 上的 gRPC 客户端拦截器）、
  每个 RPC 可观测身份的声明、fail-fast `HealthCheck`（`ListCollections`）探针、健康指示器、
  连接拆除。
- **不负责**：schema 设计（集合/字段/索引是应用的事）、检索/查询语义，以及逐 RPC 信号的
  发射——那由 executor 内的 resilience 层完成（见 §4）。

## 2. 关键抽象与 Seam

- **Client 包装** — 内嵌 SDK 的 `client.Client` 接口，其完整面被提升；
  bean 是薄持有者，因为 SDK 自己的 client 已是完整面。
- **守卫拦截器** — 一个 unary、一个 stream 建流 gRPC 拦截器，让每个 RPC 都过 resilience
  executor。它们同时是**声明 seam**：各自在 executor 之前把 RPC 身份放到调用方 ctx 上
  （`observe.go`），于是 executor 内唯一的发射器据此命名 span、`db.client.*` 指标与访问日志。
  starter 自身不发射任何信号。
- **fail-fast 探针** — 构造期 `HealthCheck`（`ListCollections`）；配错地址或凭证在启动期
  就失败，而非首次查询。

## 3. 约束

- 仅 gRPC；无可替换的 HTTP 传输层。

## 4. 权衡 / 已否决的方案

- **此处声明、resilience 层发射** — 守卫拦截器就是天然的逐 RPC seam（走 SDK 真正使用的
  传输层），因此逐操作韧性与可观测性无需为整个 `client.Client` 接口手写门面。starter 只声明
  每个 RPC 是什么（`observe.go`）；resilience 层从唯一能看到整次调用（含重试）的位置发射调用级
  `db.client.operation.duration`、尝试级 `db.client.attempt.duration` 直方图与
  `_app_milvus_access` 日志。已否决的替代方案——手写门面自行包裹每个向量操作——会重复这个 seam，
  并重新发射 executor 已经产出的信号。
- **无 TLS/鉴权 dial option 逃生口** — `Config` 不暴露追加自定义 dial option 的能力，应用若
  需要额外拦截器（鉴权 token、自定义 trace）只能 fork `newClient`。

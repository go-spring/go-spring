# starter-milvus

[English](README.md) | [中文](README_CN.md)

`starter-milvus` 提供 [Milvus](https://milvus.io)（向量数据库）支持：多实例
`client.Client` bean、可选 fail-fast 启动探针与每实例健康指示器，基于官方
[milvus-sdk-go](https://github.com/milvus-io/milvus-sdk-go) v2（gRPC）。

## 安装

```bash
go get go-spring.org/starter-milvus
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-milvus"
```

### 2. 配置

```properties
spring.milvus.instances.a.addr=127.0.0.1:19530
spring.milvus.instances.a.database=default
```

### 3. 注入

```go
type Service struct {
    Client *StarterMilvus.Client `autowire:"a"`
}
```

### 4. 使用

```go
err := s.Client.NewCollection(ctx, "docs", 768)
_, err = s.Client.Insert(ctx, "docs", "", idCol, vecCol)
```

包装类型内嵌 SDK 的 `client.Client` 接口，所有方法（集合/插入/检索/索引/
分区/…）被原样提升可用。

## 核心特性

- **多实例客户端** — 每个 `spring.milvus.instances.<name>` 条目一个 bean。
- **fail-fast 探针 + 健康指示器** — `HealthCheck`（`ListCollections`）在启动期验证连通与
  鉴权（opt-in：`ping=true`；默认关闭），同时作为就绪探针（`health=false` 可跳过）。
- **透明逐 RPC 韧性** — 治理守卫装在 SDK 的 gRPC dial options 上（unary + stream 拦截器），
  限流/熔断/隔舱/重试/超时对每个 RPC 生效，调用点零改动；治理经共享 `spring.governance.*`
  规则消费。

## 可观测性

starter 只**声明**每个 RPC 是什么（`observe.go`：span 名、`db.client` 指标前缀、
`db.system`/`db.operation` 标签、完整方法路径作为 span/日志 detail）；**发射**由 resilience
层完成——executor 内唯一能看到整次调用（含重试）的位置。因此每次调用都产出调用级
`db.client.operation.duration` 直方图、尝试级 `db.client.attempt.duration` 直方图，以及
打上 `_app_milvus_access` 标签的访问日志。治理关闭时只停保护、不停观测；指标还需 `starter-otel`
安装 provider。

## 高级特性

**多客户端** — 更多条目，各自独立的连接。

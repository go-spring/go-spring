# starter-milvus

[English](README.md) | [中文](README_CN.md)

`starter-milvus` 提供 [Milvus](https://milvus.io)（向量数据库）支持：多实例
`client.Client` bean、fail-fast 启动探针与每实例健康指示器，基于官方
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
分区/…）原样提升可用。

## 核心特性

- **多实例客户端** — 每个 `spring.milvus.instances.<name>` 条目一个 bean。
- **fail-fast 探针 + 健康指示器** — `ListCollections` 在启动期验证连通与
  鉴权，同时作为就绪探针。
- **无逐操作韧性** — SDK 没有可拒绝的拦截器 seam（gRPC dial option 仅
  构建期）；边界见 DESIGN。

## 高级特性

**多客户端** — 更多条目，各自独立的连接。

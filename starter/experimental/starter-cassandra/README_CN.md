# starter-cassandra

[English](README.md) | [中文](README_CN.md)

`starter-cassandra` 基于官方 [gocql](https://github.com/gocql/gocql) 驱动为
Go-Spring 提供 Cassandra / ScyllaDB 支持（两者都说 CQL 原生协议）：多实例
session bean、fail-fast 启动探针、韧性守卫的 `Exec` 助手、每实例健康指示
器，可选 PasswordAuthenticator 与 TLS。

## 安装

```bash
go get go-spring.org/starter-cassandra
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-cassandra"
```

### 2. 配置

```properties
spring.cassandra.instances.a.hosts=127.0.0.1
spring.cassandra.instances.a.keyspace=demo
spring.cassandra.instances.a.consistency=local-quorum

# 认证 + TLS（可选）
# spring.cassandra.instances.a.username=cassandra
# spring.cassandra.instances.a.password=cassandra
# spring.cassandra.instances.a.tls.enabled=true
# spring.cassandra.instances.a.tls.ca-file=/etc/certs/ca.pem
```

### 3. 注入

```go
type Service struct {
    Client *StarterCassandra.Client `autowire:"a"`
}
```

### 4. 使用

```go
// 守卫路径：韧性（限流/熔断）。语句的 span、指标与 access log 由此处声明，由韧性层发射。
err := s.Client.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?)", 1, "hello")

// 经包装的 session 使用完整查询能力
var msg string
err = s.Client.Query("SELECT message FROM demo.greetings WHERE id = ?", 1).
    WithContext(ctx).Scan(&msg)
```

## 核心特性

- **多实例客户端** — 每个 `spring.cassandra.instances.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **fail-fast 启动探针 + 健康指示器** — `HealthCheck`（一次 `system.local` 扫描）
  在启动期执行，`cassandra:<name>` 指示器供 `starter-actuator` 聚合，二者都委托给同一实现。
- **守卫的 Exec** — 同步语句走治理执行器；守卫的 `Query` 包装同样覆盖
  `Iter`/`Scan`，而返回迭代器内部的翻页与 batch 执行仍在守卫之外
  （语句级粒度，与 database/sql 系 starter 同立场）。
- **集群发现** — 接触点列表引导驱动自身的拓扑发现；条目可带端口
  （`host:9042`）。

## 可观测性

`gocql` 不提供官方 OpenTelemetry 插桩。本 starter 因此只声明每条语句是什么
（`observe.go`）：操作名、`db.system`/`db.operation` 标签，以及作为 `db.statement`
span/log 明细的 CQL 文本。信号本身由韧性层发射——执行器链上唯一能看到整次调用
（含重试）的位置，因此除调用级 `db.client.operation.duration` 直方图外，每次调用还
上报一个尝试级 `db.client.attempt.duration`：access log 的 tag 为 `_app_cassandra_access`。
不 import `starter-otel` 时，这一切都是 no-op。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.cassandra.instances.main.hosts=10.0.0.1,10.0.0.2
spring.cassandra.instances.main.keyspace=prod
spring.cassandra.instances.analytics.hosts=10.0.1.1
```

**自定义 driver** — 把自己的 `Driver` 作为可选容器 bean 提供，替换
session 装配（例如锁定 HostSelectionPolicy 或 Scylla 分片感知驱动）。
`spring.cassandra` 下每个 client 都经它构建；无该 bean 时 starter 回退到内置
`DefaultDriver`：

```go
func init() {
    gs.Provide(func() StarterCassandra.Driver { return scyllaDriver{} })
}
```

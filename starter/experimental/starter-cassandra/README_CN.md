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
spring.cassandra.a.hosts=127.0.0.1
spring.cassandra.a.keyspace=demo
spring.cassandra.a.consistency=local-quorum

# 认证 + TLS（可选）
# spring.cassandra.a.username=cassandra
# spring.cassandra.a.password=cassandra
# spring.cassandra.a.tls.enabled=true
# spring.cassandra.a.tls.ca-file=/etc/certs/ca.pem
```

### 3. 注入

```go
type Service struct {
    Client *StarterCassandra.Client `autowire:"a"`
}
```

### 4. 使用

```go
// 守卫路径：韧性（限流/熔断）+ 观测
err := s.Client.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?)", 1, "hello")

// 经内嵌 session 使用完整查询能力
var msg string
err = s.Client.Query("SELECT message FROM demo.greetings WHERE id = ?", 1).
    WithContext(ctx).Scan(&msg)
```

## 核心特性

- **多实例客户端** — 每个 `spring.cassandra.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **fail-fast 启动探针 + 健康指示器** — 启动期一次 `system.local` 扫描，
  `cassandra:<name>` 指示器供 `starter-actuator` 聚合。
- **守卫的 Exec** — 同步语句走治理执行器；迭代器/分页查询直接用内嵌
  session（按设计不守卫，与 MQ starter 的异步路径同立场）。
- **集群发现** — 接触点列表引导驱动自身的拓扑发现；条目可带端口
  （`host:9042`）。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.cassandra.main.hosts=10.0.0.1,10.0.0.2
spring.cassandra.main.keyspace=prod
spring.cassandra.analytics.hosts=10.0.1.1
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

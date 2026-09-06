# starter-tdengine

[English](README.md) | [中文](README_CN.md)

`starter-tdengine` 为 Go-Spring 提供 TDengine 支持，基于官方
[driver-go](https://github.com/taosdata/driver-go) v3 的 **websocket 驱动**
（taosWS）—— 纯 Go、无需安装客户端库、无 CGO。bean 是一个 `*sql.DB`
连接池：多实例客户端、fail-fast 启动探活、连接缝上的逐语句可观测与韧性，
以及每实例健康指示器。

## 安装

```bash
go get go-spring.org/starter-tdengine
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-tdengine"
```

### 2. 配置

```properties
spring.tdengine.a.dsn=root:taosdata@ws(127.0.0.1:6041)/power
spring.tdengine.a.max-open-conns=8
```

### 3. 注入

```go
type Service struct {
    Client *StarterTdengine.Client `autowire:"a"`
}
```

### 4. 使用

```go
_, err := s.Client.ExecContext(ctx,
    "INSERT INTO power.d001 USING power.meters TAGS('beijing') VALUES (NOW, 10.5)")
rows, err := s.Client.QueryContext(ctx, "SELECT COUNT(*) FROM power.meters")
```

包装类型内嵌 `*sql.DB`，`Query/Exec/BeginTx/PingContext` 与整个
`database/sql` 生态原样提升可用。

## 核心特性

- **多实例客户端** — 每个 `spring.tdengine.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **fail-fast 启动探活 + 健康指示器** — 启动期一次 `PingContext`，
  `tdengine:<name>` 指示器供 `starter-actuator` 聚合。
- **逐语句韧性 + 可观测** — 语句经守卫过的 driver.Conn 流动：限流、熔断、
  故障注入、逐语句 span + 指标 + 访问日志，全部在 Init 里武装。
- **websocket 线路、零 CGO** — 支持任何运行 taosAdapter 的 TDengine
  ≥ 3.3.6；`wss://` DSN 即 TLS。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.tdengine.hot.dsn=root:taosdata@ws(10.0.0.1:6041)/power
spring.tdengine.cold.dsn=root:taosdata@ws(10.0.0.2:6041)/archive
```

**自定义 driver** — 提供自己的 `Driver` bean 来替换客户端装配（例如锁定其它
线路协议或连接调优）。`Driver` 是可选容器 bean：`${spring.tdengine}` 下每个
客户端都经它装配，未提供时 starter 回退到内置 `DefaultDriver`。在包 init 里
注册（构造函数返回 `StarterTdengine.Driver`）：

```go
func init() {
    gs.Provide(func() StarterTdengine.Driver {
        return restDriver{}
    })
}
```

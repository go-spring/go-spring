# starter-tdengine

[English](README.md) | [中文](README_CN.md)

`starter-tdengine` 为 Go-Spring 提供 TDengine 支持，基于官方
[driver-go](https://github.com/taosdata/driver-go) v3 的 **websocket 驱动**
（taosWS）—— 纯 Go、无需安装客户端库、无 CGO。bean 是一个 `*sql.DB`
连接池：多实例客户端、可选 fail-fast 启动探活、连接缝上的逐语句可观测与韧性，
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
spring.tdengine.instances.a.dsn=root:taosdata@ws(127.0.0.1:6041)/power
spring.tdengine.instances.a.max-open-conns=8
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

包装类型内嵌裸 `*sql.DB`，整个 `database/sql` 方法集——
`Query/Exec/BeginTx/PingContext` 等——按原样提升。

## 核心特性

- **多实例客户端** — 每个 `spring.tdengine.instances.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **fail-fast 启动探活 + 健康指示器** — 启动期一次 `PingContext`（需 `ping=true`；
  默认关闭，使尚未就绪的 server 不阻塞启动），`tdengine:<name>` 指示器供
  `starter-actuator` 聚合（`health=false` 可跳过）。
- **逐语句韧性 + 可观测** — 语句经守卫过的 driver.Conn 流动：每条语句外套
  限流、熔断与故障注入，并声明其语义身份，交由 resilience 层发射
  （见[可观测](#可观测)）。
- **websocket 线路、零 CGO** — 支持任何运行 taosAdapter 的 TDengine
  ≥ 3.3.6；`wss://` DSN 即 TLS。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.tdengine.instances.hot.dsn=root:taosdata@ws(10.0.0.1:6041)/power
spring.tdengine.instances.cold.dsn=root:taosdata@ws(10.0.0.2:6041)/archive
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

## 可观测

`driver-go` 自身不带埋点。starter 因此只**声明**每条语句是什么
（`observe.go`）：语句种类（`exec`/`query`）、`db.system`/`db.operation`
标签，以及作为 `db.statement` span/日志 detail 的 SQL（截断至 512 字节）。
信号本身由 **resilience 层发射**——它是 executor 链条上唯一看得见整次调用
（含重试）的点。除 call 级 `db.client.operation.duration` 直方图外，每次
调用还多一条 attempt 级的 `db.client.attempt.duration` 直方图，以及 tag 为
`_app_tdengine_access` 的访问日志。未引入 `starter-otel` 时全部为 no-op。

## 设计说明

* **DSN 刻意保持整体。** `dsn` 是驱动的统一格式（`user:pass@ws(host:port)/db`），不拆成
  host/port/user 字段，因此驱动自身的参数面（`?readTimeout=…`）原样透传。
* **TDengine 没有事务。** 提升上来的 `Begin`/`BeginTx` 委托给驱动，由它报错——不要基于
  `database/sql` 事务来构建。
* **守卫落在 driver 连接层。** 限流、熔断与故障注入包裹每条池化连接上的每次
  `ExecContext`/`QueryContext`，因此所有调用方——包括架在池上的 ORM——都被覆盖，无需记得用
  `Guarded*` 助手。
* **并非 `database/sql` 能表达的一切都可用。** STMT 参数绑定未被建模（需要时直接用 SDK），
  超级表/无模式写入仍是原生 SQL。DSN 指向单个 taosAdapter，因此没有连接级负载均衡。
* **服务端版本有要求。** driver-go v3.8.2 在 websocket 路径上需要 TDengine ≥ 3.3.6.0；
  更老的服务端会在启动探活（`ping=true`）时报驱动的版本错误，否则在首次使用时暴露。

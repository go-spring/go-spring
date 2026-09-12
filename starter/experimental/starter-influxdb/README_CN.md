# starter-influxdb

[English](README.md) | [中文](README_CN.md)

`starter-influxdb` 为 Go-Spring 提供 InfluxDB 2.x 支持：多实例
`influxdb2.Client` bean、fail-fast 启动探针、逐请求可观测（span + 指标 +
访问日志）、阻塞写路径上的韧性（限流/熔断/故障注入）、错误自动排入日志的
托管异步写入器，以及每实例健康指示器。基于官方
[influxdb-client-go](https://github.com/influxdata/influxdb-client-go) v2。

## 安装

```bash
go get go-spring.org/starter-influxdb
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-influxdb"
```

### 2. 配置

```properties
spring.influxdb.instances.a.server-url=http://127.0.0.1:8086
spring.influxdb.instances.a.auth-token=my-token
spring.influxdb.instances.a.org=my-org
spring.influxdb.instances.a.bucket=my-bucket
```

### 3. 注入

```go
type Service struct {
    Client *StarterInfluxdb.Client `autowire:"a"`
}
```

### 4. 使用

```go
p := influxdb2.NewPointWithMeasurement("cpu").
    AddTag("host", "server-01").
    AddField("usage_idle", 42.5)
err := s.Client.WritePoints(ctx, p)

// 经内嵌客户端做 Flux 查询
raw, err := s.Client.QueryAPI(s.Client.Org()).
    QueryRaw(ctx, `from(bucket:"my-bucket") |> range(start: -1m)`, influxdb2.DefaultDialect())
```

包装类型内嵌 `influxdb2.Client`，SDK 的所有方法（QueryAPI、DeleteAPI、
Setup……）原样提升可用。

## 核心特性

- **多实例客户端** — 每个 `spring.influxdb.instances.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **双写入口** — `WritePoints`（阻塞、韧性保护、逐次报错）与
  `ManagedWriteAPI`（后台缓冲批量、停机时 flush；失败批次排入 go-spring
  日志，写入器永不阻塞）。拆分理由见 DESIGN。
- **fail-fast 启动探针 + 健康指示器** — 启动期一次 `/health` 往返，
  `influxdb:<name>` 指示器供 `starter-actuator` 聚合。
- **可观测** — 每个 HTTP 请求产出 client span（db.system/db.operation/
  db.statement 属性）、`db.client.operation.duration` 直方图 +
  `db.client.active_requests` 计量，以及 `_app_influxdb_access` tag 的访问
  日志（走 log 包原生分级）。
- **韧性** — 阻塞写路径走治理 seam；未导入 `starter-governance` 时仅做
  观测。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.influxdb.instances.metrics.server-url=http://influx-a:8086
spring.influxdb.instances.metrics.auth-token=...
spring.influxdb.instances.events.server-url=http://influx-b:8086
spring.influxdb.instances.events.auth-token=...
```

**自定义 driver** — 提供自己的 `Driver` bean 来替换客户端装配（例如接入
会话令牌凭证流）。`Driver` 是可选容器 bean：`${spring.influxdb}` 下每个客户端都
经它装配，未提供时 starter 回退到内置 `DefaultDriver`。在包 init 里注册
（构造函数返回 `StarterInfluxdb.Driver`）：

```go
func init() {
    gs.Provide(func() StarterInfluxdb.Driver {
        return v1CompatDriver{}
    })
}
```
### 日志 tag

本模块的运行期日志使用 tag `_app_influxdb`（influxdb 客户端）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.influxdb.type=Logger
logger.influxdb.level=WARN
logger.influxdb.tag=_app_influxdb
```

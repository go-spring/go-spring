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
spring.influxdb.a.server-url=http://127.0.0.1:8086
spring.influxdb.a.auth-token=my-token
spring.influxdb.a.org=my-org
spring.influxdb.a.bucket=my-bucket
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

- **多实例客户端** — 每个 `spring.influxdb.<name>` 条目都是独立 bean，
  拥有各自的配置。
- **双写入口** — `WritePoints`（阻塞、韧性保护、逐次报错）与
  `ManagedWriteAPI`（后台缓冲批量、停机时 flush；失败批次排入 go-spring
  日志，写入器永不阻塞）。拆分理由见 DESIGN。
- **fail-fast 启动探针 + 健康指示器** — 启动期一次 `/health` 往返，
  `influxdb:<name>` 指示器供 `starter-actuator` 聚合。
- **可观测** — 每个 HTTP 请求经 observe kit 产出 span、耗时指标与访问
  日志（`observability.level=off` 关闭）。
- **韧性** — 阻塞写路径走治理 seam；未导入 `starter-governance` 时仅做
  观测。

## 高级特性

**多客户端** — 配置更多条目并按名注入：

```properties
spring.influxdb.metrics.server-url=http://influx-a:8086
spring.influxdb.metrics.auth-token=...
spring.influxdb.events.server-url=http://influx-b:8086
spring.influxdb.events.auth-token=...
```

**自定义 driver** — 注册自己的 `Driver` 并用 `driver=<name>` 选中，替换
客户端装配（例如接入会话令牌凭证流）：

```go
func init() {
    StarterInfluxdb.RegisterDriver("v1-compat", v1CompatDriver{})
}
```
### 日志 tag

本模块的运行期日志使用 tag `_app_influxdb`（influxdb 客户端）。如需与主日志分开单独调整，可为该 tag 绑定独立的 logger：

```properties
logger.influxdb.type=Logger
logger.influxdb.level=WARN
logger.influxdb.tag=_app_influxdb
```

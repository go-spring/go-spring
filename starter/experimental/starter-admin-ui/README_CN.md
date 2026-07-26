# starter-admin-ui

[English](README.md) | [中文](README_CN.md)

> 面向 Go-Spring 的轻量"Spring Boot Admin 等价物":一个自包含的 HTML 看板,
> 定时轮询一组应用实例的 `starter-actuator` 端点,聚合渲染集群健康状态。

`starter-admin-ui` 自持一个独立的 HTTP 端口(默认 `:9280`),后台协程按周期拉取
每个配置实例的 `/health`、`/readiness`、`/startup`、`/info`,以一行一个实例的表格
展示状态色标、组件级健康、构建信息。

## 定位——什么时候用这个

在 K8s 部署里,聚合监控的标准做法是 **Prometheus + Grafana**(见本仓库
`contrib/observability/`)。那套栈有时序历史、告警、丰富仪表盘,是推荐默认。
本 starter 刻意收窄范围,适用场景:

- 没有 Prometheus/Grafana,或者对当前部署过重。
- 想在几个 pod 之间要一个一眼扫过去的单页视图(私有化/内网集群)。
- 新环境冒烟起步阶段,先要个粗略看板,再慢慢接完整可观测栈。

本 starter **不是** Spring Boot Admin 的功能对齐复刻;它用 Go 惯用法达成
"等价效果"(轮询 actuator、渲染状态表)——一页 HTML、一个轮询协程、零第三方
依赖。理由详见 [DECISION.md](DECISION.md)。

## 安装

```bash
go get go-spring.org/starter-admin-ui
```

## 快速开始

### 1. 导入包

参见 [example.go](example/example.go)。

```go
import _ "go-spring.org/starter-admin-ui"
```

### 2. 配置看板

在项目的[配置文件](example/conf/app.properties)里加入:

```properties
spring.admin-ui.enabled=true
spring.admin-ui.addr=:9280
spring.admin-ui.instances[0]=http://10.0.0.1:9370
spring.admin-ui.instances[1]=http://10.0.0.2:9370
spring.admin-ui.interval=10s
spring.admin-ui.timeout=3s
spring.admin-ui.title=Go-Spring Admin
```

每个 `instances[i]` 是一个 base URL——轮询时会自动拼上 `/health`、`/readiness`、
`/startup`、`/info`。`instances` 为空时看板仍然可访问,只是显示"未配置实例"。

### 3. 打开看板

```bash
open http://127.0.0.1:9280/
```

页面通过 `<meta http-equiv="refresh">` 按 `spring.admin-ui.interval` 自动刷新。
没有 CDN、没有外部资源拉取——一整页 HTML 从嵌入模板渲染,离线环境可直接使用。

## 端点

| 端点 | 方法 | 用途 |
| --- | --- | --- |
| `/` | GET | HTML 看板:聚合状态表,含色标状态胶囊。 |
| `/api/status` | GET | 同一快照的 JSON 版本,供脚本/集成使用。 |

JSON 载荷示例:

```json
{
  "polled_at": "2026-07-19T09:45:14Z",
  "instances": [
    {
      "base": "http://10.0.0.1:9370",
      "health": "UP",
      "readiness": "UP",
      "startup": "UP",
      "components": [{ "name": "mysql:orders", "status": "UP" }],
      "go": "go1.26",
      "module": "example.com/orders",
      "version": "v1.2.3",
      "revision": "deadbeef",
      "build_time": "2026-07-19T00:00:00Z"
    }
  ]
}
```

## 工作原理

- 单个后台协程按 `spring.admin-ui.interval` 触发轮询。
- 每轮对全部实例并发发起请求,每次 HTTP 调用受 `spring.admin-ui.timeout` 约束。
- 结果作为快照写入 `RWMutex` 保护的字段;HTTP handler 只读快照,永远不阻塞
  在实时轮询上,页面加载时长与实例健康无关。
- 不可达实例显示为 `DOWN` 并附带底层错误信息,不会把看板本身搞挂。
- `/readiness` 组件异常时返回 `503`——轮询保留响应体,正确渲染 `OUT_OF_SERVICE`
  或 `DOWN` 状态。

## 优雅关停

本 starter 参与框架的 server 生命周期:`Stop()` 关闭轮询协程的停止 channel,
等待协程退出,再调用 `http.Server.Shutdown`。没有 `PreStop` 钩子——看板是运维
工具,不是被探针目标,drain 时无需翻转状态。

## 配置项

| 属性 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.admin-ui.enabled` | `true` | 是否启用 admin-ui server。 |
| `spring.admin-ui.addr` | `:9280` | 监听地址。与主 HTTP server(`:9090`)、actuator(`:9370`)、pprof(`127.0.0.1:9981`)刻意错开。 |
| `spring.admin-ui.instances` | *(空)* | 要轮询的 actuator base URL 列表。 |
| `spring.admin-ui.interval` | `10s` | 轮询周期,同时驱动页面自动刷新。 |
| `spring.admin-ui.timeout` | `3s` | 单个实例单个端点的 HTTP 超时。 |
| `spring.admin-ui.title` | `Go-Spring Admin` | 看板标题,便于给不同环境的看板打上环境名。 |

## License

本项目基于 Apache License 2.0 开源。

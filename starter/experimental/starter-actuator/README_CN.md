# starter-actuator

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

`starter-actuator` 在一个独立的管理端口上暴露运维 HTTP 端点——健康探针、构建信息，
以及运行时自省（日志器、环境属性、线程转储），由 Go-Spring IoC 容器统一管理。它为
Go-Spring 应用补齐了 Kubernetes 探针、注册中心健康检查、运维巡检所需的入口。

与应用的主 HTTP 服务器不同，actuator 在监听器绑定后**立即**开始提供服务。这是有意为之：
就绪探针必须能在应用尚未就绪时就访问到端点，从而观察到 `OUT_OF_SERVICE` → `UP` 的
状态切换；存活探针也必须在漫长的启动过程中始终应答，以免 Pod 被过早重启。

## 安装

```bash
go get go-spring.org/starter-actuator
```

## 快速开始

### 1. 引入 `starter-actuator` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-actuator"
```

### 2. 配置 Actuator 服务器

在项目的[配置文件](example/conf/app.properties)中添加：

```properties
spring.actuator.addr=:9370
# 可选：给整个管理端口加鉴权（bearer token，或 spring.actuator.username/password
# 的 HTTP Basic）。都不配且监听非 loopback 地址时，启动打 WARN。
# spring.actuator.token=s3cret
```

### 3. 访问端点

```bash
curl http://127.0.0.1:9370/healthz     # 存活
curl http://127.0.0.1:9370/readyz      # 就绪（聚合健康指示器）
curl http://127.0.0.1:9370/startupz    # 启动探针（启动完成前 503，之后 200）
curl http://127.0.0.1:9370/info        # 构建/版本信息
curl http://127.0.0.1:9370/loggers     # 已配置的日志器及其级别
curl http://127.0.0.1:9370/env         # 按来源分组的配置（未合并，敏感值脱敏）
curl http://127.0.0.1:9370/threaddump  # goroutine 栈转储
```

旧路径 `/health`、`/readiness`、`/startup` 保留为 `/healthz`、`/readyz`、
`/startupz` 的别名。

将探针直接映射到 Kubernetes Pod：

```yaml
startupProbe:
  httpGet: { path: /startupz, port: 9370 }
livenessProbe:
  httpGet: { path: /healthz, port: 9370 }
readinessProbe:
  httpGet: { path: /readyz, port: 9370 }
```

## 端点

三个探针端点对应 Kubernetes 容器探针。带 z 后缀的路径为规范路径，旧名保留为别名。
探针端点与 `/info` 默认注册；敏感自省端点（`/loggers`、`/env`、`/configprops`、
`/threaddump`、`/beans`）**默认关闭**，只有列入 `spring.actuator.endpoints.include`
才注册。

| 端点 | 方法 | 含义 |
| --- | --- | --- |
| `/healthz`（别名 `/health`） | GET | **存活。** 只要进程在提供服务即返回 `200 {"status":"UP"}`。仅检查显式声明 `liveness` 分组的指示器（通常没有），因此依赖挂掉不会触发存活重启。 |
| `/readyz`（别名 `/readiness`） | GET | **就绪。** 仅当应用越过就绪屏障**且**所有 `readiness` 分组指示器均通过时返回 `200 {"status":"UP"}`；否则返回 `503`（就绪前及停机排空期间为 `OUT_OF_SERVICE`，组件失败时为 `DOWN`）。 |
| `/startupz`（别名 `/startup`） | GET | **启动探针。** 应用启动完成**且**所有 `startup` 分组指示器通过前返回 `503 OUT_OF_SERVICE`，之后返回 `200`。不受排空影响，启动成功后 kubelet 即移交存活探针，缓慢启动不会被杀掉。 |
| `/info` | GET | 从二进制内嵌的 build info 读取构建/版本元数据（模块路径/版本、Go 工具链，以及从代码库构建时的 VCS 版本/时间）。 |
| `/loggers` | GET | 列出已配置的日志器及其生效级别。只读（有意不实现运行时改级别，`POST /loggers/{name}` 不存在）。对标 Spring Boot 的 `/actuator/loggers`。默认关闭，需显式 include。 |
| `/env` | GET | 各配置源的扁平属性表，按优先级排列（高在前）、**未合并**——运维看到原始数据自行判断聚合结果。敏感命名的 key（`password`、`token`、`secret` 等）与 `ENC(...)` 值会被脱敏。 |
| `/configprops` | GET | 各配置源的嵌套树视图（对标 `/actuator/configprops`），按优先级排列、未合并，脱敏策略与 `/env` 相同。 |
| `/threaddump` | GET | 以 `text/plain` 返回 goroutine 栈转储——对标 JVM 的线程转储。 |
| `/metrics` | GET | Prometheus 抓取端点。仅当引入 `starter-otel` 且 `spring.observability.metrics.exporter=prometheus` 时出现——otel 贡献其抓取 handler，由 actuator 挂载于此（见*指标与 Kubernetes 抓取*）。 |

### 运行时日志级别

`GET /loggers` 列出每个已配置日志器及其生效级别。**只读**——运行时改级别
（`POST /loggers/{name}`）经设计权衡后未实现：

```json
{
  "loggers": { "root": { "configuredLevel": "INFO" } }
}
```

### 敏感值脱敏（`/env`、`/configprops`）

当 key 命中 `password`、`passwd`、`secret`、`token`、`credential`、
`apikey`/`api-key`、`private-key`、`access-key`（不区分大小写子串），或 key 最后
一个 `.`/`_`/`-` 分段恰好是 `key`/`api-key`/`api_key`（`some.key`、`aws.key` 命中；
`monkey`、`keyword` 不命中），或值为配置加密产生的 `ENC(...)` 占位符时，值会被脱敏
为 `******`；值若是内嵌凭据的 URL（`redis://user:pass@host`）则只把 userinfo 部分
掩为 `redis://******@host`。其余值原样输出。

## 优雅停机（Drain）

收到 `SIGTERM` 时，actuator 将 `/readyz` 翻转为 `503 OUT_OF_SERVICE`（通过
`PreStop` 钩子），而 `/healthz` 与在途请求保持正常。各 server 各自掌握自己的
排空时序——在自身 `PreStop` / `StopContext` 允许的窗口内完成在途请求并停止自身——
让 Kubernetes 端点控制器有时间把 Pod 从 Service endpoints 中摘除，之后才停止接收
新流量。这正是滚动更新做到无损的关键。框架不再提供排空延迟或停机超时：排空与停机
的边界是 server 自身的职责。

## 健康指示器

探针会聚合由其它 bean 贡献的健康检查。任何被导出为 `health.Indicator`
（来自零依赖的 `go-spring.org/cloud/actuator/health` 包）的 bean 都会被自动收集——无需任何
逐组件的注册 API，也无需 import 本 starter：

```go
import "go-spring.org/cloud/actuator/health"

type dbHealth struct{ db *sql.DB }

func (h *dbHealth) HealthName() string                    { return "mysql:orders" }
func (h *dbHealth) CheckHealth(ctx context.Context) error { return h.db.PingContext(ctx) }

// 注册为导出 health.Indicator 的 bean：
gs.Provide(&dbHealth{db}).Export(gs.As[health.Indicator]())
```

指示器默认贡献到 `readiness` 与 `startup` 两个分组（**绝不**进入 `liveness`，因此
依赖检查永远无法触发 Pod 重启）。若指示器实现可选的 `health.Grouped` 接口，可自行
覆盖所属探针分组：

```go
func (h *dbHealth) HealthGroups() []health.Group {
    return []health.Group{health.GroupReadiness}
}
```

失败的组件会列在 `/readyz` 响应的 `components` 字段下，便于定位探针失败原因：

```json
{
  "status": "DOWN",
  "components": {
    "mysql:orders": { "status": "DOWN", "error": "dial tcp ...: connection refused" }
  }
}
```

自带健康指示器的客户端 starter（如 `starter-go-redis`）在两个 starter 同时被引入时
会被自动纳入。

## 指标与 Kubernetes 抓取

actuator 还能承载 Prometheus `/metrics` 端点，让运维方**只抓一个管理端口**即可同时拿到
探针与指标，而无需指标 exporter 另起一个服务器。这一能力通过 `starter-otel` 开启：任何
被导出为 `endpoint.Endpoint`（来自零依赖的 `go-spring.org/cloud/actuator/endpoint` 包）的 bean
都会被挂载到管理端口，而 `starter-otel` 的 Prometheus exporter 恰好贡献了这样一个 bean
——本 starter 不 import otel，也无需额外接线：

```go
import (
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-otel"
)
```

```properties
# 仅通过 actuator 暴露 /metrics（不再另起专用指标服务器）：
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
spring.observability.metrics.path=/metrics
```

```bash
curl http://127.0.0.1:9370/metrics
```

### Pod 注解抓取

对使用 Pod 注解发现的 Prometheus，把它指向管理端口即可：

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9370"
    prometheus.io/path: "/metrics"
```

### ServiceMonitor（Prometheus Operator）

在 Service 上暴露管理端口，再用 `ServiceMonitor` 选中它：

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
    - port: management   # 映射到 9370 的 Service 端口
      path: /metrics
      interval: 15s
```

## 配置

| 属性 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.actuator.addr` | — | 管理端监听地址（必填——设置该 key 即启用 starter）。示例 `:9370` 绑定所有网卡以便集群内探针访问。与主 HTTP 服务器（`:9090`）、pprof 服务器（`127.0.0.1:9981`）区分开。 |
| `spring.actuator.endpoints.include` | `""` | 逗号列表。敏感自省端点（`loggers、env、configprops、threaddump、beans`）默认关闭，必须列在这里才暴露；非空列表同时对 `info` 与贡献端点（如 `metrics`）构成白名单。探针豁免。 |
| `spring.actuator.endpoints.exclude` | `""` | 逗号黑名单，始终生效——显式 include 也会被压掉。 |
| `spring.actuator.token` | `""` | 保护整个管理端口的 bearer token（`Authorization: Bearer <token>`），优先于 Basic。 |
| `spring.actuator.username` / `spring.actuator.password` | `""` | 管理端口的 HTTP Basic 凭据（两者都设才生效）。都不配且非 loopback 监听时启动打 WARN。 |

## 许可证

本项目基于 Apache License 2.0 许可证。

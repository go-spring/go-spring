# starter-actuator

[English](README.md) | [中文](README_CN.md)

> 项目已正式发布，欢迎使用！

`starter-actuator` 在一个独立的管理端口上暴露运维 HTTP 端点——存活（liveness）、
就绪（readiness）与构建信息（info），由 Go-Spring IoC 容器统一管理。它为 Go-Spring
应用补齐了 Kubernetes 探针、注册中心健康检查、运维巡检所需的入口。

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
spring.actuator.enabled=true
spring.actuator.addr=:9370
```

### 3. 访问端点

```bash
curl http://127.0.0.1:9370/health      # 存活
curl http://127.0.0.1:9370/readiness   # 就绪（聚合健康指示器）
curl http://127.0.0.1:9370/startup     # 启动探针（启动完成前 503，之后 200）
curl http://127.0.0.1:9370/info        # 构建/版本信息
```

直接映射到 Kubernetes Pod：

```yaml
startupProbe:
  httpGet: { path: /startup, port: 9370 }
livenessProbe:
  httpGet: { path: /health, port: 9370 }
readinessProbe:
  httpGet: { path: /readiness, port: 9370 }
```

## 端点

| 端点 | 方法 | 含义 |
| --- | --- | --- |
| `/health` | GET | 存活。进程开始提供服务后返回 `200 {"status":"UP"}`。它只反映进程是否存活，**不**反映依赖健康——依赖挂掉不会触发存活探针重启。 |
| `/readiness` | GET | 就绪。仅当应用越过就绪屏障**且**所有已注册的健康指示器均通过时返回 `200 {"status":"UP"}`；否则返回 `503`（就绪前及停机排空期间为 `OUT_OF_SERVICE`，组件失败时为 `DOWN`）。 |
| `/startup` | GET | 启动探针。应用启动完成前返回 `503 OUT_OF_SERVICE`，完成后返回 `200 {"status":"UP"}`。与 `/readiness` 不同，它忽略健康指示器——唯一职责是告诉 kubelet 启动已完成，从而让缓慢启动不被存活探针杀掉。 |
| `/info` | GET | 从二进制内嵌的 build info 读取构建/版本元数据（模块路径/版本、Go 工具链，以及从代码库构建时的 VCS 版本/时间）。 |
| `/metrics` | GET | Prometheus 抓取端点。仅当引入 `starter-otel` 且 `spring.observability.metrics.exporter=prometheus` 时出现——otel 贡献其抓取 handler，由 actuator 挂载于此（见*指标与 Kubernetes 抓取*）。 |

## 优雅停机（Drain）

收到 `SIGTERM` 时，框架在停止服务器前先执行一段排空流程：actuator 将 `/readiness`
翻转为 `503 OUT_OF_SERVICE`（通过 `PreStop` 钩子），而 `/health` 与在途请求保持正常；
随后等待 `app.shutdown.pre-stop-delay`，让 Kubernetes 端点控制器有时间把 Pod 从
Service endpoints 中摘除，之后才停止接收新流量。这正是滚动更新做到无损的关键。之后
才停止服务器，其耗时由 `app.shutdown.timeout` 限定。

```properties
# 就绪翻转为 false 后，停止服务器前等待这么久。
app.shutdown.pre-stop-delay=5s
# 可选，限定等待服务器停止的时长上限（0 = 无限等待）。
app.shutdown.timeout=30s
```

两项都是框架级配置（作用于所有服务器，而非仅 actuator），默认均为 `0`，即禁用排空
等待、保持立即停机的行为。

## 健康指示器

`/readiness` 会聚合由其它 bean 贡献的健康检查。任何被导出为 `health.Indicator`
（来自零依赖的 `go-spring.org/stdlib/health` 包）的 bean 都会被自动收集——无需任何
逐组件的注册 API，也无需 import 本 starter：

```go
import "go-spring.org/stdlib/health"

type dbHealth struct{ db *sql.DB }

func (h *dbHealth) HealthName() string                    { return "mysql:orders" }
func (h *dbHealth) CheckHealth(ctx context.Context) error { return h.db.PingContext(ctx) }

// 注册为导出 health.Indicator 的 bean：
gs.Provide(&dbHealth{db}).Export(gs.As[health.Indicator]())
```

失败的组件会列在 `/readiness` 响应的 `components` 字段下，便于定位探针失败原因：

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
被导出为 `endpoint.Endpoint`（来自零依赖的 `go-spring.org/stdlib/endpoint` 包）的 bean
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
| `spring.actuator.enabled` | `true` | 启用/禁用 actuator 服务器。 |
| `spring.actuator.addr` | `:9370` | 管理端监听地址。默认绑定所有网卡，以便集群内探针可访问。与主 HTTP 服务器（`:9090`）、pprof 服务器（`127.0.0.1:9981`）区分开。 |

## 许可证

本项目基于 Apache License 2.0 许可证。

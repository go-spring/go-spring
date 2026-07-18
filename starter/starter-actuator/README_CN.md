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
curl http://127.0.0.1:9370/info        # 构建/版本信息
```

直接映射到 Kubernetes Pod：

```yaml
livenessProbe:
  httpGet: { path: /health, port: 9370 }
readinessProbe:
  httpGet: { path: /readiness, port: 9370 }
```

## 端点

| 端点 | 方法 | 含义 |
| --- | --- | --- |
| `/health` | GET | 存活。进程开始提供服务后返回 `200 {"status":"UP"}`。它只反映进程是否存活，**不**反映依赖健康——依赖挂掉不会触发存活探针重启。 |
| `/readiness` | GET | 就绪。仅当应用越过就绪屏障**且**所有已注册的健康指示器均通过时返回 `200 {"status":"UP"}`；否则返回 `503`（就绪前为 `OUT_OF_SERVICE`，组件失败时为 `DOWN`）。 |
| `/info` | GET | 从二进制内嵌的 build info 读取构建/版本元数据（模块路径/版本、Go 工具链，以及从代码库构建时的 VCS 版本/时间）。 |

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

## 配置

| 属性 | 默认值 | 说明 |
| --- | --- | --- |
| `spring.actuator.enabled` | `true` | 启用/禁用 actuator 服务器。 |
| `spring.actuator.addr` | `:9370` | 管理端监听地址。默认绑定所有网卡，以便集群内探针可访问。与主 HTTP 服务器（`:9090`）、pprof 服务器（`127.0.0.1:9981`）区分开。 |

## 许可证

本项目基于 Apache License 2.0 许可证。

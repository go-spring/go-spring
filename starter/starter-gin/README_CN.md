# starter-gin

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-gin` 将 [gin-gonic/gin](https://github.com/gin-gonic/gin) Web 框架接入 Go-Spring。
`*gin.Engine` 及其 HTTP 服务器由 starter 依据配置创建并持有，应用只需提供一个 `RouterRegister` Bean
来挂载路由与中间件，整体通过 Go-Spring 的服务器生命周期对外提供服务。

## 安装

```bash
go get go-spring.org/starter-gin
```

## 快速开始

### 1. 引入 `starter-gin` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-gin"
```

### 2. 配置 Gin 服务器

在项目的[配置文件](example/conf/app.properties)中添加配置，比如：

```properties
# 让 gin 独占 HTTP 端口，关闭 Go-Spring 内置服务器。
spring.http.server.enabled=false
# 本示例中 starter-gin 默认监听 :8001。
spring.gin.server.addr=:8001

# 超时（继承自 SimpleHttpServerConfig）。
spring.gin.server.readTimeout=5s
spring.gin.server.headerTimeout=1s
spring.gin.server.writeTimeout=5s
spring.gin.server.idleTimeout=60s

# 请求体大小上限（字节，0 表示不限制）。
spring.gin.server.maxBodySize=1048576

# 可选的、由 starter 提供的存活探针端点。
spring.gin.server.health.enabled=true
spring.gin.server.health.path=/healthz

# HTTPS：启用并指定 PEM 证书/私钥路径。
spring.gin.server.tls.enabled=false
spring.gin.server.tls.cert-file=
spring.gin.server.tls.key-file=

# 内置中间件。Recovery、Tracing、Metrics、AccessLog 合并进常开的 Observe 中间件（无开关）；
# RequestID 默认开启；CORS、Gzip、SecureHeaders 默认关闭，按需开启（见"内置中间件"）。
spring.gin.server.middleware.requestId.enabled=true
spring.gin.server.middleware.requestId.header=X-Request-Id
spring.gin.server.middleware.accessLog.skipPaths=
spring.gin.server.middleware.accessLog.payload.enabled=true
spring.gin.server.middleware.cors.enabled=false
spring.gin.server.middleware.cors.allowedOrigins=
spring.gin.server.middleware.gzip.enabled=false
spring.gin.server.middleware.gzip.level=5
spring.gin.server.middleware.secureHeaders.enabled=false
```

当 `spring.gin.server.enabled` 为 `true`（默认）且应用提供了 `RouterRegister` Bean 时，
starter 会自动注册服务器 Bean。

> **端口约定** —— 三个 HTTP starter 使用互不相同的端口，可同时启动：
> `starter-gin` → `:8001`，`starter-echo` → `:8002`，`starter-hertz` → `:8003`。

### 3. 提供 `RouterRegister` Bean

starter 负责创建并配置 `*gin.Engine`（release 模式，并安装下列内置中间件），再交给你的注册器。
在其中挂载路由与中间件即可。参见 [example.go](example/example.go) 文件。

```go
gs.Provide(func(c *Controller) StarterGin.RouterRegister {
    return func(e *gin.Engine) {
        e.GET("/echo/:name", c.Echo)
    }
})
```

## 核心功能

[example](example/example.go) 通过真实 HTTP 请求端到端演示了三项能力：

* **中间件**：starter 默认安装 Recovery、Tracing、Metrics、AccessLog（合并进一个 Observe 中间件，外加 RequestID 与可选的 CORS/Gzip/SecureHeaders）；注册器再加一个自定义中间件，会在每个响应上写入
  `X-App: go-spring` 头。
* **路径参数 + JSON 响应**：`GET /echo/:name` 通过 `ctx.Param` 与 `ctx.JSON` 返回
  `{"message":"Hello, <name>"}`。
* **查询参数**：`GET /greet?name=...` 通过 `ctx.Query("name")` 读取参数并返回
  `{"message":"Hi, <name>"}` JSON。

## 内置中间件

starter 在应用的 `RouterRegister` 执行**之前**，按固定顺序在 `*gin.Engine` 上安装一组横切中间件，
因此它们会包裹所有路由。Recovery、Tracing、Metrics、AccessLog 为必选且常开（合并进一个 `Observe` 中间件，无开关）；RequestID 默认开启；其余默认关闭，按需通过 `spring.gin.server.middleware.*` 开启。

| 中间件 | 默认 | 来源 | 说明 |
|---|---|---|---|
| `observe` | 常开（必选） | 自实现 | 将 Recovery、Tracing、Metrics、AccessLog 合并进单一请求生命周期：一个 defer 同时结束 span、记录指标、输出访问日志，即使 handler panic 也能正确收尾。Tracing/Metrics 搭载 OTel 全局对象（未引入 `starter-otel` 时为 no-op）；访问日志 4xx 记 Warn、5xx 记 Error，自动跳过健康端点，携带 `request_id`/`trace_id`/`span_id`，并通过 `accessLog.payload.enabled`（默认开）抓取请求 body+query+headers 与响应 body，各 512 KiB 截断，二进制内容打占位符。SSE 响应（`text/event-stream`）按每次 flush 实时逐事件落盘，不缓冲到流关闭。 |
| `requestId` | 开 | `gin-contrib/requestid` | 生成/透传 `X-Request-Id`，同时写入请求 context（见 `RequestIDFromContext`）。 |
| `cors` | 关 | `gin-contrib/cors` | 没有安全的通用默认值，需显式配置 `allowedOrigins`（或开发期用 `allowAllOrigins`）。配置非法会在启动期失败。 |
| `gzip` | 关 | `gin-contrib/gzip` | `level`（1-9，-1=默认）、`minLength`（0=全部压缩）。 |
| `secureHeaders` | 关 | 自实现 | `X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`；HSTS 仅在启用 TLS 时生效。 |
| 请求体限制 | `maxBodySize>0` 时开 | 自实现 | 位于链内，超限的 413 会像普通响应一样被记录。 |

顺序（最外层在前）：`Observe -> RequestID -> SecureHeaders -> CORS -> Gzip -> BodyLimit`。
Observe 在最外层，由其单一 defer 兜住后续所有层的 panic 并统一收尾 span、指标与访问日志；RequestID 位于 Observe 内部，使每条访问日志都带上请求 id 并落在被 recover 的 span 之内；策略类中间件位于链内，使短路响应（413、204、403）也能被观测到。

> **设计上不提供请求超时中间件。** Go 无法在不使用 goroutine 缓冲 hack（会破坏流式/SSE）的前提下
> 抢占正在运行的 handler，因此硬性时限仍由 `SimpleHttpServerConfig` 中 `http.Server` 的读写超时兜底。

若要把请求 id 带到业务日志，配置一次 log 包的 context 钩子即可：

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
    if rid := StarterGin.RequestIDFromContext(ctx); rid != "" {
        return []log.Field{log.String("request_id", rid)}
    }
    return nil
}
```

## 高级功能

* **自定义服务器配置**：通过 `spring.gin.server.*`（监听地址、TLS、超时等）绑定 `SimpleHttpServerConfig`
  进行调优。
* **完整的 gin 生态**：任何 gin 中间件、路由分组、渲染器、绑定器都可以在注册器拿到的 `*gin.Engine`
  上自由组合。

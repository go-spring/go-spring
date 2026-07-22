# starter-hertz

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-hertz` 将 [CloudWeGo Hertz](https://github.com/cloudwego/hertz) HTTP
框架接入 Go-Spring 的服务生命周期。`*server.Hertz` 及其监听器由 starter 依据配置
创建并持有，应用只需提供一个 `RouterRegister` Bean 来挂载路由与中间件：容器就绪后
再启动 Hertz，应用退出时优雅关闭。

## 安装

```bash
go get go-spring.org/starter-hertz
```

## 快速开始

### 1. 引入 `starter-hertz` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-hertz"
```

### 2. 提供 `RouterRegister` Bean

starter 会依据配置在指定地址创建 `*server.Hertz` 并交给你的注册器，你在其中挂载
中间件与路由即可。参见 [example.go](example/example.go) 文件。

```go
gs.Provide(func(c *Controller) StarterHertz.RouterRegister {
    return func(h *server.Hertz) {
        h.Use(func(ctx context.Context, r *app.RequestContext) {
            r.Response.Header.Set("X-App", "go-spring")
            r.Next(ctx)
        })
        h.GET("/echo/:name", c.Echo)
        h.GET("/greet", c.Greet)
    }
})
```

> **端口约定** —— 三个 HTTP starter 使用互不相同的端口，可同时启动：
> `starter-gin` → `:8001`，`starter-echo` → `:8002`，`starter-hertz` → `:8003`。
> Hertz 虽自己管理监听器，但地址仍从 `spring.hertz.server.addr` 读取，由 starter
> 通过 `WithHostPorts` 传入 engine。

### 3. 配置地址并关闭内置 HTTP 服务

在 [app.properties](example/conf/app.properties) 中设置 Hertz 监听地址，并关闭
Go-Spring 内置 HTTP 服务（Hertz 自己管理监听器）：

```properties
spring.http.server.enabled=false
spring.hertz.server.addr=127.0.0.1:8003

# 超时（命名对齐 SimpleHttpServerConfig，通过 Hertz 选项应用）。
spring.hertz.server.readTimeout=5s
spring.hertz.server.writeTimeout=5s
spring.hertz.server.idleTimeout=60s

# 请求体大小上限（字节，0 表示使用 Hertz 默认值）。
spring.hertz.server.maxBodySize=1048576

# 可选的、由 starter 提供的存活探针端点。
spring.hertz.server.health.enabled=true
spring.hertz.server.health.path=/healthz

# HTTPS：启用并指定 PEM 证书/私钥路径。
spring.hertz.server.tls.enabled=false
spring.hertz.server.tls.cert-file=
spring.hertz.server.tls.key-file=

# 内置中间件。Recovery、RequestID、AccessLog 默认开启；
# CORS、Gzip、SecureHeaders 默认关闭，按需开启（见"内置中间件"）。
spring.hertz.server.middleware.recovery.enabled=true
spring.hertz.server.middleware.requestId.enabled=true
spring.hertz.server.middleware.accessLog.enabled=true
spring.hertz.server.middleware.accessLog.skipPaths=
spring.hertz.server.middleware.cors.enabled=false
spring.hertz.server.middleware.cors.allowedOrigins=
spring.hertz.server.middleware.gzip.enabled=false
spring.hertz.server.middleware.gzip.level=5
spring.hertz.server.middleware.secureHeaders.enabled=false
```

### 4. 运行应用

```go
func main() {
    gs.Run()
}
```

## 核心功能

[example.go](example/example.go) 演示了三个核心 Hertz 特性，`runTest` 通过真实
HTTP 请求逐个断言：

* **中间件（Middleware）**：starter 默认安装 Recovery、RequestID、AccessLog；通过 `h.Use(...)` 注册的中间件为每个响应写入
  `X-App: go-spring` 响应头；测试用例校验该 header 是否回传。
* **路径参数 + JSON**：`GET /echo/:name` 从 `c.Param("name")` 读取路径参数，
  返回 `{"message":"Hello, <name>"}`；测试请求 `/echo/hertz` 并断言
  `message == "Hello, hertz"`。
* **查询参数 + JSON**：`GET /greet` 从 `c.Query("name")` 读取查询参数，
  返回 `{"message":"Hi, <name>"}`；测试请求 `/greet?name=world` 并断言
  `message == "Hi, world"`。

## 内置中间件

starter 在应用的 `RouterRegister` 执行**之前**，按固定顺序在 `*server.Hertz` 上安装一组横切中间件，
因此它们会包裹所有路由。每个中间件均可通过 `spring.hertz.server.middleware.*` 独立开关。Recovery 来自
hertz core；RequestID/CORS/Gzip 来自 hertz-contrib；AccessLog 与 SecureHeaders 为自实现。

| 中间件 | 默认 | 来源 | 说明 |
|---|---|---|---|
| `recovery` | 开 | core `recovery.Recovery()` | 捕获请求 goroutine 的 panic；关闭可能导致进程崩溃。starter 用 `server.New`（而非 `server.Default`）使其可配置。 |
| `requestId` | 开 | hertz-contrib/requestid | 生成/透传 `X-Request-Id`，同时写入请求 context（见 `RequestIDFromContext`）。 |
| `accessLog` | 开 | 自实现（项目 `log` 包） | 每个请求一条结构化访问日志；4xx 记 Warn、5xx 记 Error；健康端点路径自动跳过。 |
| `cors` | 关 | hertz-contrib/cors | 没有安全的通用默认值，需显式配置 `allowedOrigins`（或开发期用 `allowAllOrigins`）。配置非法会在启动期失败。 |
| `gzip` | 关 | hertz-contrib/gzip | `level`（1-9，-1=默认）。 |
| `secureHeaders` | 关 | 自实现 | `X-Content-Type-Options`/`X-Frame-Options`/`Referrer-Policy`；HSTS 仅在启用 TLS 时生效。（hertz-contrib/secure 默认带 10 年 HSTS + SSL 重定向，故刻意不用。） |
| 请求体限制 | `maxBodySize>0` 时开 | 引擎选项 `WithMaxRequestBodySize` | 非中间件；超限的 413 会像普通响应一样被记录。 |

顺序（最外层在前）：`Recovery -> RequestID -> AccessLog -> SecureHeaders -> CORS -> Gzip`。
Recovery 在最外层以兜住后续所有层的 panic；RequestID 在 AccessLog 之前，使每条访问日志都带上请求 id；
AccessLog 包裹策略类中间件，使短路响应（204、403）也能被记录。

> **设计上不提供请求超时中间件。** Go 无法在不使用 goroutine 缓冲 hack（会破坏流式/SSE）的前提下
> 抢占正在运行的 handler，因此硬性时限仍由 `${spring.hertz.server}` 的读写超时兜底。
> 指标与链路追踪同样不内置--请使用 `starter-actuator` 与 `starter-otel`。

若要把请求 id 带到业务日志，配置一次 log 包的 context 钩子即可：

```go
log.FieldsFromContext = func(ctx context.Context) []log.Field {
    if rid := StarterHertz.RequestIDFromContext(ctx); rid != "" {
        return []log.Field{log.String("request_id", rid)}
    }
    return nil
}
```

## 高级功能

* **框架托管 engine**：starter 依据配置创建 `*server.Hertz` 并交给你的
  `RouterRegister`，Hertz 的 TLS、Tracer、自定义 transport 等选项
  统一生效。
* **托管的生命周期**：适配器会等待 Go-Spring 就绪信号后再调用 `h.Run()`，
  在关闭阶段调用 `h.Shutdown(ctx)`。
* **可选的注册开关**：将 `spring.hertz.server.enabled=false` 可以关闭自动注册
  （默认在存在 `RouterRegister` Bean 时开启）。

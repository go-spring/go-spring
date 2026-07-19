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
```

当 `spring.gin.server.enabled` 为 `true`（默认）且应用提供了 `RouterRegister` Bean 时，
starter 会自动注册服务器 Bean。

> **端口约定** —— 三个 HTTP starter 使用互不相同的端口，可同时启动：
> `starter-gin` → `:8001`，`starter-echo` → `:8002`，`starter-hertz` → `:8003`。

### 3. 提供 `RouterRegister` Bean

starter 负责创建并配置 `*gin.Engine`（release 模式、`gin.Recovery()`），再交给你的注册器。
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

* **中间件**：starter 已安装 `gin.Recovery()`；注册器再加一个自定义中间件，会在每个响应上写入
  `X-App: go-spring` 头。
* **路径参数 + JSON 响应**：`GET /echo/:name` 通过 `ctx.Param` 与 `ctx.JSON` 返回
  `{"message":"Hello, <name>"}`。
* **查询参数**：`GET /greet?name=...` 通过 `ctx.Query("name")` 读取参数并返回
  `{"message":"Hi, <name>"}` JSON。

## 高级功能

* **自定义服务器配置**：通过 `spring.gin.server.*`（监听地址、TLS、超时等）绑定 `SimpleHttpServerConfig`
  进行调优。
* **完整的 gin 生态**：任何 gin 中间件、路由分组、渲染器、绑定器都可以在注册器拿到的 `*gin.Engine`
  上自由组合。

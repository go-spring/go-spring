# starter-echo

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-echo` 将 [labstack/echo](https://github.com/labstack/echo) Web 框架接入 Go-Spring，
使应用提供的 `*echo.Echo` Bean 能够通过 Go-Spring 的服务器生命周期对外提供服务。

## 安装

```bash
go get go-spring.org/starter-echo
```

## 快速开始

### 1. 引入 `starter-echo` 包

参见 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-echo"
```

### 2. 配置 Echo 服务器

在项目的[配置文件](example/conf/app.properties)中添加配置，比如：

```properties
# 让 echo 独占 HTTP 端口，关闭 Go-Spring 内置服务器。
spring.http.server.enabled=false
```

当 `spring.echo.server.enabled` 为 `true`（默认）且应用提供了 `*echo.Echo` Bean 时，
starter 会自动注册服务器 Bean。

### 3. 提供 `*echo.Echo` Bean

参见 [example.go](example/example.go) 文件。

```go
gs.Provide(func(c *Controller) *echo.Echo {
    e := echo.New()
    e.HideBanner = true
    e.Use(middleware.Recover())
    e.GET("/echo/:name", c.Echo)
    return e
})
```

## 核心功能

[example](example/example.go) 通过真实 HTTP 请求端到端演示了三项能力：

* **中间件**：`middleware.Recover()` 加一个自定义中间件，会在每个响应上写入 `X-App: go-spring` 头。
* **路径参数 + JSON 响应**：`GET /echo/:name` 通过 `ctx.Param` 与 `ctx.JSON` 返回
  `{"message":"Hello, <name>"}`。
* **路由分组**：`e.Group("/api")` 挂载 `GET /api/greet?name=...`，从 query 中取值并返回
  `{"message":"Hi, <name>"}`。

## 高级功能

* **自定义服务器配置**：通过 `spring.echo.server.*`（监听地址、TLS、超时等）绑定 `SimpleHttpServerConfig`
  进行调优。
* **完整的 echo 生态**：任何 echo 中间件、路由分组、渲染器、绑定器都可以在 `*echo.Echo` Bean 交给 starter
  之前自由组合。

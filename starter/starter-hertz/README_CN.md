# starter-hertz

[English](README.md) | [中文](README_CN.md)

> 该项目已经正式发布，欢迎使用！

`starter-hertz` 将 [CloudWeGo Hertz](https://github.com/cloudwego/hertz) HTTP
框架接入 Go-Spring 的服务生命周期：容器就绪后再启动 Hertz，应用退出时优雅关闭。

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

### 2. 提供 `*server.Hertz` 实例

Hertz 实例（监听地址、中间件、路由）由应用自己创建；starter 只负责在容器就绪
后调用 `Run`，在退出时调用 `Shutdown`。参见 [example.go](example/example.go) 文件。

```go
gs.Provide(func(c *Controller) *server.Hertz {
    h := server.Default(server.WithHostPorts("127.0.0.1:9090"))
    h.Use(func(ctx context.Context, r *app.RequestContext) {
        r.Response.Header.Set("X-App", "go-spring")
        r.Next(ctx)
    })
    h.GET("/echo/:name", c.Echo)
    h.GET("/greet", c.Greet)
    return h
})
```

### 3. 关闭内置 HTTP 服务

Hertz 会自己管理监听器，因此需要在 [app.properties](example/conf/app.properties)
中关闭 Go-Spring 内置的 HTTP 服务：

```properties
spring.http.server.enabled=false
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

* **中间件（Middleware）**：通过 `h.Use(...)` 注册的中间件为每个响应写入
  `X-App: go-spring` 响应头；测试用例校验该 header 是否回传。
* **路径参数 + JSON**：`GET /echo/:name` 从 `c.Param("name")` 读取路径参数，
  返回 `{"message":"Hello, <name>"}`；测试请求 `/echo/hertz` 并断言
  `message == "Hello, hertz"`。
* **查询参数 + JSON**：`GET /greet` 从 `c.Query("name")` 读取查询参数，
  返回 `{"message":"Hi, <name>"}`；测试请求 `/greet?name=world` 并断言
  `message == "Hi, world"`。

## 高级功能

* **自定义 Hertz 实例**：starter 不会替你创建 `*server.Hertz`，因此 TLS、
  Tracer、自定义 transport 等任意 Hertz 选项都可以照常使用。
* **托管的生命周期**：适配器会等待 Go-Spring 就绪信号后再调用 `h.Run()`，
  在关闭阶段调用 `h.Shutdown(ctx)`。
* **可选的注册开关**：将 `spring.hertz.server.enabled=false` 可以关闭自动注册
  （默认在存在 `*server.Hertz` Bean 时开启）。

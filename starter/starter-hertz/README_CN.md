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

* **框架托管 engine**：starter 依据配置创建 `*server.Hertz` 并交给你的
  `RouterRegister`，`server.Default` 附带的 TLS、Tracer、自定义 transport 等选项
  统一生效。
* **托管的生命周期**：适配器会等待 Go-Spring 就绪信号后再调用 `h.Run()`，
  在关闭阶段调用 `h.Shutdown(ctx)`。
* **可选的注册开关**：将 `spring.hertz.server.enabled=false` 可以关闭自动注册
  （默认在存在 `RouterRegister` Bean 时开启）。

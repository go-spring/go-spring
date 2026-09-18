# starter-http-server
[English](README.md) | [中文](README_CN.md)

框架内置 stdlib HTTP 服务器(`gs.SimpleHttpServer`,由 `spring.http.server.*`
配置)的安全中间件套件。它不提供 server、也不注册 bean——只提供构建在
[`cloud/security`](../../cloud/security) 共享身份模型之上的 `net/http` 装饰器
家族。

| 中间件 | 作用 |
| --- | --- |
| `Chain(ms...)` | 组合中间件,最外层优先 |
| `Authenticate(v, required)` | 经 `security.TokenValidator` 校验 bearer token,把 `Authentication` 挂到请求 context |
| `Authorize(authorities...)` | 按权限守卫路由(匿名 401 / 缺权限 403) |
| `CORS(cfg)` | 添加 `Access-Control-Allow-*` 响应头,应答 preflight |
| `CSRF(cfg)` | 面向浏览器流的 double-submit-cookie CSRF 防护 |
| `Observe()` | 每请求一个 span、请求指标与访问日志 —— 可选启用,见「可观测」 |

## 快速开始

```go
import (
    "net/http"

    "go-spring.org/cloud/security"
    "go-spring.org/spring/gs"
    httpsvr "go-spring.org/starter-http-server"
)

gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle("/api/me", httpsvr.Chain(
        httpsvr.Authenticate(v, true),
        httpsvr.Authorize("orders:read"),
    )(http.HandlerFunc(meHandler)))
    return &gs.HttpServeMux{Handler: mux}
})
```

服务方法级校验用核心包的 `security.Require`。这些中间件的 gin / echo 等价物
见 `starter-gin`、`starter-echo`。可运行、自校验的示例在
[example](example/example.go)。

## 可观测

`Observe()` 是服务端可观测中间件。它是**可选启用**的,这与 gin/echo/hertz 三个 starter 的
内置插桩不同:本包装饰的是应用自己传给框架 server 的 handler,没有地方自我安装。按普通
装饰器组合即可:

```go
mux.Handle("/api/me", httpsvr.Chain(
    httpsvr.Observe(),
    httpsvr.CORS(cfg),
    httpsvr.Authenticate(v, true),
)(handler))
```

一次请求产出:

* 一个 server span,名为 `<METHOD> <path>`,带 `http.request.method`、`url.path`,退出时
  再带 `http.response.status_code` 与 `status`;
* `http.server.request.duration`(Float64Histogram,秒)与 `http.server.active_requests`
  (Int64UpDownCounter),label 为 `http.request.method` 与 `http.response.status_code`;
* 一行访问日志,tag `_app_http_server_access`
  (`log.RegisterAppTag("http_server", "access")`),带 `http.request.method`、`url.path`、
  `http.response.status_code`、`status` 与 `duration_ms`。5xx 为 Warn 行;span 与该行在
  `status` 上一致。

这些命名刻意与 HTTP 家族一致 —— 与 gin、echo、hertz 用的是同一套,因此应用在 stdlib server
与 gin 之间切换时看板不变。span 与指标走 `starter-otel` 安装的全局 OTel;未引入时它们是
no-op,只写访问日志。

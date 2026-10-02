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
| `ServerPolicy(label, mgr)` | 入站准入 —— 同时也是本请求的 span、指标与访问日志,见「可观测」 |

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
[example](example/main.go)。

## 可观测

一个请求的信号来自 `ServerPolicy` —— 与其它每个 starter 现在同一套分工:中间件**声明**这个请求是什么,resilience 发射点据此产出信号。没有别的要装。

(早先有一个独立、可选启用的 `Observe()` 中间件,自己起 span、自己建仪器。它已经删除:两个中间件各持一个 executor,等于两种被观测的方式、一种被准入的方式,而信号本来就归发射点。)

```go
mux.Handle("/api/me", httpsvr.Chain(
    httpsvr.ServerPolicy("http-server::9090", mgr),
    httpsvr.CORS(cfg),
    httpsvr.Authenticate(v, true),
)(handler))
```

一次请求产出:

* 一个 server span,名为 `<METHOD> <path>`,带 `http.request.method`、`url.path`,退出时
  再带 `http.response.status_code` 与 `status`;
* `http.server.request.duration`(Float64Histogram,秒)与 `http.server.active_requests`
  (Int64UpDownCounter),label 为 `http.request.method`、`http.response.status_code` 与 `status`;
* 一行访问日志,tag `_app_http_server_access`
  (`log.RegisterAppTag("http_server", "access")`),带 `http.request.method`、`url.path`、
  `http.response.status_code`、`status` 与 `duration_ms`。5xx 为 Warn 行;span 与该行在
  `status` 上一致。

这些命名刻意与 HTTP 家族一致 —— 与 gin、echo、hertz 用的是同一套,因此应用在 stdlib server
与 gin 之间切换时看板不变。span 与指标走 `starter-otel` 安装的全局 OTel;未引入时它们是
no-op,只写访问日志。

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

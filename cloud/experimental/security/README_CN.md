# security
[English](README.md) | [中文](README_CN.md)

`security` 是与框架无关、零依赖的认证与授权抽象——Spring Security 等价能力
用 Go 惯用法表达,而非对其 filter-chain 机制的移植。它回答两个问题:"调用
者是谁?"(挂在请求 context 上的 `Authentication`)与"这个调用者能做什么?"
(`HasAnyAuthority`、`Require`、`Authorize`)。

## 特性

- 零第三方依赖。
- 中立身份模型:`Principal{Subject, Claims}`,
  `Authentication{Principal, Token, Authenticated, Authorities}`,
  `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities` 均 nil-safe。
- 可插拔 `TokenValidator` 缝隙 + driver 注册表(`RegisterValidator` /
  `GetValidator` / `MustGetValidator`),与 `discovery.Register` /
  `resilience.RegisterDriver` 同构。
- 方法级安全:`Require(authorities...)` 返回普通装饰器——`@PreAuthorize` 的
  等价物,与其他横切按普通函数嵌套组合。
- HTTP 中间件链:`Chain`、`Authenticate`、`Authorize`、`CORS`、`CSRF`
  (double-submit-cookie)——普通 `func(http.Handler) http.Handler` 装饰器,
  不是自建 filter 注册中心。
- `WithAuthentication` / `FromContext` 用于 context 传递。

## 快速开始

Import 路径: `go-spring.org/cloud/experimental/security`。

资源服务器把安全过滤链挂在业务 handler 之前:

```go
package main

import (
    "context"
    "net/http"

    "go-spring.org/cloud/experimental/security"
)

type myValidator struct{ /* ... */ }

func (v *myValidator) Validate(ctx context.Context, token string) (*security.Authentication, error) {
    // 这里验证 bearer token 并返回 Authentication
    return &security.Authentication{
        Principal:     security.Principal{Subject: "u-1"},
        Token:         token,
        Authenticated: true,
        Authorities:   []string{"orders:read"},
    }, nil
}

func main() {
    v := &myValidator{}
    mux := http.NewServeMux()
    mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
        // auth 已挂在 r.Context() 上
        _, _ = security.FromContext(r.Context())
        _, _ = w.Write([]byte("ok"))
    })

    chain := security.Chain(
        security.CORS(security.CORSConfig{AllowedOrigins: []string{"*"}}),
        security.Authenticate(v, true),
        security.Authorize("orders:read"),
    )
    _ = http.ListenAndServe(":8080", chain(mux))
}
```

服务方法级校验用 `security.Require` 装饰器直接包调用:

```go
err := security.Require("orders:write")(ctx, svc.placeOrder)
```

JWT 资源服务器 starter(`starter-security-jwt`)提供具体 `TokenValidator` 并
`Wrap` 服务 mux;授权服务器 starter(`starter-oauth2-server`)签发
`Authenticate` 校验的令牌。

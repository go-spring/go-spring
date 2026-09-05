# starter-http-server Usage

[English](USAGE.md) | [中文](USAGE_CN.md)

Anchored on the self-verifying [example](example/example.go) (`./check.sh`
exits 0).

## 1. 快速开始

无外部依赖。依赖框架内置 server:

```
go get go-spring.org/starter-http-server
```

```properties
# conf/app.properties — server 本体是 gs 内置的,不是本 starter 提供的
spring.http.server.addr=:9090
```

```go
gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.Handle("/api/me", httpsvr.Chain(
        httpsvr.Authenticate(v, true),
        httpsvr.Authorize("orders:read"),
    )(http.HandlerFunc(me)))
    return &gs.HttpServeMux{Handler: mux}
})
```

Token 来源:注入 `security.TokenValidator`(由 `starter-oauth2-resource-server`
或 `starter-security-jwt` 按配置贡献),或自行实现。

## 2. 配置参考

本 starter 无自有配置 key。server 侧配置(`spring.http.server.addr` /
`readTimeout` / `writeTimeout` / `idleTimeout`)属于 `gs` 内置,见 spring 文档。

## 3. API

- `Chain(ms ...Middleware) Middleware` — 组合,最外层优先;规范顺序
  `Chain(CORS, CSRF, Authenticate, Authorize)`。
- `Authenticate(v security.TokenValidator, required bool)` — 缺 token 时
  `required=true` 401、`false` 透传;非法 token 恒 401。
- `Authorize(authorities ...string)` — 匿名 401;已认证但缺任一所需权限
  403;无参数 = 仅要求已认证。
- `CORS(CORSConfig)` / `CSRF(CSRFConfig)` — 见
  [cloud/security DESIGN](../../cloud/security/DESIGN_CN.md) 的约束(wildcard
  × credentials、double-submit-cookie)。

## 4. 运维

中间件不产生日志、指标;观测由 server 侧与 `starter-governance` 提供。
CSRF cookie 名/头名默认 `csrf_token` / `X-CSRF-Token`,与 gin/echo 壳一致。

## 5. 设计体检表

| 指标 | 数值 |
|------|------|
| 自有配置 key | 0 |
| 前置外部依赖 | 0 |
| bean 数 | 0(纯库) |

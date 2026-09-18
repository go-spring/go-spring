# starter-http-server 使用说明

[English](USAGE.md) | [中文](USAGE_CN.md)

锚定自校验示例 [example](example/example.go)(`./check.sh` 退出码 0)。

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
- `Observe() Middleware` — 服务端可观测中间件(span + 指标 + 访问日志);可选启用,
  见 §4。
- `CORS(CORSConfig)` / `CSRF(CSRFConfig)` — 见
  [cloud/security DESIGN](../../cloud/security/README_CN.md) 的约束(wildcard
  × credentials、double-submit-cookie)。

## 4. 运维

安全中间件不产生日志、指标;观测由 server 侧与 `starter-governance` 提供。
CSRF cookie 名/头名默认 `csrf_token` / `X-CSRF-Token`,与 gin/echo 壳一致。

`Observe()` 是唯一的例外:它是**可选启用**的服务端可观测中间件(不像 gin/echo/hertz 是内置
插桩——本包装饰的是应用自己传给框架 server 的 handler,没有地方自我安装),按普通装饰器
组合进链即可:

```go
mux.Handle("/api/me", httpsvr.Chain(
    httpsvr.Observe(),
    httpsvr.CORS(cfg),
    httpsvr.Authenticate(v, true),
)(handler))
```

一次请求产出:

- 一个 server span,名为 `<METHOD> <path>`,属性 `http.request.method`、`url.path`,退出时
  再带 `http.response.status_code` 与 `status`;
- 两个指标:`http.server.request.duration`(Float64Histogram,秒)与
  `http.server.active_requests`(Int64UpDownCounter),label 为 `http.request.method`、
  `http.response.status_code`;
- 一行访问日志,tag `_app_http_server_access`(`log.RegisterAppTag("http_server", "access")`),
  字段 `http.request.method`、`url.path`、`http.response.status_code`、`status`、
  `duration_ms`;5xx 为 Warn 行。

命名刻意与 gin/echo/hertz 的 HTTP 族一致,因此同一应用在 stdlib server 与 gin 之间切换时
看板不变。span 与指标走 starter-otel 安装的全局 OTel,未引入时为 no-op,只写访问日志。

## 5. 设计体检表

| 指标 | 数值 |
|------|------|
| 自有配置 key | 0 |
| 前置外部依赖 | 0 |
| bean 数 | 0(纯库) |

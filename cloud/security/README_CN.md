# security
[English](README.md) | [中文](README_CN.md)

`security` 是与框架无关、零依赖的认证与授权抽象——Spring Security 的等价能力
用 Go 惯用法表达，而非对其 filter-chain 机制的移植。它为资源服务器回答两个
问题：**调用者是谁？**（挂在请求 context 上的 `Authentication`）与**这个调用
者能做什么？**（`HasAnyAuthority`、`Require`）。

令牌校验本身可插拔：starter 实现单一的 `TokenValidator` 接口并作为容器 bean
贡献出来；server 家族中间件拿这个 validator 校验传入凭证，并把得到的身份挂到
context 上供下游守卫读取。

## 快速开始

Import 路径：`go-spring.org/cloud/security`。

本文走查的可运行版本在 [`example/`](example/) 下（`go run ./example`，或
`./example/check.sh`）。

### 1. 校验凭证：`TokenValidator` 缝隙

实现这一个方法：给定原始令牌，返回它代表的身份；对任何无法背书的凭证返回非
nil error。

```go
type jwtValidator struct{ /* keys, issuer, ... */ }

func (v *jwtValidator) Validate(ctx context.Context, token string) (*security.Authentication, error) {
    claims, err := v.verify(ctx, token) // 签名与过期校验由你的代码完成
    if err != nil {
        return nil, err
    }
    return &security.Authentication{
        Principal:     security.Principal{Subject: claims.Subject, Claims: claims.Raw},
        Token:         token,
        Authenticated: true,
        Authorities:   claims.Roles, // 扁平化的 scope/role，命名约定由你决定
    }, nil
}
```

starter 会把具体 validator 作为容器 bean 贡献出来（例如
`starter-security-jwt`）；资源服务器用哪个 validator 是装配决策，不是本包按名
的全局查表。

### 2. 把身份挂到请求上

各 server 家族基于本身份模型、用自家惯用法自带中间件，本包无需安装任何东西。
壳经装配给它的 `TokenValidator` 完成校验，并把结果放到请求 context 上：

```go
import httpsvr "go-spring.org/starter-http-server"

chain := httpsvr.Chain(
    httpsvr.Authenticate(validator, true), // required=true:无 token 则 401
    httpsvr.Authorize("orders:read"),      // 路由级闸门
)
http.ListenAndServe(":8080", chain(mux))
```

下游代码用 `FromContext` 读回调用者：

```go
auth, _ := security.FromContext(r.Context()) // auth 可能为 nil / 未认证
if auth.HasAnyAuthority("orders:read") {     // 判定方法是 nil-safe 的
    _ = auth.Principal.Subject               // 字段访问不是：要先判空
}
```

### 3. 方法级守卫

`Require` 是普通装饰器——`@PreAuthorize` 的等价物。它从 context 读身份，并返回
调用方映射为状态码的哨兵错误：

```go
err := security.Require("orders:write")(ctx, svc.placeOrder)
switch {
case errors.Is(err, security.ErrUnauthenticated): // 401:无已验证身份
case errors.Is(err, security.ErrForbidden):       // 403:已认证但缺该权限
}
```

不传权限时，`Require()` 退化为“必须有已认证调用者”。这里刻意不建共享拦截器链
协议：装饰器就是普通函数，组合横切关注点就是普通嵌套——

```go
err := security.Require("orders:write")(ctx, func(ctx context.Context) error {
    return transaction.GlobalTransactional(coord, reg)(ctx, "OrderService.Place", place)
})
```

## 包内有什么

- **身份模型。** `Principal{Subject, Claims}` 与
  `Authentication{Principal, Token, Authenticated, Authorities}`，配
  `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities`。
- **`TokenValidator`**——starter 或应用接入 JWT 校验、opaque-token 内省等的
  唯一缝隙。
- **`Require(authorities...)`**——方法级守卫装饰器。
- **`WithAuthentication` / `FromContext`**——用未导出 key 类型做 context 传
  递，防止碰撞。
- **共享纯函数**，保证各家族中间件行为不漂移：`ParseBearerToken`、
  `NewCSRFToken` / `MatchCSRFToken`（常量时间比较）、
  `DefaultCSRFCookieName` / `DefaultCSRFHeaderName`。
- **错误哨兵** `ErrUnauthenticated`（→ 401）与 `ErrForbidden`（→ 403）。

## HTTP 中间件在哪里

本包刻意不提供 HTTP 中间件：各 server 家族基于共享身份模型、用自家惯用法自行
装配——stdlib 的 `http.Handler` 装饰器在 `starter-http-server`，
`gin.HandlerFunc` 在 `starter-gin`，`echo.MiddlewareFunc` 在
`starter-echo`。CORS 同理（`starter-http-server` 自带一份；gin 用
`gin-contrib/cors`，echo 用其内建）。

## 设计

### 职责与边界

- 只两个问题：**调用者是谁**，以及**能不能做**。
- 不是密码学库。`TokenValidator` 是缝隙；JWT / opaque-token /
  session-cookie 的具体实现在 starter 或调用方应用中。
- 不是 session 库（见 `cloud/experimental/session`），不是 OAuth2 授权服务器（见
  `starter-oauth2-server`）。
- HTTP 中间件、路由级 `Authorize` 与 CORS 随各 server 家族走；只有这些壳共用
  的安全敏感逻辑以纯函数形式暴露在本包，保证各家族行为不漂移。方法级
  `Require` 留在本包——同一权限集，两道闸：路由闸用 server 惯用法，装饰器闸在
  本包。

### 约束（禁止破坏）

- **`Authentication` 方法 nil-safe** 且 `!Authenticated` 一律 false。下游代码
  可以直接对 `FromContext` 取到的值调 `HasAnyAuthority`，不必 nil 判空；别引
  入破坏该性质的字段。
- **`Authenticate(v, required=false)`**（各家族壳）无 token 时必须让请求原样
  透传，不挂 `Authentication`——“authority 决策由后续过滤器决定”。**非法**
  token 一律 401；**缺失** token 仅在 `required=true` 时 401。
- **CORS 通配符与 credentials**：`AllowCredentials=true` 时不能发
  `Access-Control-Allow-Origin: *`——规范禁止。要回显具体 origin 并加
  `Vary: Origin`。
- **CSRF 是 double-submit-cookie**：服务端无状态。安全方法种下 cookie；非安全
  方法必须在 header 里经 `MatchCSRFToken`（常量时间）回显该 cookie。它与
  bearer-token API 正交，后者不易 CSRF，不要强推。
- **非对称密钥场景绝不接受 HMAC**（算法混淆防护）——不变量在 starter 里体现，
  但这里点名，免得后续在本包新写 validator 时踩坑。

### 权衡 / 未做的方案

- **不建共享 HTTP 中间件协议。** 共享的
  `func(http.Handler) http.Handler` 层只适配 net/http，gin/echo 要靠适配器硬
  套（hertz 根本接不上）；已删除，改为各家族自带壳。壳的重复成本由共享纯函数
  与身份模型兜底，收益是各家族读起来都是原生惯用法。
- **不复刻 Spring Security filter 注册表。** 顺序 = 各壳的链式顺序；推理显
  式，没有看不见的优先级。
- **无 validator 驱动注册表。** validator 由 starter 作为容器 bean 贡献；中间
  件拿它装配时接到的 `TokenValidator` 值做每请求校验——装配期与请求期都不做按
  名的全局查找。
- **无注解扫描。** `@PreAuthorize` 由显式的 `security.Require(...)` 装饰器取
  代。

JWT 资源服务器 starter（`starter-security-jwt`）提供具体 `TokenValidator`；授
权服务器 starter（`starter-oauth2-server`）签发中间件校验的令牌。

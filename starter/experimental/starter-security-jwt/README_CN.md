# starter-security-jwt

[English](README.md) | [中文](README_CN.md)

`starter-security-jwt` 把 Go-Spring 应用变成 OAuth2/OIDC **资源服务器**:对进入的
请求校验 JWT bearer token,成功后把认证身份挂到请求上下文。它以中间件形式工作在
`net/http` 层,因此与后端具体 Web 框架(gin/echo/hertz/net-http)无关;同时实现了
`security.TokenValidator`,可在非 HTTP 传输上以编程方式校验 token。

## 安装

```bash
go get go-spring.org/starter-security-jwt
```

## 快速开始

### 1. 导入 `starter-security-jwt` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-security-jwt"
```

### 2. 配置 Authenticator

在项目的[配置文件](example/conf/app.properties)中添加配置。`spring.security.jwt.*`
下每个条目生成一个具名的 `*Authenticator`。必须且只能配置一种校验密钥来源 —— HMAC
密钥、PEM 公钥,或远程 JWKS 端点:

```properties
# HMAC(对称)
spring.security.jwt.api.secret=example-shared-secret

# 或非对称 PEM 公钥
# spring.security.jwt.api.public-key-file=./conf/public.pem

# 或远程 JWKS 端点(自动拉取并刷新密钥)
# spring.security.jwt.api.jwks-url=https://issuer.example.com/.well-known/jwks.json
```

### 3. 把 Authenticator 接入 HTTP 服务

Authenticator 按配置子键(`api`)注入并包裹业务 handler。把包裹后的 handler 交给
`*gs.HttpServeMux`,认证就位于服务器前面。参考 [example.go](example/example.go) 文件。

```go
gs.Provide(func(auth *StarterSecurityJWT.Authenticator) *gs.HttpServeMux {
    mux := http.NewServeMux()
    mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
        a, _ := security.FromContext(r.Context())
        _, _ = fmt.Fprintf(w, "hello %s", a.Principal.Subject)
    })
    return &gs.HttpServeMux{Handler: auth.Wrap(mux)}
}, gs.TagArg("api"))
```

### 4. 在 handler 中读取身份

在 handler 内,`security.FromContext` 返回已校验的 `*security.Authentication`。它的
`HasAuthority`/`HasAnyAuthority` 帮助方法基于 scope 与 role 做访问控制:

```go
a, _ := security.FromContext(r.Context())
if !a.HasAuthority("admin") {
    http.Error(w, "forbidden", http.StatusForbidden)
    return
}
```

## 配置项

所有键位于 `spring.security.jwt.<name>` 下:

| 键 | 默认值 | 说明 |
| --- | --- | --- |
| `issuer` | — | 期望的 `iss` claim;为空则不校验 |
| `audience` | — | 可接受的 `aud` 值(列表);为空则不校验 |
| `algorithm` | — | 固定单一签名算法(如 `RS256`);为空则接受与密钥来源兼容的任意算法 |
| `secret` | — | 共享 HMAC 密钥(HS256/384/512) |
| `public-key` | — | 内联 PEM RSA/ECDSA 公钥 |
| `public-key-file` | — | PEM 公钥文件路径 |
| `jwks-url` | — | 远程 JWKS 端点 |
| `jwks-refresh` | `15m` | 已拉取的 JWKS 缓存多久后刷新 |
| `jwks-timeout` | `10s` | 单次 JWKS HTTP 拉取超时 |
| `scope-claim` | `scope` | 承载 scope 的 claim(空格分隔字符串或数组) |
| `roles-claim` | `roles` | 承载 role 的 claim(字符串或数组) |
| `leeway` | `0` | exp/nbf/iat 的时钟偏移容忍 |
| `required` | `true` | `true` 时缺 token 返回 401;`false` 时放行且不挂身份 |

## 核心特性

[example.go](example/example.go) 程序演示并断言:

* **拒绝缺失 token** —— `required=true`(默认)时,无 bearer token 的请求返回 `401`。
* **认证** —— 合法 token 通过校验,subject 可经 `security.FromContext` 取到。
* **方法级授权** —— handler 强制 `admin` 授权,缺失时返回 `403`。
* **拒绝非法 token** —— 垃圾或过期 token 返回 `401`。

## 高级特性

* **三种密钥来源**:HMAC 密钥、非对称 PEM 公钥(RSA 或 ECDSA),或远程 JWKS 端点
  —— 启动时拉取、缓存,并按间隔或遇到未知 `kid` 时刷新(吸收密钥轮换)。
* **算法混淆防护**:非对称密钥来源永不接受 HMAC 算法,阻断经典的"用公钥当 HMAC
  密钥签名"攻击。可进一步用 `algorithm` 锁定。
* **框架无关**:因为它包裹的是普通 `http.Handler`,无论 gin/echo/hertz/net-http
  serve 路由,同一个 authenticator 都能用。
* **可选认证**:设 `required=false` 让未认证请求放行且不挂身份,把决策交给方法级守卫。
* **编程式校验**:`*Authenticator` 实现 `security.TokenValidator`,可在 HTTP 路径外
  (如 gRPC/WebSocket 传输)校验原始 token 字符串。
* **多实例**:在 `spring.security.jwt.*` 下定义多个条目,用 `gs.TagArg("...")` 按名选取。

# starter-oauth2-server

[English](README.md) | [中文](README_CN.md)

`starter-oauth2-server` 让 Go-Spring 应用充当 OAuth2/OIDC **授权服务器**：它通过标准的
`/authorize`、`/token` 和 `/jwks` 端点签发令牌，支持 `authorization_code`（带 **PKCE**）、
`client_credentials` 和 `refresh_token` 三种授权模式。它是 `starter-oauth2-client`
（已覆盖客户端一侧）的服务端对应物，并与资源服务器一侧的 `starter-security-jwt` 配套：
本服务器签发的令牌在那里用共享的密钥材料验证。

它是 **Contributor（贡献者）** 形态的 starter —— 自身不开监听端口。应用注入
`*AuthServer` bean，并把它的 `Handler()` 挂到应用已经运行的 HTTP 服务器上。

## 范围

本 starter 实现的是 OAuth2/OIDC **协议端点**，而非完整的身份提供方（IdP）。它不附带
用户存储、不做 MFA、也不做社交登录聚合；资源属主（resource-owner）的登录是一个缝隙
（`UserAuthFunc`），由应用把自己的会话/登录接进来。它不是 Spring Security
`SecurityFilterChain` DSL 的移植 —— 等价的 Web 安全过滤链是用 `spring/security` 中的
普通 `net/http` 中间件（`Chain` / `CORS` / `CSRF` / `Authenticate` / `Authorize`）组装的。

## 安装

```bash
go get go-spring.org/starter-oauth2-server
```

## 快速开始

### 1. 导入包

```go
import _ "go-spring.org/starter-oauth2-server"
```

### 2. 配置服务器及其客户端

在项目的[配置文件](example/conf/app.properties)中添加配置。必须且只能设置一个签名密钥
—— 要么是共享的 HMAC `secret`（资源服务器用同一个 secret 验证），要么是 PEM 格式的
`private-key` / `private-key-file`（其公钥部分发布在 `/jwks`）：

```properties
spring.oauth2.server.enabled=true
spring.oauth2.server.issuer=https://issuer.example.com
spring.oauth2.server.secret=example-shared-secret

# 公开客户端（SPA / 原生 App）：无 secret，强制 PKCE。
spring.oauth2.server.clients.spa.public=true
spring.oauth2.server.clients.spa.redirect-uris=http://127.0.0.1:9090/callback
spring.oauth2.server.clients.spa.scopes=read,write

# 仅使用 client_credentials 模式的机密客户端。
spring.oauth2.server.clients.svc.secret=svc-secret
spring.oauth2.server.clients.svc.scopes=read
spring.oauth2.server.clients.svc.grant-types=client_credentials
```

### 3. 挂载端点并接入登录缝隙

注入 `*AuthServer`，设置 `UserAuthFunc`（资源属主登录），并把 `Handler()`
挂到你的 HTTP 服务器上。参考 [example.go](example/example.go) 文件。

```go
gs.Provide(func(as *StarterOAuth2Server.AuthServer) *gs.HttpServeMux {
    // 接入你自己的登录：返回要授予的 subject 和 authorities。
    as.UserAuthFunc = func(r *http.Request) (string, []string, bool) {
        return "alice", []string{"admin"}, true
    }
    mux := http.NewServeMux()
    mux.Handle("/oauth2/", http.StripPrefix("/oauth2", as.Handler()))
    return &gs.HttpServeMux{Handler: mux}
})
```

### 4. 用过滤链保护资源

统一的 Web 安全过滤链位于 `spring/security`。用 `security.Chain` 按顺序编排各关注点
—— 先 CORS，再认证，最后授权：

```go
validator := /* 一个 security.TokenValidator，例如 starter-security-jwt 的 Authenticator */
mux.Handle("/api/admin", security.Chain(
    security.CORS(security.CORSConfig{AllowedOrigins: []string{"https://app.example.com"}}),
    security.Authenticate(validator, true), // 认证 bearer 令牌
    security.Authorize("admin"),            // 要求 "admin" 权限
)(businessHandler))
```

## 端点

| 端点             | 方法     | 用途                                                          |
|------------------|----------|---------------------------------------------------------------|
| `/authorize`     | GET      | `authorization_code` 模式的授权端点。                          |
| `/token`         | POST     | 三种模式共用的令牌端点。                                        |
| `/jwks`          | GET      | 发布验证密钥（HMAC 签名密钥时为空集）。                          |

## 授权模式

- **`authorization_code`（+ PKCE）** —— 用户经 `UserAuthFunc` 认证后，一个一次性 code
  被重定向回客户端，客户端在 `/token` 兑换它。**公开客户端（`public: true`）强制 PKCE**；
  `/token` 提交的 `code_verifier` 会与 `/authorize` 时记录的 `code_challenge` 校验。
- **`client_credentials`** —— 机密客户端用自己的 secret 认证，获得代表自身的访问令牌
  （无 refresh 令牌）。
- **`refresh_token`** —— refresh 令牌轮换（一次性）换取新的访问令牌；请求可以收窄、
  但不能扩大已授予的 scope。

## 安全须知

- 签名密钥通过配置与资源服务器共享；对于 HMAC 两者无需直接通信，对于非对称密钥资源
  服务器拉取 `/jwks`。
- `redirect_uri` 在被用作重定向目标之前，会先对照每个客户端的允许列表校验，堵住
  开放重定向漏洞。
- 客户端 secret 与 PKCE verifier 均以常量时间比较。
- 授权码与 refresh 令牌保存在进程内存中（单节点）；多节点部署应在其前置一个共享存储。

## 设计

每个官方 starter 都遵循的设计约束，见 [DESIGN.md](../DESIGN.md)。

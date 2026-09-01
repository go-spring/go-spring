# starter-oauth2-resource-server（OAuth2 资源服务器）

把 Go-Spring 应用变成 OAuth2 资源服务器：校验请求携带的 JWT Bearer
Token，并把结果挂到框架中立的 `security.TokenValidator`
抽象（`go-spring.org/cloud/security`）上。

每个 `spring.security.oauth2.resource.jwt.<name>` 条目注册一个
`Validator` bean——配置即开关，只导入包而不配置不会装配任何东西。

## 功能特性

- **JWT 签名校验**：RS256/RS384/RS512、ES256/ES384/ES512、PS256/PS384/PS512
  与 HS256/HS384/HS512，基于 `golang-jwt/jwt/v5`。
- **标准声明校验**：`exp`/`nbf`（支持时钟偏移容忍），以及配置后的
  `iss`/`aud`。
- **三种密钥来源**（每个实例必须且只能配置一种，否则启动失败）：
  - `issuer-uri`：通过 OIDC 发现
    （`{issuer-uri}/.well-known/openid-configuration`）定位 JWKS 端点；
    密钥缓存定时刷新，遇到未知 `kid` 立即重拉以吸收密钥轮换。
  - `public-key` / `public-key-file`：静态 RSA/ECDSA PEM 公钥。
  - `secret`：共享 HMAC 密钥。
- **防算法混淆**：非对称密钥来源永不接受 HMAC；可用 `algorithm`
  钉死单一算法。
- **权限映射**：`scope`、`roles` 声明（字符串或数组）展平进
  `Authentication.Authorities`，供 `security.Authorize` /
  `security.Require` 使用。

## 快速开始

```properties
spring.security.oauth2.resource.jwt.api.issuer-uri=https://auth.example.com
spring.security.oauth2.resource.jwt.api.audiences=orders-api
```

```go
import (
    "go-spring.org/cloud/security"
    "go-spring.org/spring/gs"

    _ "go-spring.org/starter-oauth2-resource-server"
)

func init() {
    gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
        mux := http.NewServeMux()
        mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            fmt.Fprintf(w, "hello %s", a.Principal.Subject)
        })
        return &gs.HttpServeMux{
            Handler: security.Chain(security.Authenticate(v, true))(mux),
        }
    }, gs.TagArg("api"))
}
```

## 配置项

配置键：`spring.security.oauth2.resource.jwt.<name>.*`

| 属性 | 默认值 | 说明 |
| --- | --- | --- |
| `issuer-uri` | — | 签发方标识；经 OIDC 发现解析出 JWKS 端点，同时作为期望的 `iss` |
| `issuer` | — | 覆盖期望的 `iss` 声明 |
| `public-key` / `public-key-file` | — | 静态 RSA/ECDSA 公钥（PEM） |
| `secret` | — | 共享 HMAC 密钥 |
| `audiences` | — | 可接受的 `aud` 值（命中任一即可） |
| `algorithm` | 兼容集合 | 钉死单一签名算法 |
| `jwks-refresh` | `15m` | JWKS 缓存刷新间隔 |
| `jwks-timeout` | `10s` | 发现/JWKS 拉取超时 |
| `scope-claim` / `roles-claim` | `scope` / `roles` | 映射为权限的声明名 |
| `leeway` | `0` | `exp`/`nbf`/`iat` 的时钟偏移容忍 |

## 使用方式

运行示例：

```bash
cd example
./check.sh
```

设计取舍见[设计文档](DESIGN_CN.md)。

## 许可证

Apache License 2.0

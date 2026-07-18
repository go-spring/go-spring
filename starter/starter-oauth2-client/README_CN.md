# starter-oauth2-client

[English](README.md) | [中文](README_CN.md)

`starter-oauth2-client` 基于 [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2)
提供 OAuth2 *client-credentials*(客户端凭证)模式的 HTTP 客户端,让 Go-Spring
应用能方便地调用受保护的下游服务。生成的 `*http.Client` 会自动获取并刷新
bearer token,调用方像使用普通 HTTP 客户端一样使用它即可。

## 安装

```bash
go get go-spring.org/starter-oauth2-client
```

## 快速开始

### 1. 导入 `starter-oauth2-client` 包

参考 [example.go](example/example.go) 文件。

```go
import _ "go-spring.org/starter-oauth2-client"
```

### 2. 配置 OAuth2 客户端

在项目的[配置文件](example/conf/app.properties)中添加配置,例如:

```properties
spring.oauth2.client.downstream.client-id=demo-client
spring.oauth2.client.downstream.client-secret=demo-secret
spring.oauth2.client.downstream.token-url=https://auth.example.com/oauth/token
spring.oauth2.client.downstream.scopes=read,write
spring.oauth2.client.downstream.auth-style=header
spring.oauth2.client.downstream.timeout=5s
```

### 3. 注入 HTTP 客户端

starter 会为每个配置键注册一个 `*http.Client`,按该名称注入。参考
[example.go](example/example.go) 文件。

```go
type Service struct {
    Client *http.Client `autowire:"downstream"`
}
```

### 4. 使用 HTTP 客户端

参考 [example.go](example/example.go) 文件。首次请求时自动获取 token,并以
`Authorization: Bearer <token>` 形式附加到请求上。

```go
resp, err := s.Client.Get("https://api.example.com/resource")
```

## 核心特性

[example.go](example/example.go) 会在进程内启动一个 OAuth2 token 端点和一个
受保护资源服务,随后演示并断言:

* **自动获取 token** — 注入的客户端在首次请求时通过 client-credentials 授权拿到 token。
* **透明注入 bearer** — token 被附加到下游请求,受保护端点返回 `200 OK`。

## 高级特性

* **支持多个 OAuth2 客户端**:可在配置文件中定义多个客户端(各自独立凭证),按名称引用。
* **可配置认证方式**:`auth-style` 决定凭证如何发送到 token 端点 —— `auto`(默认)、
  `header`(HTTP Basic)、`params`(请求体参数)。
* **`*TokenSource` bean**:除 `*http.Client` 之外,starter 还会为每个
  `spring.oauth2.client.<name>` 配置项以**相同名称**注册一个
  `*StarterOAuth2Client.TokenSource`。它实现了 `oauth2.TokenSource`,可直接用于任何
  需要该接口的场景。当你需要直接拿到 bearer token 时(例如注入到 gRPC metadata),
  按名称注入即可:

  ```go
  import StarterOAuth2Client "go-spring.org/starter-oauth2-client"

  type Service struct {
      TokenSrc *StarterOAuth2Client.TokenSource `autowire:"downstream"`
  }

  // tok, err := s.TokenSrc.Token()
  // 使用 tok.AccessToken
  ```

  除 `Token()` 外,它还暴露了缓存 token 的状态,便于观测且不会触发请求:

  | 方法 | 说明 |
  | --- | --- |
  | `Token()` | 返回有效 token,按需获取/刷新并缓存。 |
  | `Peek()` | 返回最近一次观测到的 token,尚未获取时返回 `nil`。 |
  | `Valid()` | 报告是否已获取到 token 且未过期。 |
  | `Expiry()` | 返回最近观测 token 的过期时间,没有则返回零值时间。 |

* **附加 token 端点参数(`endpoint-params`)**:部分提供方需要在 token 端点携带额外参数,
  例如 Auth0 的 `audience` 或 Azure AD 的 `resource`。以 map 形式声明,map 的 key
  即为参数名:

  ```properties
  spring.oauth2.client.downstream.endpoint-params.audience=https://api.example.com
  ```

### 授权码模式(Authorization Code)

对于需要用户登录/重定向的交互式流程,使用单独的配置前缀
`spring.oauth2.authcode.<name>`。starter 会为每个配置项注册一个
`*oauth2.Config`,按名称注入即可。

```properties
spring.oauth2.authcode.login.client-id=web-client
spring.oauth2.authcode.login.client-secret=web-secret
spring.oauth2.authcode.login.auth-url=https://auth.example.com/oauth/authorize
spring.oauth2.authcode.login.token-url=https://auth.example.com/oauth/token
spring.oauth2.authcode.login.redirect-url=https://app.example.com/callback
spring.oauth2.authcode.login.scopes=openid,profile
```

注入并使用:

```go
import "golang.org/x/oauth2"

type LoginService struct {
    OAuth *oauth2.Config `autowire:"login"`
}

// url := s.OAuth.AuthCodeURL("state")       // 将用户重定向到该地址
// tok, err := s.OAuth.Exchange(ctx, code)   // 用回调 code 换 token
```

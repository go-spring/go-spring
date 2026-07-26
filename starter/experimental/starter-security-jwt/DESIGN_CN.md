# starter-security-jwt 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-security-jwt` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),基于零依赖抽象 `spring/security`
提供 Spring Security 的 JWT 资源服务器等价能力。不开端口,以中间件形式挂到
已有的 `*gs.HttpServeMux`。

## 1. 职责与边界

- **在范围内:**解析并校验 Bearer JWT;三种密钥来源(HMAC secret / PEM 公钥
  / JWKS URL);将 claims 映射为 `security.Authentication`;暴露
  `Wrap(next http.Handler)` 缝隙及对非 HTTP 传输可复用的
  `security.TokenValidator`。
- **不在范围内:**签发 token(那是 `starter-oauth2-server` 的事);授权
  *策略*(那是 `spring/security` 的 aspect / middleware);用户存储 / 登录 UI。

## 2. 关键抽象与缝隙

- **`spring/security`——零依赖抽象。**本 starter 是其中一种具体实现。
  `TokenValidator` 是 driver 缝隙(注册表式——与 `spring/discovery`、
  `spring/resilience` 同款)。`Principal` + `Authentication` 是中立类型,
  自带 `HasAuthority` / `HasAnyAuthority` / `HasAllAuthorities` 帮助函数,
  均 nil-safe,`!Authenticated` 时短路返回 `false`。
- **同一 bean,两个挂载点。**
  - `Wrap(next http.Handler) http.Handler`——HTTP 中间件缝隙,对齐
    `starter-lua-filter`,无框架耦合。
  - `security.TokenValidator`——同一 `*Authenticator` 亦满足与传输无关的
    validator,可从 gRPC metadata、WebSocket 握手等场景复用。
- **多实例经 `gs.Group("${spring.security.jwt}", ...)`。**应用可为多签发方
  各配一项。destroy 传 `nil`(无 goroutine,JWKS 缓存按需刷新)。

## 3. 约束

- **每实例恰有一种密钥来源——fail-fast。**HMAC `secret`、PEM `public-key` /
  `public-key-file`、或 JWKS `jwks-url`。两个都配 = 启动错;都没配 = 启动错。
- **算法混淆防护。**非对称来源永不接受 HMAC 算法——挡住"用公钥当 HMAC
  secret 签名"的经典攻击。`validMethods` 通过 `golang-jwt/jwt/v5` parser
  强制。
- **JWKS 内建解析**,不引外部 `keyfunc`。RSA `n`/`e`、EC `crv`/`x`/`y` 直接
  从 base64url 解码;依赖图仅止于 `golang-jwt/jwt/v5`。缓存按配置间隔刷新,
  遇未知 `kid` 时也刷新。
- **构造期取不到 bean 名。**`gs.Group` 构造函数签名收不到名字
  (`gs.go:488` 限制),per-instance `RegisterValidator(name)` 无法直接实现;
  调用方需要具名复用时通过 DI 注入 `*Authenticator`。
- 内部依赖靠 `go.work` 解析;不跑 `go mod tidy`。外部依赖仅
  `github.com/golang-jwt/jwt/v5`。

## 4. 取舍 / 弃选方案

- **`MicahParks/keyfunc` 处理 JWKS——弃选。**几百行内建解析可让传递依赖
  图保持小。
- **Server 形态(自有端口)——弃选。**认证本身没有端口;再开监听只会与
  应用主端口做冗余复用。
- **强绑定某个 web 框架——弃选。**`Wrap` 与 `TokenValidator` 是仅有的两个
  缝隙,任何最终塌缩到 `http.Handler` 的框架(所有 Go web 框架皆是)都能
  直接使用。

# starter-oauth2-server 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-oauth2-server` 属于 **Contributor** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.3),在进程内实现 OAuth2 / OIDC 授权
服务器。不开端口;通过 `Handler()` 挂到应用已有的 HTTP server。

## 1. 职责与边界

- **端点:**`/authorize`、`/token`、`/jwks`。
- **Grant:**`authorization_code`(+ PKCE)、`client_credentials`、
  `refresh_token`。
- **不在范围内:**用户身份 / 登录 UI(那由应用的 `UserAuthFunc` 提供);
  token *校验*(那是 `starter-security-jwt`);分布式 / 集群 session 与 code
  存储(本 starter 为单进程)。

## 2. 关键决策

- **单 bean,非 `gs.Group`。**一应用一授权服务器;`clients` 是配置数据
  (`clients.<id>.public / secret / redirect-uris / scopes / grant-types`),
  不是 bean map。注册方式:
  `gs.Provide(newAuthServer, gs.TagArg("${spring.oauth2.server}")) +
  OnProperty(...enabled).HavingValue("true")`。
- **签名密钥恰有一种——fail-fast。**HMAC `secret` **或** PEM `private-key` /
  `private-key-file`,不许两个都配,也不许都不配(`errNoSigningKey` /
  `errBothSigning`)。HMAC 时 `/jwks` 返回空集;非对称时公钥发布在
  `/jwks`。
- **登录缝隙 = `UserAuthFunc`。**`func(r) (subject, authorities, ok)` 是
  server 的结构体字段,不做可选 bean 注入。应用构造 bean 后设该字段,再挂
  `Handler()`——与 jwt example 的"注入后再建 mux"模式一致。
- **公开客户端强制 PKCE。**`public:true` 的客户端必须携带 `code_challenge`;
  verifier / client secret 常量时间比较;`redirect_uri` 按每客户端白名单精确
  比对,防开放重定向。
- **进程内 code / refresh 存储,单节点。**惰性过期 + 写时 sweep,无后台
  goroutine 故不注册 destroy hook。refresh 每次使用轮换,refresh scope 只能
  收窄原授权。

## 3. 对外工具

`GenerateVerifier()` 与 `Challenge(verifier, method)` 已导出,便于客户端
SDK / 测试构造 PKCE 对,不必手写算法。

## 4. 约束

- **异步 JWKS 引导是坑。**example 有意用 HMAC——若用 RSA + JWKS URL 回指
  本进程,jwt authenticator eager 拉 JWKS 会与尚未起服务的授权服务器死锁。
  demo 用 HMAC;非对称留给真实部署,那时 JWKS URL 通常指向另一个服务。
- **配置错误一律 `errutil.Explain`。**签名密钥不符、未知 grant type、缺失
  redirect——启动时给出清晰错误,而不是"半可用的默认"。
- 内部依赖靠 `go.work`。第三方依赖仅 `golang-jwt/jwt/v5`。

## 5. 取舍 / 弃选方案

- **`gs.Group` 多授权服务器——弃选。**一应用一 AS;多租户是 `clients` map,
  而不是 bean 分组。
- **分布式存储(Redis / DB)——v1 弃选。**"应用自签自用 token"的场景单节点
  足够;Contributor 模式允许后续再有 durable store starter 贡献 `CodeStore`
  bean,不用改本模块。
- **example 里跨 import `starter-security-jwt`——弃选。**跨 workspace 内部
  `require` 会通过 proxy 404;example 内联一个满足 `security.TokenValidator`
  的极小 `hmacValidator` 即可。

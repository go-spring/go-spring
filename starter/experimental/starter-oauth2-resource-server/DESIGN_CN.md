# starter-oauth2-resource-server 设计文档

## 定位

本 starter 是 `cloud/security` 抽象与 `golang-jwt/jwt/v5`
库之间的装配层。抽象层拥有接缝（`TokenValidator`、
`Authenticate`/`Authorize` 中间件、上下文传播）；本 starter 只做一件事：
把配置变成一个可用的 JWT `TokenValidator` bean。它不导出端口、不持有
server——应用在自己的 HTTP mux 上组合 `security.Authenticate(v,
required)`。

## 为什么与 starter-security-jwt 分开

`starter-security-jwt` 已经在 `spring.security.jwt` 键下做 JWT 校验。本
starter 面向的是 OAuth2 资源服务器工作流：

- **OIDC 发现**：只配 `issuer-uri` 就够了——starter 自动拉取
  `.well-known/openid-configuration` 并跟随其中的 `jwks_uri`，对应
  Spring Boot 的 `spring.security.oauth2.resourceserver.jwt.issuer-uri`
  语义，无需手写 JWKS URL。
- **OAuth2 声明语义**：`audiences`（任一命中）以及"签发方标识即期望的
  `iss`"（OIDC 规范约定）。
- **独立配置键**：遵循 client starter 配置原则——相同能力、不同工作
  流，用户敲下的配置键就是技术选型的显式声明。

校验核心（parser 选项、JWKS 缓存、防算法混淆）沿用 starter-security-jwt
已验证的模式，而不是跨 starter 模块共享代码：starter 各自独立版本化，
不应互相依赖。

## 设计取舍

- **JWT 库**：`golang-jwt/jwt/v5`——事实上的 Go JWT 维护版实现。签名
  原语都在标准库 crypto；只把解析、声明校验和 PEM/JWK 解码委托出去。
- **密钥来源互斥**：`issuer-uri`、`public-key`/`public-key-file`、
  `secret` 三选一。无法决定如何验 token 的资源服务器就是配错了，启动
  即失败好过线上 401 排查。
- **发现失败即启动失败**：discovery 文档与首次 JWKS 都在 bean 构造期拉
  取，`issuer-uri` 写错直接让启动失败。
- **JWKS 缓存**：密钥缓存 `jwks-refresh` 时长；未知 `kid` 立即重拉一次
  （吸收密钥轮换）；刷新失败时继续用缓存密钥兜底。
- **Bean 形态**：每个配置条目一个命名 bean，经
  `gs.As[security.TokenValidator]()` 以接口导出，用 `gs.Module` +
  `conf.BindEach` 注册，配置键本身就是启用条件。刻意不用包级
  `security.RegisterValidator` 注册表：把一个活的、来自配置的 validator
  塞进进程级全局 map，在测试与重启之间都是错的（与
  `cloud/experimental/session` 注册表的注释同一推理）。
- **不自带中间件**：传输层策略（401 还是放行、权限检查）已经住在
  `security.Authenticate`/`Authorize` 里，在此复制会分叉策略。

## 测试

单测覆盖：HS256 正常路径与错误密钥、RS256/ES256 静态 PEM、过期 /
未生效 / 签发方不符 / 受众不符、leeway、算法混淆拒绝、以及针对
`httptest` 假 IdP（discovery + JWKS + 未知 kid）的完整 issuer-uri 发现行
为，加上 fail-fast 场景（发现端点损坏、密钥来源缺失/重复）。示例的
`check.sh` 用共享密钥对装配后的应用做端到端冒烟。

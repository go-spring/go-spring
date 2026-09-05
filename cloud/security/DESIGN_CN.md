# security 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`security` 是零依赖的认证/授权抽象。`starter-security-jwt` 贡献 JWT
`TokenValidator`(无自有端口);`starter-oauth2-server` 负责签发令牌;业
务代码只看到 `security.*`。

## 1. 职责与边界

- 只两个问题:**调用者是谁**(挂 ctx 上的 `Authentication`)与**能不能做**
  (`HasAnyAuthority` / `Require`)。
- 不是密码学库。`TokenValidator` 是缝隙;JWT / opaque-token / session-cookie
  的具体实现在 starter 或调用方应用中。
- 本包不放 HTTP 中间件。各 server 家族基于本身份模型、用自家惯用法装配
  自家的壳——stdlib 装饰器在 `starter-http-server`,`gin.HandlerFunc` 在
  `starter-gin`,`echo.MiddlewareFunc` 在 `starter-echo`。壳共用的安全敏感
  逻辑以纯函数暴露(`ParseBearerToken`、`NewCSRFToken` /
  `MatchCSRFToken` 常量时间比较),保证各家族行为不漂移。
- 不是 session 库(见 `cloud/session`),不是 OAuth2 授权服务器(见
  `starter-oauth2-server`)。

## 2. 关键抽象与缝隙

- `TokenValidator`——单方法接口,同时驱动各家族中间件与 driver 注册表。
  实现必须并发安全,并对任何无法背书的凭证**返回非 nil error**,而不是
  返回 `Authenticated=false` 的 `Authentication`。
- `RegisterValidator` / `GetValidator` / `MustGetValidator`——driver-registry
  范式(空名/nil/重名一律 panic),与 `discovery.Register` /
  `resilience.RegisterDriver` 同构。
- `WithAuthentication` / `FromContext`——用未导出 key 类型的 ctx 传递,防碰
  撞。
- `Require(authorities...)`——普通装饰器;读 `FromContext(ctx)`,
  缺失时返 `ErrUnauthenticated`,认证但缺权限时返 `ErrForbidden`,否则
  `Proceed`。这是**AOP 等价**的方法守卫,走普通函数装饰器,而非字节码/注解
  移植。

## 3. 约束(禁止破坏)

- **`Authentication` 方法 nil-safe** 且 `!Authenticated` 一律 false。下游代
  码可以直接对 `FromContext` 取到的值调 `HasAnyAuthority`,不必 nil 判空;别
  引入破坏该性质的字段。
- **`Authenticate(v, required=false)`**(各家族壳)无 token 时必须让请求原
  样透传,不挂 `Authentication`——"authority 决策由后续过滤器决定"。
  **非法** token 一律 401;**缺失** token 仅在 `required=true` 时 401。
- **CORS 通配符与 credentials**:`AllowCredentials=true` 时不能发
  `Access-Control-Allow-Origin: *`——规范禁止。要回显具体 origin 并加
  `Vary: Origin`。
- **CSRF 是 double-submit-cookie**:服务端无状态。安全方法种下 cookie;非安
  全方法必须在 header 里经 `MatchCSRFToken`(常量时间)回显该 cookie。它与
  bearer-token API 正交,后者不易 CSRF,不要强推。
- **非对称密钥场景绝不接受 HMAC**(算法混淆防护)——不变量在 starter 里体现,
  但这里点名,免得后续在本包新写 validator 时踩坑。

## 4. 权衡 / 未做的方案

- **不建共享 HTTP 中间件协议**。共享的 `func(http.Handler) http.Handler` 层
  只适配 net/http,gin/echo 要靠适配器硬套(hertz 根本接不上);已删除,改为
  各家族自带壳。壳的重复成本由共享纯函数与身份模型兜底,收益是各家族读起来
  都是原生惯用法。
- **不复刻 Spring Security filter 注册表**。顺序 = 各壳的链式顺序;推理
  显式,没有看不见的优先级。
- **路由级 `Authorize` 随各家族壳走,方法级 `Require` 留在本包**——同一权限
  集,两道闸:路由闸用 server 惯用法,装饰器闸在本包,保持一致。
- **注册表不在请求路径解析 validator**。中间件直接接 `TokenValidator`
  值;注册表用于装配期**查找** validator,不是每个请求都查。
- **无注解扫描**。`@PreAuthorize` 由显式的 `security.Require(...)` 装饰器
  取代——AOP 等价链。

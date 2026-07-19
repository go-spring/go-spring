# security 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`security` 是 stdlib 层零依赖的认证/授权抽象。`starter-security-jwt` 贡献
JWT `TokenValidator`(无自有端口);`starter-oauth2-server` 负责签发令牌;业
务代码只看到 `security.*`。

## 1. 职责与边界

- 只两个问题:**调用者是谁**(挂 ctx 上的 `Authentication`)与**能不能做**
  (`HasAnyAuthority` / `Require` / `Authorize`)。
- 不是密码学库。`TokenValidator` 是缝隙;JWT / opaque-token / session-cookie
  的具体实现在 starter 或调用方应用中。
- Web filter 链放在本包内,因为它只是普通 net/http 胶水且无外部依赖,并与
  aspect 侧的 `Require` 相对应,共同覆盖传输与方法两层。
- 不是 session 库(见 `stdlib/session`),不是 OAuth2 授权服务器(见
  `starter-oauth2-server`)。

## 2. 关键抽象与缝隙

- `TokenValidator`——单方法接口,同时驱动 `Authenticate` 中间件与 driver 注
  册表。实现必须并发安全,并对任何无法背书的凭证**返回非 nil error**,而不是
  返回 `Authenticated=false` 的 `Authentication`。
- `RegisterValidator` / `GetValidator` / `MustGetValidator`——driver-registry
  范式(空名/nil/重名一律 panic),与 `discovery.Register` /
  `resilience.RegisterDriver` 同构。
- `WithAuthentication` / `FromContext`——用未导出 key 类型的 ctx 传递,防碰
  撞。
- `Require(authorities...)`——`aspect.Interceptor`;读 `FromContext(jp.Context)`,
  缺失时返 `ErrUnauthenticated`,认证但缺权限时返 `ErrForbidden`,否则
  `Proceed`。这是**AOP 等价**的方法守卫,走 aspect 拦截链,而非字节码/注解
  移植。
- `Middleware = func(http.Handler) http.Handler`,`Chain(a,b,c)(h) ==
  a(b(c(h)))`——最外层优先。资源服务器规范顺序:`Chain(CORS, CSRF,
  Authenticate, Authorize)`。

## 3. 约束(禁止破坏)

- **`Authentication` 方法 nil-safe** 且 `!Authenticated` 一律 false。下游代
  码可以直接对 `FromContext` 取到的值调 `HasAnyAuthority`,不必 nil 判空;别
  引入破坏该性质的字段。
- **`Authenticate(v, required=false)`** 无 token 时必须让请求原样透传,不挂
  `Authentication`——"authority 决策由后续过滤器决定"。**非法** token 一律
  401;**缺失** token 仅在 `required=true` 时 401。
- **CORS 通配符与 credentials**:`AllowCredentials=true` 时不能发
  `Access-Control-Allow-Origin: *`——规范禁止。要回显具体 origin 并加
  `Vary: Origin`。
- **CSRF 是 double-submit-cookie**:服务端无状态。安全方法种下 cookie;非安
  全方法必须在 header 里回显该 cookie(常量时间比较)。它与 bearer-token API
  正交,后者不易 CSRF,不要强推。
- **非对称密钥场景绝不接受 HMAC**(算法混淆防护)——不变量在 starter 里体现,
  但这里点名,免得后续在本包新写 validator 时踩坑。

## 4. 权衡 / 未做的方案

- **不复刻 Spring Security filter 注册表**。顺序 = `Chain(...)` 的顺序;推理
  显式,没有看不见的优先级。
- **`Authorize` 覆盖 HTTP 层、`Require` 覆盖方法层**——同一权限集,两道闸。
  HTTP 层管路由,aspect 层管服务方法。都放本包保一致。
- **注册表不在请求路径解析 validator**。`Authenticate` 直接接 `TokenValidator`
  值;注册表用于装配期**查找** validator,不是每个请求都查。
- **无注解扫描**。`@PreAuthorize` 由显式的 `aspect.NewChain(security.Require(...))`
  取代——AOP 等价链。

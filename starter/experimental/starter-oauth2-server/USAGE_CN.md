# starter-oauth2-server 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`、`config.go`、`server.go`、`token.go`、`store.go`、`pkce.go`)并锚定可运行的
[example/](example/)。**OAuth2/OIDC 协议语义归 RFC——[RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749)
(框架、token 端点错误码)、[RFC 7636](https://datatracker.ietf.org/doc/html/rfc7636)(PKCE);
JWT/JWKS 格式见 [RFC 7515/7517](https://datatracker.ietf.org/doc/html/rfc7517)**——以下全部是
go-spring 增量:激活、绑定、挂载、seam。

**激活条件**:唯一开关 `spring.oauth2.server.enabled=true`,不设则 import 后不生效。
单 bean 模型:一个应用一个授权服务器;client 是配置数据,不是 bean。

---

## 1. 完整工程示例

一个同时是授权服务器(挂 `/oauth2`)和资源服务器(业务 API 挂 `/api`)的服务,
自签自发 HS256 token。example 跑的就是它——不需要外部身份提供商。

```
demo/
├── go.mod
├── main.go
└── conf/
    └── app.properties
```

**go.mod**(关键依赖):

```
require (
    github.com/golang-jwt/jwt/v5     v5.3.1
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-oauth2-server latest
    // 可选生态:starter-actuator / starter-otel / starter-governance
)
```

**main.go**——挂载、登录 seam、资源侧校验器:

```go
package main

import (
    "context"
    "fmt"
    "net/http"

    "github.com/golang-jwt/jwt/v5"
    "go-spring.org/cloud/experimental/security"
    "go-spring.org/spring/gs"
    StarterOAuth2Server "go-spring.org/starter-oauth2-server"
)

const secret = "example-shared-secret" // = spring.oauth2.server.secret

func main() {
    // 应用围绕注入的 *AuthServer 组装自己唯一的 mux。
    gs.Provide(func(as *StarterOAuth2Server.AuthServer) *gs.HttpServeMux {
        // 资源所有者登录 seam:真实应用在这里校验自己的会话。
        as.UserAuthFunc = func(*http.Request) (string, []string, bool) {
            return "alice", []string{"admin"}, true
        }

        validator := hmacValidator{secret: []byte(secret)}

        mux := http.NewServeMux()
        // 授权服务器:/oauth2/authorize、/oauth2/token、/oauth2/jwks。
        mux.Handle("/oauth2/", http.StripPrefix("/oauth2", as.Handler()))

        // 资源服务器:先认证后授权(有序链)。
        mux.Handle("/api/me", security.Chain(
            security.Authenticate(validator, true))(
            http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                a, _ := security.FromContext(r.Context())
                fmt.Fprintf(w, "hello %s", a.Principal.Subject)
            })))
        mux.Handle("/api/admin", security.Chain(
            security.Authenticate(validator, true), security.Authorize("admin"))(
            http.HandlerFunc(func(w http.ResponseWriter, *http.Request) {
                w.Write([]byte("admin ok"))
            })))
        return &gs.HttpServeMux{Handler: mux}
    })
    gs.Run()
}

// hmacValidator 是资源侧:用共享 secret 校验 HS256 token,把 scope+roles
// claim 映射成 authorities(实现 security.TokenValidator)。
type hmacValidator struct{ secret []byte }

func (v hmacValidator) Validate(_ context.Context, token string) (*security.Authentication, error) {
    claims := jwt.MapClaims{}
    tok, err := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"})).
        ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return v.secret, nil })
    if err != nil || !tok.Valid {
        return nil, fmt.Errorf("invalid token: %w", err)
    }
    subject, _ := claims["sub"].(string)
    authorities := append(claimStrings(claims["scope"]), claimStrings(claims["roles"])...)
    return &security.Authentication{
        Principal: security.Principal{Subject: subject, Claims: claims},
        Authorities: authorities, Authenticated: true,
    }, nil
}
```

(`claimStrings` 把空格分隔或数组形式的 claim 归一——从 example/example.go 复制。)

**conf/app.properties**——完整注释配置面:

```properties
# --- 激活(唯一开关)---------------------------------------------------------
spring.oauth2.server.enabled=true

# --- token 内容 / 生命周期 ----------------------------------------------------
# iss claim;资源侧如校验 issuer,必须两边一致。
spring.oauth2.server.issuer=https://issuer.example.com
spring.oauth2.server.access-token-ttl=1h
spring.oauth2.server.refresh-token-ttl=24h
spring.oauth2.server.code-ttl=1m

# --- 签名 key:恰好一个来源(这里用 HMAC)-------------------------------------
spring.oauth2.server.secret=example-shared-secret
# (非对称替代:private-key / private-key-file;公钥半发布在 /jwks)

# --- 登记的 client(配置数据,非 bean)----------------------------------------
# 公共 client(SPA):无 secret,强制 PKCE。
spring.oauth2.server.clients.spa.public=true
spring.oauth2.server.clients.spa.redirect-uris=http://127.0.0.1:9090/callback
spring.oauth2.server.clients.spa.scopes=read,write

# 只允许 client_credentials 的机密 client。
spring.oauth2.server.clients.svc.secret=svc-secret
spring.oauth2.server.clients.svc.scopes=read
spring.oauth2.server.clients.svc.grant-types=client_credentials

# --- 可选生态 -----------------------------------------------------------------
# spring.actuator.addr=:9370
# spring.observability.service-name=demo        (starter-otel)
```

**验证**(与 `example/check.sh` 同构;example 自断言全流程):

```bash
bash example/check.sh
```

手动往返(example 以 `-manual` 跑,base http://127.0.0.1:9090):

```bash
# 1. 授权(PKCE):302 带 code + state
V=$(openssl rand -hex 32)                       # 语义等同 GenerateVerifier()
C=$(printf %s "$V" | openssl dgst -sha256 -binary | basenc --base64url | tr -d '=')
curl -sD- -o/dev/null "http://127.0.0.1:9090/oauth2/authorize?response_type=code&client_id=spa&redirect_uri=http://127.0.0.1:9090/callback&scope=read+write&state=xyz&code_challenge=$C&code_challenge_method=S256" | grep -i '^location'

# 2. 用 code 换 token(CODE 取自上面 Location)
curl -s http://127.0.0.1:9090/oauth2/token -d grant_type=authorization_code \
  -d code=CODE -d redirect_uri=http://127.0.0.1:9090/callback \
  -d client_id=spa -d code_verifier="$V"

# 3. 带 access token 调受保护 API
curl -i -H "Authorization: Bearer $ACCESS_TOKEN" http://127.0.0.1:9090/api/me

# 4. 机密 client 的 client_credentials
curl -s http://127.0.0.1:9090/oauth2/token -d grant_type=client_credentials \
  -d client_id=svc -d client_secret=svc-secret -d scope=read

# 5. JWKS(HMAC 签名时空 key set)
curl -s http://127.0.0.1:9090/oauth2/jwks
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-oauth2-server
  └─ gs.Provide(newAuthServer, TagArg("${spring.oauth2.server}"))
         .Condition(OnProperty("spring.oauth2.server.enabled").HavingValue("true"))
        │
gs.Run()
  ├─ 条件门:enabled != true → 无 bean,starter 不生效
  ├─ 配置绑定:${spring.oauth2.server} → Config(value tag)
  ├─ 构造:newSigner 对歧义 key 快速失败
  │     (secret 与 private-key(-file) 零个或两个都有)→ 启动报错
  │     JWKS 文档预计算;内存 store 分配
  ├─ 你的 mux provider 运行:注入 *AuthServer、设 UserAuthFunc、
  │     把 Handler()(或单 handler)挂到你自己的 server
  ├─ Run/服务:/authorize、/token、/jwks 与业务路由同进程同端口
  └─ 停机:signer 与 store 无 goroutine/无可关闭资源 → 无 destroy hook
```

server **不监听任何端口**——它是 Contributor 形态的 bean;HTTP server 与路由归应用。
没有 `gs.Server`,本 starter 也没有端口 key。

### 2.2 一次完整 auth 走读(浏览器到受保护调用)

公共 client `spa` 的 authorization-code + PKCE,逐层:

1. **GET /oauth2/authorize**,带 `response_type=code`、`client_id`、`redirect_uri`、
   `scope`、`state`、`code_challenge`、`code_challenge_method=S256`:
   - 未知 `client_id` → 裸 400 "unknown client_id"(不重定向:没有可信的落点);
   - `redirect_uri` 不在 client 的精确允许列表 → 裸 400(开放重定向防护);
   - grant 不被允许 / `response_type != code` / PKCE method 非法 / scope 越界 →
     302 回 `redirect_uri`,带 `error=…` 与回显的 `state`;
   - 公共 client 缺 `code_challenge` → 302 `error=invalid_request`;
   - `UserAuthFunc` 为 nil → 503 "authorization_code flow not enabled"
     (client_credentials 不受影响);用户未认证 → 401;
   - 成功:存一张一次性 code(32 字节随机、base64url),携带授权上下文(client、
     redirect_uri、scopes、subject、authorities、PKCE challenge)和 `code-ttl` 过期;
     302 到 `redirect_uri?code=…&state=…`。
2. **POST /oauth2/token**(`grant_type=authorization_code`):client 身份取自 Basic
   auth 或 form body;机密 secret 恒时比较。code 原子消费(一次性),随后 client_id
   与 redirect_uri 必须与授权一致,再对 PKCE verifier 做哈希后与存储 challenge 恒时
   比较。任一失败 → RFC 6749 §5.2 错误 JSON(`invalid_client` 401 / `invalid_grant`
   400),`Cache-Control: no-store`。
3. **issueTokens**:access token 是 JWT(`sub`、`client_id`、`iat`、`exp`、`jti`、
   可选 `iss`、非空时的 `scope`、未进 scope 的 authorities 进 `roles`;header 带
   `kid`);refresh token 是不透明的 32 字节值,服务端记录带 `refresh-token-ttl`。
   响应是标准 `access_token`/`token_type`/`expires_in`/`refresh_token`/`scope` JSON。
4. **受保护调用**:`Authorization: Bearer <jwt>` 到你的路由;资源侧校验器(你的代码
   ——`starter-security-jwt`,或如本例内联 `security.TokenValidator`)验签/验期并把
   claim 映射成 authorities;`security.Authenticate` + `security.Authorize` 执法。
5. **刷新**:`grant_type=refresh_token` 轮换(一次性——即使响应丢失旧 token 也作废),
   且只能在原始 scope 内收窄。

client_credentials 跳过 1-2 的 code 机制:仅机密 client,client 即主体
(`sub = client_id`,scope 兼作 authorities),且**不发** refresh token。

### 2.3 store——什么存在哪、存多久

授权 code 与 refresh token 存**进程内存**(互斥锁下的普通 map):读时惰性查过期、
每次写入顺带清扫,因此没有后台 goroutine 也没有 destroy hook。code 与 refresh token
都是一次性,兑换时原子删除。store **没有** `OnMissingBean` seam:它在 server bean
内部构造,不是可替换 bean——多节点部署需要共享存储,超出本 starter 范围(见 §6)。

---

## 3. 逐 key 行为参考

`enabled` 是条件 key(门控 bean;不绑定进 Config)。其余 key 绑定在
`spring.oauth2.server.*` 下。

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `enabled` | bool | false | 激活开关;`HavingValue("true")`。 | 不设/false → starter 不生效,mux 上 `/oauth2/*` 404。 |
| `issuer` | string | — | 每个 token 的 `iss` claim;空则省略。 | 与资源侧钉死的 issuer 不一致 → 所有 token 在下游被拒。 |
| `algorithm` | string | — | 钉死签名算法;空则按 key 来源选:HMAC→HS256、RSA→RS256、EC→按曲线 ES256/384/512。HMAC 接受 HS256/384/512;RSA 接受 RS*/PS*;EC 必须匹配曲线。 | 不兼容值(如 `secret` 配 `RS256`)→ 启动快速失败(`errBadAlgForKey`)。 |
| `secret` | string | — | HMAC 签名 key;资源侧带外共享。⚠ 与 `private-key`/`private-key-file` 互斥。 | 无 PEM 又为空 → 启动失败(`errNoSigningKey`);两者都设 → `errBothSigning`。 |
| `private-key` | string | — | 内联 RSA/ECDSA PEM,非对称签名;公钥半发布在 /jwks。⚠ 与 `secret` 互斥;若同时设了 `private-key-file`,文件静默优先于内联值。 | PEM 解不开 → 启动失败("neither a valid RSA nor ECDSA PEM")。 |
| `private-key-file` | string | — | `private-key` 的文件路径替代;启动时读取。 | 路径读不了 → 启动失败,带解释的读取错误。 |
| `key-id` | string | `default` | token header 与 JWKS 条目的 `kid`;让轮换中的资源侧选对 key。 | 两台 server 的 JWKS 用了同一 `kid` → 选错 key。 |
| `access-token-ttl` | duration | 1h | access token 生命周期(`exp`);同时作为 `expires_in` 返回。 | 过长 → 授权过期后仍有效;过短 → 刷新频繁。 |
| `refresh-token-ttl` | duration | 24h | 服务端 refresh 记录的生命周期;每次使用都轮换。 | 比 access-token-ttl 短则刷新无意义;极长 = 长期授权。 |
| `code-ttl` | duration | 1m | 授权 code 生命周期;code 一次性。⚠ 保持短——code 本该立刻兑换。 | 窗口长则 code 被截获的暴露面大(公共 client 有 PKCE 兜底)。 |
| `clients.<id>.secret` | string | — | 机密 client 的凭据;/token 恒时比较。⚠ `public=true` 时无意义(被忽略)。 | 机密 client 留空 → secret 认证永远失败(`clientAuthenticated` 返回 false)。 |
| `clients.<id>.public` | bool | false | 公共 client(SPA/原生):无 secret,/authorize **强制** PKCE。 | 公共 client 不带 PKCE → 每次授权 302 `invalid_request`。 |
| `clients.<id>.redirect-uris` | []string | — | auth-code 重定向的精确匹配允许列表。 | 缺失/未列 URI → 裸 400,不重定向。 |
| `clients.<id>.scopes` | []string | — | 可授予 scope 的允许列表;空 = 不限制。 | 请求的 scope 越界 → `invalid_scope`。 |
| `clients.<id>.grant-types` | []string | — | grant 允许列表;空允许全部三种(`authorization_code`、`client_credentials`、`refresh_token`)。 | 越界 grant → `unauthorized_client`。 |

⚠ 耦合:`secret` vs `private-key`/`private-key-file`(恰好一个,启动校验);
`algorithm` vs key 来源(兼容性校验);`public=true` vs PKCE(协议级耦合);
`issuer` vs 资源侧校验。

---

## 4. 验证与故障演练

所有演练针对 `cd example && go run . -manual`(base :9090),或由
`bash example/check.sh` 自动断言。

### 4.1 正常路径与 token claim

```bash
# client_credentials 后查看 JWT(HS256 demo secret)
TOK=$(curl -s :9090/oauth2/token -d grant_type=client_credentials \
  -d client_id=svc -d client_secret=svc-secret -d scope=read | jq -r .access_token)
echo "$TOK" | cut -d. -f2 | basenc --base64url -d 2>/dev/null; echo
# → claim:sub=svc、client_id=svc、scope=read、jti、iat、exp、iss;无 refresh_token
```

### 4.2 token 过期

设 `spring.oauth2.server.access-token-ttl=2s`,取 token 后等 3 秒:

```bash
curl -i -H "Authorization: Bearer $TOK" :9090/api/me   # 你的校验器回 401
```

### 4.3 错误 client secret

```bash
curl -i :9090/oauth2/token -d grant_type=client_credentials \
  -d client_id=svc -d client_secret=WRONG
# 401 {"error":"invalid_client","error_description":"client authentication failed"}
```

### 4.4 PKCE 失败(verifier 不匹配)

用 verifier A 导出的 challenge 授权,却用 verifier B 兑换:

```bash
curl -s :9090/oauth2/token -d grant_type=authorization_code -d code=$CODE \
  -d redirect_uri=http://127.0.0.1:9090/callback -d client_id=spa \
  -d code_verifier=the-wrong-verifier
# 400 {"error":"invalid_grant","error_description":"PKCE verification failed"}
```

### 4.5 一次性:code 与 refresh 重放

同一 code 兑换两次——第二次 `invalid_grant` "code invalid or expired"。用过的 refresh
token 同理(轮换)。过期 code(超过 `code-ttl`)也失败。

### 4.6 公共 client 被挡在 client_credentials 之外

```bash
curl -i :9090/oauth2/token -d grant_type=client_credentials -d client_id=spa
# 401 invalid_client——公共 client 不能用该 grant
```

### 4.7 /jwks 与签名模式

- HMAC(`secret`):`curl :9090/oauth2/jwks` → `{"keys":[]}`——无可发布;资源侧带外
  共享 secret。
- RSA/EC(`private-key-file`):公钥带 `kid`、`use: sig`、`alg` 出现。
- ⚠ 演示脚坑(源自 DESIGN):RSA + JWKS URL 指回*本进程*时,急引导的校验器会与
  尚未开始服务的 server 死锁——单进程演示用 HMAC。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "no signing key configured" | `secret` 和 `private-key`/`private-key-file` 都没设 | 恰好设一个。 |
| 启动失败 "both HMAC secret and PEM private key" | 两个 key 来源都设了 | 去掉一个。 |
| 启动失败 "algorithm is not compatible" | `algorithm` 与 key 来源不符 | 匹配或留空(自动)。 |
| `/oauth2/*` 404 | `enabled` 不为 true(starter 不生效) | 设 `spring.oauth2.server.enabled=true`。 |
| /authorize 返回 503 | `UserAuthFunc` 为 nil | 在 mux provider 里设置(只影响 auth-code 流)。 |
| /authorize 裸 400 而非重定向 | 未知 `client_id` 或 `redirect_uri` 未登记 | 登记 client / 精确列出 URI。 |
| 公共 client 在 /authorize 恒 `invalid_request` | 缺 `code_challenge` | `public=true` 强制 PKCE。 |
| 换 token `invalid_grant` "code does not match client/redirect_uri" | 兑换用的 client/redirect 与授权不一致 | 用同一对兑换。 |
| refresh `invalid_grant` | token 已用过(轮换)或已过期 | 用最新的 refresh token;丢失即重登。 |
| 单节点正常,LB 后 code 被拒 | 内存 store 按进程隔离 | 超范围;需要共享存储(见 §6)。 |
| 资源侧拒掉所有 token | 两半之间 secret/issuer/algorithm 不一致 | 对齐 key 材料与 `issuer`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | server 10 + 每 client 5(+1 条件 key `enabled`) |
| 必填 | 2(`enabled=true` + 恰好一个签名 key) |
| quickstart 前置外部依赖 | 0 |
| "注意/坑"条数 | 5 |

设计嫌疑(待设计裁决;保留旧 USAGE/DESIGN 已有条目,新增写作中发现的):

- code 与 refresh token 存进程内存——仅单节点;多节点需要共享存储(既有)。store 是
  内部实现且无 `OnMissingBean`/bean seam,DESIGN 里"未来可贡献 `CodeStore` bean"目前
  并无实际扩展点(新)。
- `UserAuthFunc` 是注入后设置的公开可变字段,不是配置驱动或 bean 驱动的 seam(既有)。
- /authorize 在请求内同步认证资源所有者(没有自己的登录跳转):`UserAuthFunc` 返回
  ok=false 时是裸 401,不是 OAuth2 错误重定向——SPA 需自行处理(新,轻微)。
- /token 无限流/防爆破防护(secret 比较是恒时的,但尝试次数无上限)(新)。
- `private-key` 与 `private-key-file` 同时设置时静默文件优先——无告警(新)。
- 只支持 `authorization_code`、`client_credentials`、`refresh_token`;无 password/device/
  扩展 grant,无 introspection/revocation 端点(范围边界,重述入账)。

# starter-oauth2-resource-server 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`config.go`、`jwks.go`、`starter.go`、`validator.go`、`validator_test.go`）与可运行的
[example/](example/)（冒烟脚本 `example/check.sh`）核验。**JWT claim 语义属于
[RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519) 与
[jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)；发现机制属于
[OIDC Discovery](https://openid.net/specs/openid-connect-discovery-1_0.html)** ——
本文只写 go-spring 的增量：绑定、bean、fail-fast 装配与 key source 纪律。

**激活方式**：`spring.security.oauth2.resource.jwt.<name>` 下每个条目生成一个
`*Validator` bean；空 map 什么都不注册 —— 配置即开关（starter.go:37-49）。
**每个条目必须且只能配一个 key source**（`issuer-uri` / `public-key(-file)` / `secret`）；
配零个或多个都会让启动失败（config.go:94-113）。

---

## 1. 完整工程示例

一个接受授权服务器签发 bearer token 的订单 API。下面用 HMAC secret 模式（与 example/
一致）做到零外部依赖即可跑；生产换成 issuer-uri 只需改一个 key（§3）。文件树：

```
demo/
├── go.mod
├── main.go
├── router.go
├── token.go          # 仅开发期本地铸 token（顶替授权服务器）
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/golang-jwt/jwt/v5 v5.3.1      // 仅自己铸开发 token 时需要
    go-spring.org/spring                     v1.3.x
    go-spring.org/cloud                      latest   // security seam 在这里
    go-spring.org/starter-oauth2-resource-server latest
    go-spring.org/starter-actuator           latest   // 可选：探针
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "demo/router"
)

func main() { gs.Run() }
```

**router.go** —— 全部 HTTP 面。starter 把 validator 导出为框架无关的
`security.TokenValidator` seam，应用用 `security.Authenticate` / `security.Authorize`
组合，不依赖 starter 的具体类型：

```go
package router

import (
    "net/http"

    "go-spring.org/cloud/experimental/security"
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-oauth2-resource-server" // 按配置注册 validator
)

func init() {
    // gs.TagArg("api") 选中配置在 spring.security.oauth2.resource.jwt.api.*
    // 下的那个 validator。
    gs.Provide(func(v security.TokenValidator) *gs.HttpServeMux {
        mux := http.NewServeMux()

        // 身份回显：能进这个 handler 就说明 bearer token 已验证通过。
        mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            _, _ = w.Write([]byte("hello " + a.Principal.Subject))
        })

        // 路由级 authority 门槛：需要 orders:read scope/authority。
        mux.Handle("/orders", security.Authorize("orders:read")(
            http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
                _, _ = w.Write([]byte("orders ok"))
            })))

        // 先认证后鉴权 —— security kit 的链式顺序。
        handler := security.Chain(
            security.Authenticate(v, true), // required=true：无 token -> 401
            security.Authorize(),           // 兜底：要求已认证
        )(mux)
        return &gs.HttpServeMux{Handler: handler}
    }, gs.TagArg("api"))
}
```

**token.go** —— 仅开发期铸 token（生产由授权服务器签发；语义见
[RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519#section-3)）：

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

const secret = "example-shared-secret" // 必须与 ...jwt.api.secret 一致

func mint(subject string, scopes ...string) string {
    claims := jwt.MapClaims{
        "sub":   subject,
        "aud":   "example-api", // 必须匹配 ...jwt.api.audiences
        "exp":   time.Now().Add(time.Hour).Unix(),
        "scope": scopes, // 数组形式；空格分隔字符串也接受
    }
    s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
    if err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
    return s
}
```

**conf/app.properties** —— 完整、带注释的配置面：

```properties
# --- resource server（本 starter）--------------------------------------------
# 每个 validator 一个条目。每个条目只能配一个 key source：
#   issuer-uri  （生产推荐：OIDC discovery -> JWKS，自动轮换感知）
#   public-key / public-key-file  （静态 RSA/ECDSA PEM）
#   secret  （共享 HMAC；仅开发/演示）
spring.security.oauth2.resource.jwt.api.secret=example-shared-secret

# 可接受的 "aud" 值（any-of）。留空关闭 audience 检查。
spring.security.oauth2.resource.jwt.api.audiences=example-api

# 生产环境接真实 issuer 时，把上面两行换成：
#   spring.security.oauth2.resource.jwt.api.issuer-uri=https://auth.example.com
# （预期 "iss" 默认取 issuer URI；JWKS 自动发现）

# --- http server（gs 核心）---------------------------------------------------
# validator 不持有端口；gs 内置 server 在 :9090 上服务注入的 mux。
# spring.http.server.addr=:9090    # 显式配置更符合项目惯例

# --- actuator（可选）---------------------------------------------------------
spring.actuator.addr=:9370
```

**验证**（与 example/check.sh 同构 —— 它在进程内跑同样五条断言；这里用
`-manual` + curl）：

```bash
go run . -manual &                          # 服务保持运行
TOKEN=$(go run ./cmd/mint alice orders:read)  # 包一层 token.go mint 的小 main)

curl -i :9090/me                                   # 401 missing bearer token
curl -i -H "Authorization: Bearer $TOKEN" :9090/me # 200 "hello alice"
curl -i :9370/healthz                              # actuator liveness
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-oauth2-resource-server
  └─ gs.Module(gs.OnProperty("spring.security.oauth2.resource.jwt"))   [starter.go:37]
        │  （前缀检查：任意 spring.security.oauth2.resource.jwt.* 条目都触发）
gs.Run()
  ├─ 绑定：每个子 key -> Config（value tag），随即内联校验 Config.source()
  │        （starter.go:39-41）—— "no/multiple key source" 在任何 bean 产生前
  │        失败，错误里带实例名
  ├─ r.Provide(newValidator, gs.ValueArg(c)).Name(name)
  │        .Export(gs.As[security.TokenValidator]())                    [starter.go:43-46]
  ├─ newValidator（validator.go:59-106），按 source：
  │     HMAC ： keyfunc 返回 secret                              —— 无 I/O
  │     PEM  ： parsePEMPublicKey（先 RSA 后 ECDSA；file 优先于 inline）—— fail fast
  │     Issuer： discoverJWKSURI  GET {issuer-uri}/.well-known/openid-configuration
  │             再 newJWKSCache -> 首次拉取 JWKS                   —— fail fast：
  │             issuer 不可达、PEM 非法或 key set 为空都会中止启动
  ├─ 构建 parser：jwt.WithValidMethods(...)、WithLeeway、非空时 WithIssuer
  ├─ 应用装配：security.TokenValidator 按名注入你的 mux provider
  ├─ 开始服务；请求流经 Authenticate -> Authorize -> handler
  └─ SIGTERM：无需清理 —— JWKS 缓存按需刷新，没有后台 goroutine
     （jwks.go:35-39、starter 注释）
```

### 2.2 一次请求的逐层走读 —— 带 bearer token 的 `GET /orders`

1. **Authenticate**（security/middleware.go:72）：`BearerToken(r)` 抽取
   `Authorization: Bearer <token>` 头；缺失且 `required=true` → 401
   `WWW-Authenticate: Bearer`（writeUnauthorized）。
2. **Validator.Validate**（validator.go:189-211）：`parser.ParseWithClaims` 走
   source 对应的 keyfunc：
   - **签名**：算法先由 `WithValidMethods(validMethods)` 过滤 —— 非对称 source
     的允许集只有 RS/ES/PS，**永远不含 HS***（validator.go:110-129），因此即使
     不 pin `algorithm`，"拿公钥当 HMAC secret 签名"的算法混淆攻击也被结构性封死；
   - **issuer-uri source**：token 头里的 `kid` 在 JWKS 缓存中选 key；`kid`
     未知或缓存超过 `jwks-refresh` 会触发一次同步 reload（不等周期即可吸收
     key 轮换）；reload 失败但缓存里还有该 key 时继续用旧 key（jwks.go:66-88）；
   - **claims**：`exp`/`nbf`（含 `leeway`）、派生非空时的 `iss`
     （validator.go:101-103），再对 `audiences` 做 any-of 校验
     （validator.go:198-200）—— 每种失败都是 error，绝不返回"未认证的成功"。
3. 成功后把 `Authentication{Subject, Claims, Authorities}` 挂到请求 context；
   authorities = `scope-claim` + `roles-claim` 展开的结果（空格分隔字符串或
   JSON 数组皆可，validator.go:230-247）。
4. **Authorize("orders:read")**（security/middleware.go:102）：匿名 → 401；
   已认证但缺 authority → 403；否则进 handler。
5. 所有失败路径都返回 401 带简短原因（`missing bearer token`、`invalid token`）——
   parser 的具体错误（过期、issuer 不对……）由 `Validate` 返回，但 HTTP 边缘
   统一渲染，避免泄露校验细节。

---

## 3. 逐 key 行为参考

key 位于 `spring.security.oauth2.resource.jwt.<name>.*`，共 12 个（与
`grep -rhoE 'value:"[^"]+"' | sort -u` 完全一致）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `issuer-uri` | string | — | key source ①：在 `{issuer-uri}/.well-known/openid-configuration` 做 OIDC discovery → `jwks_uri`（容忍尾斜杠，validator.go:134-136）。同时是预期 `iss`（`issuer` 可覆盖）。 | 启动时 discovery/JWKS 不可达 → 启动失败。 |
| `public-key` | string | — | key source ②a：inline RSA/ECDSA PEM。 | 非 PEM → 启动失败（"neither a valid RSA nor ECDSA PEM"）。 |
| `public-key-file` | string | — | key source ②b：PEM 文件路径；**优先于 `public-key`**（两者同设仍算一个 source，validator.go:169-175）。 | 路径读不到 → 启动失败。 |
| `secret` | string | — | key source ③：HS256/384/512 的 HMAC secret。 | 与任一其它 source 同设 → 启动失败（multiple sources）。 |
| `issuer` | string | — | 覆盖预期 `iss`（如内部 issuer 与公网 URL 不同）。 | 配错 → 所有 token 401（`invalid token`）。 |
| `audiences` | []string | — | 可接受的 `aud` 值，**any-of**；留空关闭检查。 | token 的 `aud` 不同 → 401（`token audience not accepted`）。 |
| `algorithm` | string | — | pin 单个算法（大小写不敏感）；必须与 source 兼容。留空 = 全兼容集。非对称 source 永不接受 HMAC。 | 不兼容的 pin（如 PEM 配 `HS256`）→ **启动失败**（validator.go:128）。 |
| `jwks-refresh` | duration | 15m | JWKS 缓存 TTL；`kid` 未知也会立即触发 reload。⚠ 对 HMAC/PEM source 是死 key。 | 过长会把轮换感知推迟到 kid 触发路径。 |
| `jwks-timeout` | duration | 10s | discovery **和** JWKS 每次拉取的 HTTP 超时。⚠ 对 HMAC/PEM source 是死 key。 | 过小 → 刷新失败（退回旧 key）/ 启动抖动。 |
| `scope-claim` | string | `scope` | 展开进 Authorities 的 claim（空格分隔或数组）。 | 名字配错 → 基于 scope 的 Authorize 门槛全变 403。 |
| `roles-claim` | string | `roles` | 同上，追加在 scope 之后。 | 同上。 |
| `leeway` | duration | 0 | exp/nbf/iat 的时钟偏差容忍。 | 过大接受已过期 token；0 会拒绝边界时钟。 |

⚠ 耦合：`{issuer-uri}`、`{public-key, public-key-file}`、`{secret}` 三选一；
`public-key` + `public-key-file` 同设算一个 source（file 优先）。
⚠ `algorithm` 必须属于所选 source 的家族。

---

## 4. 验证与故障演练

所有演练针对 §1 工程（`go run . -manual`），铸 token 用 token.go 的 mint。

1. **正常往返**：合法 token → `200 hello alice`；带 scope 的 token → `200 orders ok`。
2. **缺 token**：
   `curl -s -o/dev/null -w '%{http_code}\n' :9090/me` → `401`，body
   `missing bearer token`，响应头 `WWW-Authenticate: Bearer`。
3. **错误 key source（secret 不对）**—— 用 `[]byte("wrong")` 铸 token：
   `curl -s -H "Authorization: Bearer $FORGED" :9090/me` → `401 invalid token`
   （example/check.sh 的 Feature 5 断言的就是这个）。
4. **过期 token** —— 铸 `"exp": time.Now().Add(-time.Minute).Unix()` 的 token：
   → `401`。再铸 `+30s` 过期、同时配 `spring.security...api.leeway=1m`：
   变为接受 —— leeway 生效。
5. **算法混淆攻击尝试** —— 配 `public-key` source 后，用**公钥 PEM 本身当 HMAC
   secret** 签一个 HS256 token：依旧 `401` —— `validMethods` 对非对称 source
   永不放开 HS*（validator.go:113-119），该保护不依赖 pin。若硬配
   `algorithm=HS256` + PEM source，则直接在**启动期**失败。
6. **issuer/audience 缺失** —— 配 `issuer-uri=https://auth.example.com` 后铸
   `iss` 为其它值的 token → `401`；配了 `audiences=example-api` 但 token 无
   `aud` claim → `401`（无 aud 无法满足 any-of）。
7. **key source 配置错误** —— 同时配 `secret` 和 `issuer-uri`（或都不配），
   重启动：启动中止并报 `multiple/no verification key source configured ...
   instance "api"`。
8. **JWKS 轮换**（issuer-uri 部署）：先发布含 kid `k1` 的 JWKS 并验证 `k1`
   token；改为只发布 `k2`，再铸 `k2` token —— 首个请求触发按需 reload
   （jwks.go:66-88），不重启即通过。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动报 `no verification key source configured` | 该实例没配 key | 补 `issuer-uri`、`public-key(-file)` 或 `secret` |
| 启动报 `multiple verification key sources configured` | 如 `secret` + `issuer-uri` 同设 | 只留一组 |
| 启动报 `fetch discovery document ... status ...` | issuer 挂了 / URL 错 / TLS 问题 | 修 `issuer-uri`；查 `jwks-timeout` |
| 启动报 `JWKS ... contains no usable key` | 端点只有 okp/ed25519 或垃圾数据 | 只解析 RSA + EC P-256/384/521 的 JWK（jwks.go:147-156） |
| 所有请求 401 `invalid token` | issuer/audience/algorithm 不匹配或 key 不对 | 核对 `iss` vs `issuer-uri`、`aud` vs `audiences`、铸 token 的 key |
| token 换版后 scope 门槛全 403 | authority claim 改名或 scope-claim 不匹配 | 比对 token 的 claim 名与 `scope-claim`/`roles-claim` |
| 新轮换 key 的 token 短暂 401 | 缓存尚未过期 | 未知 `kid` 已会强制 reload；确认新 key 确实已发布 |
| 启动报 `algorithm ... not compatible` | pin 与 key source 家族矛盾 | `algorithm` 对齐 source（PEM/issuer 用 RS/ES/PS，secret 用 HS） |
| 完全没注册 validator bean | 前缀拼错 / map 为空 | key 必须位于 `spring.security.oauth2.resource.jwt.<name>.*` |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 12 |
| 其中必填 | 每实例 1 个 key source |
| quickstart 外部依赖 | 0（HMAC 模式）；生产模式 1 个 OIDC issuer |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（保留 + 新增，供设计裁决）：

- **保留**：与 `starter-security-jwt` 高度重叠 —— parser/JWKS/校验核心相同，
  差异主要在前缀与 `issuer-uri` discovery；是共享 validator 模块的候选。
- 新增：`jwks.go` 与 jwt starter 的同名文件逐字节重复（仅模块前缀不同）——
  应合并为公共内部包。
- 新增：JWKS reload 失败时无上限地继续用旧缓存 key（jwks.go:75-79）——
  长时间故障会静默让旧 key 持续有效。
- 新增：认证失败无可观测性（401 路径无计数/日志）—— 安全姿态在指标里不可见。
- 新增：`algorithm` 大小写不敏感，但 `audiences` 的 any-of 语义与 leeway 默认 0
  对多时钟部署是隐形坑；除笼统的 body 文本外没有任何"为什么 401"的出口。
- 新增：`Authenticate` 的 `required` 在本 starter 由应用侧传参，而 jwt starter
  有配置 key `required` —— 兄弟 starter 之间姿态旋钮不一致。

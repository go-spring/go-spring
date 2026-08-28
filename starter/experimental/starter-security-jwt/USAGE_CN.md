# starter-security-jwt 使用说明 — 参考手册

详细使用参考。总览见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`config.go`、`jwks.go`、`jwt.go`、`starter.go`、`jwt_test.go`）与可运行的
[example/](example/)（冒烟脚本 `example/check.sh`）核验。**JWT claim 语义属于
[RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519) 与
[jwt/v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5)** —— 本文只写 go-spring
的增量：绑定、bean、`Wrap(http.Handler)` seam 与 fail-fast 的 key source 纪律。

**激活方式**：`spring.security.jwt.<name>` 下每个条目生成一个 `*Authenticator`
bean；空 map 什么都不注册 —— 配置即开关（starter.go:35）。**每个条目必须且只能
配一个校验 key source**（`secret` / `public-key(-file)` / `jwks-url`）；配零个或
多个都会让启动失败（config.go:97-116）。本 starter 只**验证** token，不负责签发 ——
生产环境接你的签发方（演示期可像 example/ 那样在开发二进制里用 jwt/v5 自己铸）；
基于 OIDC discovery 的变体见兄弟 starter
[starter-oauth2-resource-server](../starter-oauth2-resource-server/)。

---

## 1. 完整工程示例

一个小 API：`/me` 回显已验证的身份，`/admin` 要求 `admin` authority。HMAC 模式
零外部依赖；生产换成 `jwks-url`（§3）。文件树：

```
demo/
├── go.mod
├── main.go
├── router.go
├── token.go          # 仅开发期铸 token（顶替身份提供方）
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/golang-jwt/jwt/v5 v5.3.1      // 仅自己铸开发 token 时需要
    go-spring.org/spring                     v1.3.x
    go-spring.org/cloud                      latest   // security seam 在这里
    go-spring.org/starter-security-jwt       latest
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

**router.go** —— 全部 HTTP 面。`Wrap` 是 seam：它装饰的是普通 `http.Handler`，
gin/echo/net-http 任何引擎都可以放在它后面：

```go
package router

import (
    "net/http"

    "go-spring.org/cloud/experimental/security"
    "go-spring.org/spring/gs"
    StarterSecurityJWT "go-spring.org/starter-security-jwt"
)

func init() {
    // gs.TagArg("api") 选中配置在 spring.security.jwt.api.* 下的 authenticator。
    // gs 只在没有自定义 HttpServeMux 时才注册默认的，所以这里提供的会胜出。
    gs.Provide(func(auth *StarterSecurityJWT.Authenticator) *gs.HttpServeMux {
        mux := http.NewServeMux()

        // 能进这个 handler 就说明 bearer token 已在 Wrap 中验证通过。
        mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            _, _ = w.Write([]byte("hello " + a.Principal.Subject))
        })

        // 认证之上的方法级 authority 检查。
        mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
            a, _ := security.FromContext(r.Context())
            if !a.HasAuthority("admin") {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            _, _ = w.Write([]byte("admin ok"))
        })

        return &gs.HttpServeMux{Handler: auth.Wrap(mux)}
    }, gs.TagArg("api"))
}
```

**token.go** —— 仅开发期铸 token（生产 token 来自你的身份提供方；语义见
[RFC 7519](https://datatracker.ietf.org/doc/html/rfc7519#section-3)）：

```go
package main

import (
    "fmt"
    "os"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

const secret = "example-shared-secret" // 必须与 spring.security.jwt.api.secret 一致

func mint(subject string, roles ...string) string {
    claims := jwt.MapClaims{
        "sub":   subject,
        "exp":   time.Now().Add(time.Hour).Unix(),
        "roles": roles, // 匹配 roles-claim；数组形式，空格分隔字符串亦可
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
# --- security-jwt（本 starter）-----------------------------------------------
# 每个 authenticator 一个条目。每个条目只能配一个校验 key source：
#   secret                （共享 HMAC；开发/演示）
#   public-key / public-key-file  （静态 RSA/ECDSA PEM）
#   jwks-url              （远程 JWKS；启动即拉取，按 kid 轮换感知）
spring.security.jwt.api.secret=example-shared-secret

# 承载 roles 的 claim；展开进 Authentication.Authorities。
spring.security.jwt.api.roles-claim=roles

# true（默认）：缺 token -> 401。false：放行且不带身份，
# 由方法级 guard 决定。
spring.security.jwt.api.required=true

# 接远程签发方时，把 `secret` 换成例如：
#   spring.security.jwt.api.jwks-url=https://auth.example.com/.well-known/jwks.json
#   spring.security.jwt.api.issuer=https://auth.example.com
#   spring.security.jwt.api.audience=demo-api

# --- http server（gs 核心）---------------------------------------------------
# authenticator 不持有端口；gs 内置 server 在 :9090 上服务注入的 mux。
# spring.http.server.addr=:9090    # 显式配置更符合项目惯例

# --- actuator（可选）---------------------------------------------------------
spring.actuator.addr=:9370
```

**验证**（与 example/check.sh 同构 —— 它在进程内跑同样五条断言；这里用
`-manual` + curl）：

```bash
go run . -manual &                                # 服务保持运行
TOKEN=$(go run ./cmd/mint alice user)             # 包一层 token.go mint 的小 main
ADMIN=$(go run ./cmd/mint root admin)

curl -i :9090/me                                     # 401 missing bearer token
curl -i -H "Authorization: Bearer $TOKEN" :9090/me   # 200 "hello alice"
curl -i -H "Authorization: Bearer $TOKEN" :9090/admin # 403 forbidden
curl -i -H "Authorization: Bearer $ADMIN" :9090/admin # 200 "admin ok"
curl -i :9370/healthz                                # actuator liveness
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-security-jwt
  └─ gs.Group("${spring.security.jwt}", newAuthenticator, nil)      [starter.go:35]
gs.Run()
  ├─ 绑定：每个子 key -> Config（value tag）
  ├─ newAuthenticator（jwt.go:57-103），按 source —— 全部 fail fast：
  │     source()：secret / PEM / jwks-url 恰好一个，否则启动报错    [config.go:97-116]
  │     HMAC ： keyfunc 返回 secret                              —— 无 I/O
  │     PEM  ： parsePEMPublicKey（先 RSA 后 ECDSA；file 优先于 inline）[jwt.go:130-146]
  │     JWKS ： newJWKSCache -> 启动期首次拉取                    —— 端点
  │             不可达或 key set 为空都会中止启动                   [jwks.go:52-62]
  ├─ 构建 parser：jwt.WithValidMethods(...)、WithLeeway、非空时 WithIssuer
  ├─ 应用装配：*Authenticator 按名（gs.TagArg）注入你的 mux provider；
  │     Wrap 装饰业务 mux -> 自定义 *gs.HttpServeMux 取代 gs 默认的
  ├─ 开始服务；每个请求：Wrap ->（JWKS 时）查 key -> Validate -> handler
  └─ SIGTERM：无需清理 —— JWKS 缓存按需刷新、没有后台 goroutine，
     所以 group 的 destroy hook 为 nil                            [starter.go:33-34]
```

### 2.2 一次请求的逐层走读 —— 带 bearer token 的 `GET /me`

1. **Wrap**（jwt.go:182-200）：`bearerToken(r)` 抽取
   `Authorization: Bearer <token>`（前缀大小写不敏感，jwt.go:204-211）。
2. 无 token：`Required`（默认 true）→ 401 `missing bearer token`，响应头
   `WWW-Authenticate: Bearer error="invalid_token"`；false → **不带身份**放行，
   决定权交给方法级 guard。
3. 有 token → `Validate`（jwt.go:150-172）：`parser.ParseWithClaims`：
   - **先做算法筛查**：`WithValidMethods(validMethods)` —— 非对称 source 的
     允许集只有 RS256/384/512、ES256/384/512、PS256/384/512，**永远不含 HS***
     （jwt.go:110-126）。这从结构上封死了经典的算法混淆攻击（"拿公钥当 HMAC
     secret 签名"），且不依赖是否 pin 了 `algorithm`；
   - **key 解析**：HMAC → 配置的 secret；PEM → 解析出的 key；JWKS → token 头
     `kid` 在缓存中选 key；`kid` 未知或缓存超过 `jwks-refresh` 触发一次同步
     reload（不等周期即可吸收轮换）；reload 失败但缓存里有该 key 时继续用旧
     key（jwks.go:66-88）；
   - **claims**：签名、`exp`/`nbf`（含 `leeway`）、配置时的 `iss`
     （jwt.go:98-100），再对 `audience` 做 any-of 校验（jwt.go:159-161）。
     任一失败都是 error，绝不返回"未认证的成功"。
4. 成功：经 `security.WithAuthentication` 把
   `Authentication{Subject, Claims, Authorities}` 挂到请求 context（jwt.go:198）；
   authorities = `scope-claim` + `roles-claim` 展开（空格分隔字符串或 JSON 数组，
   jwt.go:236-253）。
5. handler 用 `security.FromContext` 读取，并以
   `HasAuthority`/`HasAnyAuthority`（security.go:81-107）设门槛。非法 token 恒定
   401 `invalid token` —— parser 的具体原因不外泄给客户端。

---

## 3. 逐 key 行为参考

key 位于 `spring.security.jwt.<name>.*`，共 13 个（与
`grep -rhoE 'value:"[^"]+"' | sort -u` 完全一致）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|------------|---------|
| `secret` | string | — | key source ①：HS256/384/512 的 HMAC secret。 | 与其它 source 同设 → 启动失败（multiple sources）。 |
| `public-key` | string | — | key source ②a：inline RSA/ECDSA PEM。 | 非 PEM → 启动失败（"neither a valid RSA nor ECDSA PEM"）。 |
| `public-key-file` | string | — | key source ②b：PEM 文件路径；**优先于 `public-key`**（同设仍算一个 source，jwt.go:131-138）。 | 路径读不到 → 启动失败。 |
| `jwks-url` | string | — | key source ③：远程 JWKS 端点；启动即拉取，`kid` 选 key。 | 不可达 / 空 key set → 启动失败。 |
| `jwks-refresh` | duration | 15m | JWKS 缓存 TTL；`kid` 未知也会立即触发 reload。⚠ 对 HMAC/PEM source 是死 key。 | 过长会把轮换感知推迟到 kid 触发路径。 |
| `jwks-timeout` | duration | 10s | JWKS 每次拉取的 HTTP 超时。⚠ 对 HMAC/PEM source 是死 key。 | 过小 → 刷新失败（退回旧 key）/ 启动抖动。 |
| `issuer` | string | — | 预期 `iss`；留空**关闭** issuer 检查。 | 配错 → 所有 token 401。 |
| `audience` | []string | — | 可接受的 `aud` 值，**any-of**；留空关闭检查。 | token 的 `aud` 不同/缺失 → 401（`token audience not accepted`）。 |
| `algorithm` | string | — | pin 单个算法（大小写不敏感）；必须与 source 兼容。留空 = 全兼容集。非对称 source 永不接受 HMAC。 | 不兼容的 pin（如 PEM 配 `HS256`）→ **启动失败**（jwt.go:125）。 |
| `scope-claim` | string | `scope` | 展开进 Authorities 的 claim（空格分隔或数组）。 | 名字配错 → 基于 scope 的守卫全变 403。 |
| `roles-claim` | string | `roles` | 同上，追加在 scope 之后。 | 同上。 |
| `leeway` | duration | 0 | exp/nbf/iat 的时钟偏差容忍。 | 过大接受已过期 token；0 会拒绝边界时钟。 |
| `required` | bool | true | false = 缺 token 不带身份放行（Wrap，jwt.go:185-191）。**非法** token 依旧 401。 | false 且下游无 guard → 端点静默匿名。 |

⚠ 耦合：`{secret}`、`{public-key, public-key-file}`、`{jwks-url}` 三选一；
`public-key` + `public-key-file` 同设算一个 source（file 优先）。
⚠ `algorithm` 必须属于所选 source 的家族。

---

## 4. 验证与故障演练

所有演练针对 §1 工程（`go run . -manual`），铸 token 用 token.go。

1. **正常往返**：user token → `200 hello alice`；admin token → `200 admin ok`；
   user token 访问 `/admin` → `403`（example/check.sh 断言了这三条）。
2. **缺 token**：`curl -s -o/dev/null -w '%{http_code}\n' :9090/me` → `401`，
   body `missing bearer token`，响应头
   `WWW-Authenticate: Bearer error="invalid_token"`。
3. **垃圾/非法 token**：
   `curl -s -H "Authorization: Bearer not-a-real-token" :9090/me` → `401 invalid
   token`（check.sh Feature 5）。用错误 secret（`[]byte("wrong")`）铸的 token
   同样 401。
4. **过期 token** —— 铸 `"exp": time.Now().Add(-time.Minute).Unix()` 的 token：
   → `401`。再铸 `+30s` 过期、同时配 `spring.security.jwt.api.leeway=1m`：
   变为接受 —— leeway 生效。
5. **算法混淆攻击被拒** —— 配 `public-key` source 后，用**公钥 PEM 本身当 HMAC
   key** 签一个 HS256 token：`curl -s -H "Authorization: Bearer $CONFUSED" :9090/me`
   → `401` —— `validMethods` 对非对称 source 永不放开 HS*（jwt.go:113-119），
   pin 与否都一样。若硬配 `algorithm=HS256` + PEM source，则直接在**启动期**失败
   （`algorithm "HS256" is not compatible ...`）。
6. **issuer/audience 缺失** —— 配 `issuer=https://auth.example.com` 后铸 `iss`
   不同的 token → `401`；配 `audience=demo-api` 但 token 无 `aud` claim →
   `401`（无 aud 无法满足 any-of）。
7. **错误 key source / JWKS 演练** —— 把 `secret` 换成
   `jwks-url=http://127.0.0.1:8080/jwks` 且无人监听，重启：启动中止
   （`fetch JWKS ...`）。随后发布含 kid `k1` 的 JWKS 并验证 `k1` token；改为只
   发布 `k2` 再铸 `k2` token —— 首个请求触发按需 reload（jwks.go:66-88），
   不重启即通过。
8. **required=false 姿态** —— 配 `spring.security.jwt.api.required=false` 重启：
   `curl -s :9090/me` 不再 401（handler 看不到身份 —— 需自行守卫，例如
   `a, ok := security.FromContext(...); !ok` → 401）。垃圾 token 依旧 401。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|---------|------|
| 启动报 `no verification key source configured` | 该实例没配 key | 补 `secret`、`public-key(-file)` 或 `jwks-url` |
| 启动报 `multiple verification key sources configured` | 如 `secret` + `jwks-url` 同设 | 只留一组 |
| 启动报 `fetch JWKS ...: status ...` / `contains no usable key` | 端点挂了 / URL 错 / 只有 okp/ed25519 key | 修 `jwks-url`；只解析 RSA + EC P-256/384/521 的 JWK（jwks.go:147-156） |
| 启动报 `public key is neither a valid RSA nor ECDSA PEM` | inline/file 的 PEM 非法 | 检查 PEM block 格式 |
| 所有请求 401 `invalid token` | issuer/audience/algorithm 不匹配或 key 不对 | 核对 `iss` vs `issuer`、`aud` vs `audience`、铸 token 的 key |
| token 换版后全量 403 | authority claim 改名或 claim 名不匹配 | 比对 token 的 claim 名与 `scope-claim`/`roles-claim` |
| 期望 401 却匿名放行 | `required=false` 且下游无 guard | 恢复 `required` 或用 `FromContext` 守卫 |
| 新轮换 key 的 token 短暂 401 | JWKS 缓存尚未过期 | 未知 `kid` 已会强制 reload；确认新 key 已发布 |
| 启动报 `algorithm ... not compatible` | pin 与 key source 家族矛盾 | `algorithm` 对齐 source（PEM/JWKS 用 RS/ES/PS，secret 用 HS） |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 13 |
| 其中必填 | 每实例 1 个 key source |
| quickstart 外部依赖 | 0（example 自己铸 token） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（保留 + 新增，供设计裁决）：

- **保留**：与 `starter-oauth2-resource-server` 几乎完全重叠 —— parser、JWKS
  缓存与校验核心相同，那边多一个不同前缀下的 `issuer-uri` discovery；是共享
  validator 模块的候选。
- 新增：`jwks.go` 与 resource-server starter 的同名文件逐字节重复（仅模块前缀
  不同）—— 应抽出公共内部包。
- 新增：JWKS reload 失败时无上限地继续用旧缓存 key（jwks.go:75-79）—— 长时间
  故障会静默让旧 key 持续有效。
- 新增：认证失败无可观测性（401 路径无计数/日志）。
- 新增：`WWW-Authenticate` 对**缺 token** 的场景也写 `error="invalid_token"`
  （jwt.go:214-217）—— 按 RFC 6750，缺 token 场景本不该带 error 参数；瑕疵
  但属规范可见面。
- 新增：`required` 在这里是配置 key，而兄弟 starter 把同一个旋钮放在应用侧
  （`security.Authenticate(v, required)`）—— 姿态旋钮不一致。

# starter-oauth2-client 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均经 starter 源码核对
(`starter.go`、`config.go`、`tokensource.go`、`authcode.go`、`trace.go`)并锚定可运行的
[example/](example/)。**OAuth2 协议语义见
[x/oauth2 官方文档](https://pkg.go.dev/golang.org/x/oauth2)与
[RFC 6749](https://datatracker.ietf.org/doc/html/rfc6749) 的 grant 定义**——以下全部是
go-spring 增量:绑定、bean、tracing、resilience 接线。

**激活条件**:两个互相独立的前缀组。任一 `spring.oauth2.client.instances.<name>.*` key 激活
client-credentials 组(多实例:一个 `<name>` = 一个 `*http.Client` + 一个 `*TokenSource`);
任一 `spring.oauth2.authcode.instances.<name>.*` key 注册一个 `*oauth2.Config`。两者都没有 `enabled` key。

---

## 1. 完整工程示例

一个用 client-credentials bearer token 调用受保护下游 API 的服务,带 retry/breaker
韧性与全程 OTel tracing。example 仓库就是这个布局:

```
demo/
├── go.mod
├── example.go            (或 main.go + service.go)
└── conf/
    └── app.properties
```

**go.mod**(关键依赖):

```
require (
    golang.org/x/oauth2          v0.36.0
    go-spring.org/spring         v1.3.x
    go-spring.org/starter-oauth2-client latest
    go-spring.org/starter-governance    latest  // 可选:真实韧性策略
    go-spring.org/starter-otel          latest  // 可选:真实 trace 导出
)
```

**example.go**——应用的全部面(与 example/example.go 同构):

```go
package main

import (
    "io"
    "net/http"

    "go-spring.org/spring/gs"
    "golang.org/x/oauth2"

    _ "go-spring.org/starter-governance"
    StarterOAuth2Client "go-spring.org/starter-oauth2-client"
)

// Service 消费 OAuth2 背书的 HTTP client。两个 bean 都由 starter 按组名
// "downstream" 注册(见 conf/app.properties),按名注入。
type Service struct {
    // 开箱即用的 client:取 token、刷新 token、给每个请求附 bearer 头,
    // 经治理 executor 做 retry/breaker。
    Client *http.Client `autowire:"downstream"`
    // 面向非 HTTP 调用点(gRPC metadata、WebSocket)的裸 token 源。
    // 注意:没有 resilience 包裹——见 §2.3。
    TokenSrc *StarterOAuth2Client.TokenSource `autowire:"downstream"`
    // 交互式登录流的 authorization-code 配置(如有)。
    OAuth *oauth2.Config `autowire:"login"`
}

func main() {
    svr := gs.Provide(&Service{}).Export(gs.As[gs.Rooter]()) // 根可达
    _ = svr
    // ... 在 handler 里 s.Client.Get("https://api.example.com/resource")
    gs.Run()
}
```

**conf/app.properties**——完整注释配置面:

```properties
# --- client-credentials 实例 "downstream"(激活该组)--------------------------
spring.oauth2.client.instances.downstream.client-id=demo-client
spring.oauth2.client.instances.downstream.client-secret=demo-secret
spring.oauth2.client.instances.downstream.token-url=https://auth.example.com/oauth/token
spring.oauth2.client.instances.downstream.scopes=read,write
spring.oauth2.client.instances.downstream.auth-style=header
spring.oauth2.client.instances.downstream.timeout=5s
spring.oauth2.client.instances.downstream.endpoint-params.audience=https://api.example.com

# --- authorization_code 实例 "login"(激活 authcode 组)----------------------
spring.oauth2.authcode.instances.login.client-id=web-client
spring.oauth2.authcode.instances.login.client-secret=web-secret
spring.oauth2.authcode.instances.login.auth-url=https://auth.example.com/oauth/authorize
spring.oauth2.authcode.instances.login.token-url=https://auth.example.com/oauth/token
spring.oauth2.authcode.instances.login.redirect-url=https://app.example.com/callback
spring.oauth2.authcode.instances.login.scopes=openid,profile

# --- governance(*http.Client transport 的韧性)-------------------------------
# 与 client 同一资源标签:oauth2:<client-id>。
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.max-retries=3
govern.default.error-threshold=10
govern.default.attempt-timeout=2s

# --- observability(starter-otel,可选)--------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
```

**验证**(与 `example/check.sh` 同构——example 自带假 token 端点 :9401 和受保护资源
:9402,不需要外部 IdP):

```bash
bash example/check.sh
# 预期输出包含:
#   Response from protected resource: hello from protected resource
#   Token from TokenSource: demo-access-token
#   Resilience: recovered after 3 attempts: recovered after retries
```

手动模式保持进程存活,便于自己 curl 实验:

```bash
cd example && go run . -manual
curl -s http://127.0.0.1:9401/oauth/token -d 'grant_type=client_credentials' \
  -d client_id=demo-client -d client_secret=demo-secret   # 假 token 端点
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-oauth2-client
  ├─ gs.Module(OnProperty("spring.oauth2.client"))
  │     └─ 每个 <name>:Provide(newClient).Name(<name>).Destroy(destroyClient)
  │                    Provide(newTokenSource) 走 gs.Group
  └─ gs.Group("${spring.oauth2.authcode}", newAuthCodeConfig)
        │
gs.Run()
  ├─ 配置绑定(逐实例):value tag + expr 校验
  │     client 组:client-id/client-secret/token-url 必须非空
  │     authcode 组:没有 expr 校验——空值静默绑定 ⚠
  ├─ bean 装配:*http.Client、*TokenSource、*oauth2.Config 以 (type, name) 注册,
  │     结构体的 `autowire:"<name>"` 按名取用
  ├─ 构造:newClient 包 transport(otel → resilience);此时不拨号——
  │     首次取 token 是惰性的,发生在第一个请求
  ├─ Run/服务:业务代码运行;token 按需取/刷新
  └─ 停机:Destroy(destroyClient) 关闭 resilience roundtripper
        (裸 client 过不了 io.Closer 断言,什么都不释放)
```

`OnProperty("spring.oauth2.client")` 是前缀检查:任一 `spring.oauth2.client.instances.<name>.<k>`
key 触发该 module,前缀下每个子 map 条目成为一个实例(`conf.BindEach`)。这就是为什么
没有 `enabled` key。

### 2.2 哪里接了什么——两个 bean,精确对照

一个 `spring.oauth2.client.instances.<name>` 条目会构建**两个同名 bean**(bean 身份 = type + name):

| Bean | token 机制 | Tracing | Resilience | Destroy |
|------|-----------|---------|------------|---------|
| `*http.Client` | `clientcredentials.Config.Client(...)`——惰性取 token、自动刷新、每个请求带 bearer 头 | 有:transport base 是 `otelhttp`,每次取 token 和每个下游请求各一个 span | 有:transport 包了 `resilience.NewRoundTripper`,资源标签 `oauth2:<client-id>` | 有(关 transport) |
| `*TokenSource` | `clientcredentials.Config.TokenSource(...)`——同样的惰性取/刷新,返回裸 token | 部分:传给 source 的 context 带 otel client(token 端点请求有 trace) | **没有**——不包 executor,无 retry/breaker/limiter | 无(无可关闭资源) |

设计理由(源自源码注释):client 包 transport 是让 bearer token 在 resilience 层
*之前*附好,"每次受保护的重试都是完整请求"(重试重新执行一个带完整授权的请求)。
TokenSource 服务于自己注入 token 的调用点(gRPC metadata、WebSocket 握手),同时是
可观测句柄(`Peek`/`Valid`/`Expiry` 免往返读缓存 token)。这一不对称是已知设计嫌疑(§6)。

`*oauth2.Config`(authcode 组)是纯配置——无 transport、无 tracing、无 resilience。
你自己调 `AuthCodeURL(state)` 和 `Exchange(ctx, code)`;除非 context 里带
`oauth2.HTTPClient`,否则它们用 `http.DefaultClient`。

### 2.3 一次请求,逐层走读

governance 开启时的 `s.Client.Get("https://api.example.com/resource")`:

1. `client.Timeout` 约束整个请求(`timeout` key,>0 时)。
2. `oauth2.Transport`(来自 `clientcredentials`)检查缓存 token;缺失则向 `token-url`
   POST `grant_type=client_credentials`——凭据按 `auth-style` 放置(auto 探测 / Basic
   header / body 参数),`endpoint-params` 并入 body。
3. 该 token POST 本身走 otel 包裹的 base client——恰好一个 span;但它**不受**治理
   (executor 包的是外层 transport,不含 token 交换——见 §4.4 演练)。
4. access token 入缓存;出站请求在 base transport 运行*之前*带上
   `Authorization: Bearer <token>` 头。
5. `resilience.NewRoundTripper` 经资源标签 `oauth2:<client-id>` 解析出的 executor 执行
   请求——retry/熔断/限流策略来自治理中心;governance 关闭时是透明空操作。解析出的
   executor 已自带 observe 层,trip/reject/retry 会发 span + 计数 + 直方图 +
   访问日志。
6. `otelhttp` 发 client span(method/url);无 starter-otel 时为空操作。
7. 响应解栈;401 时 oauth2 层不重试(client_credentials 无 refresh token)——下一次调用
   重新取 token。

---

## 3. 逐 key 行为参考

### 3.1 `spring.oauth2.client.instances.<name>.*`——7 个 key

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `client-id` | string | — | 发往 token 端点的客户端标识(按 `auth-style` 放置)。 | 空 → **启动失败**(expr `$ != ''`)。 |
| `client-secret` | string | — | 客户端凭据;按 `auth-style` 进 Basic 头或 body。 | 空 → 启动失败(expr)。 |
| `token-url` | string | — | client-credentials grant 的 token 端点。 | 空 → 启动失败(expr);URL 错 → 首个请求报 token 端点错误。 |
| `scopes` | []string | — | 随 token 申请的逗号列表。 | 被 IdP 拒绝的 scope → 首次请求时报错,不在启动期。 |
| `endpoint-params.<k>` | map[string]string | — | token 请求的额外 body 参数(Auth0 `audience`、Azure `resource`);一个子 key 一个参数。 | 参数错 → 请求期 token 端点报错。 |
| `auth-style` | string | `auto` | `auto` \| `header` \| `params`——凭据的发送方式;`auto` 由 x/oauth2 探测一次并缓存。 | IdP 要求 Basic 而配了 `params` → 每次取 token 401。 |
| `timeout` | duration | 0 | 应用两处:otel base client(约束取 token)*和*返回的 `*http.Client`(约束每个下游请求)。0 = 无超时。⚠ 值大 + governance `attempt-timeout`:先到的是 per-attempt 上限。 | 0 → token 端点或下游挂死则永久挂起。 |

### 3.2 `spring.oauth2.authcode.instances.<name>.*`——6 个 key

| Key | 类型 | 默认 | 行为/联动 | 配错的后果 |
|-----|------|------|----------|-----------|
| `client-id` | string | — | 嵌进 `AuthCodeURL` 输出的客户端标识。 | 无 expr 校验——空值静默绑定;跳转 /authorize 时被 IdP 以 `invalid_client` 类错误拒绝。 |
| `client-secret` | string | — | `Exchange` 用它认证 token 请求。 | 同样静默绑定;换 token 401。 |
| `auth-url` | string | — | 构建的 `*oauth2.Config` 中的授权端点。 | 空 → `AuthCodeURL` 产出畸形 URL;只在运行期失败。 |
| `token-url` | string | — | `Exchange` 用的 token 端点。 | 空 → `Exchange` 运行期失败。 |
| `redirect-url` | string | — | 回调 URL;必须与 IdP 侧登记的完全一致。 | 不一致 → /authorize 处被拒(精确匹配)。 |
| `scopes` | []string | — | 授权期间申请的逗号列表。 | 被拒的 scope → 授权错误重定向。 |

两组只绑定这些 key;没有 `enabled`、没有 `name` key、没有 TLS 块——要自带 transport
就注入 `*TokenSource` 而不是 `*http.Client`。

---

## 4. 验证与故障演练

### 4.1 正常路径(冒烟)

```bash
bash example/check.sh    # 断言:受保护资源 200、token 值、缓存 Peek/Valid、
                         # AuthCodeURL 形状、重试次数
```

### 4.2 取 token 与刷新的观测

成功调用后 TokenSource 缓存 token:

```go
tok, _ := s.TokenSrc.Token()   // 取或刷新,成功则缓存
s.TokenSrc.Peek()              // 零往返;首次 Token() 前为 nil
s.TokenSrc.Valid()             // 过期即 false
s.TokenSrc.Expiry()            // 首次取之前为零时刻
```

演练:把 example 假端点的 `expires_in` 改成 `1`,隔 2 秒发两次调用——第二次触发刷新
(otel span 或访问日志里多一次对 `token-url` 的 POST)。

### 4.3 错误凭据 / 错误 token URL

把 `token-url` 指向真实 IdP 并配错 `client-secret`:每个下游请求立刻以 x/oauth2 的
取 token 错误失败。失败是逐请求的(惰性获取),从不在启动期——starter 刻意快速绑定、
惰性取 token。

### 4.4 带 resilience 的 client vs 裸 client(不对称)

example 的 `/api/flaky`(先 503 两次再 200)经 `*http.Client` 透明恢复(check.sh 断言
恰好 3 次尝试)。反向演练:

```go
// 裸 token source 没有重试:取到 token 后用普通 client 手动带
// tok.AccessToken 调 flaky 端点——前两次拿到 503。
tok, _ := s.TokenSrc.Token()
req, _ := http.NewRequest("GET", "http://127.0.0.1:9402/api/flaky", nil)
req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
// http.DefaultClient.Do(req) → 前两次 503,无重试。
```

同样可观测:starter-otel + governance 下,被包 client 的熔断动作发出 `oauth2` 标签的
span/指标(`resilience.ExecutorFor("oauth2", resource)`);`TokenSource` 没有对等物。

### 4.5 governance 热切换

配置 `govern.source.file.path` 后,example 运行中把文件里 `max-retries` 改成 `0`——
同一个 flaky 调用立刻快速失败给调用方(executor 策略在调用期解析)。无需重启。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动失败 "expr … != ''" | client 条目的 `client-id`/`client-secret`/`token-url` 为空 | 补齐或删掉该条目。 |
| starter 完全不生效 | 没有任何 `spring.oauth2.client.instances.*` key(前缀激活) | 至少加一个实例块。 |
| 注入的 client 为 nil / "no bean" | autowire 名不匹配(bean 名 = 实例 `<name>`) | 让 `autowire:"<name>"` 与配置 key 一致。 |
| 下游 401 但 token 正常 | 下游要别的 scheme 或 audience | 检查 `endpoint-params`(audience)与 `scopes`。 |
| 取 token 401 | `auth-style` 与 IdP 不符 | 试 `header`(Basic)——最常见要求。 |
| IdP 明明是短 TTL 但 token 永不刷新 | IdP 不返回 `expires_in` → x/oauth2 视 token 为永不过期、永久缓存 | IdP 侧修(始终下发 `expires_in`)。 |
| governance 开了却不重试 | 用的是 `*TokenSource` + 自己的 client(裸路径,无 executor) | 用注入的 `*http.Client`,或自己包 transport。 |
| 无 span | 未引入 starter-otel | 加上;otelhttp 否则是静默空操作。 |
| 首个请求永久挂起 | `timeout=0` 且 token 端点无响应 | 设置 `timeout`。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key | 7(client)+ 6(authcode) |
| 必填 | 3(client 组,expr 校验) |
| quickstart 前置外部依赖 | 0(example 内置进程内 token 端点) |
| "注意/坑"条数 | 4 |

设计嫌疑(待设计裁决;保留旧 USAGE/DESIGN 已有条目,新增写作中发现的):

- `*TokenSource` 不带 resilience 包裹而 `*http.Client` 带——同一配置条目产出治理行为
  不同的两个 bean(既有)。
- token 端点交换本身不受治理:executor 只包外层 transport,IdP 抖动时调用直接失败、
  无重试(新——重试鉴权也许是对的,但意图未文档化)。
- authcode 组没有 expr 校验而 client 组有——空值静默绑定(新)。
- `timeout` 一个 key 应用在两层(取 token + 下游请求);取 token 还会吃掉下游预算(新)。
- `*oauth2.Config` bean 不带 tracing/resilience——除非调用方传入带 `oauth2.HTTPClient`
  的 context,否则 `Exchange` 用 DefaultClient(新)。

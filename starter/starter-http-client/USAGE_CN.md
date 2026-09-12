# starter-http-client 使用说明 — 参考手册

详细使用文档,概览见 [README_CN.md](README_CN.md)。所有行为声明均已对照 starter 源码
(`starter.go`、`config.go`、`driver.go`)、装配器 `starter-http-client/httpx/httpx.go`、
发送 seam `stdlib/httpclt/httpclt.go` 及可运行的 [example/](example/)(自断言冒烟)、
[example-load/](example-load/)、[example-otel/](example-otel/) 核实。**HTTP 语义归
[net/http](https://pkg.go.dev/net/http),trace 语义归
[W3C Trace Context / OTel](https://opentelemetry.io/docs/specs/otel/trace/)**——本文只写
go-spring 增量:分派、服务发现/负载均衡、韧性/治理、可观测。

**激活条件**:只有存在至少一个 `spring.http-client.instances.<name>.*` 配置项时才安装进程级 transport
(`gs.OnProperty("spring.http-client.instances")`,starter.go:60)。每个配置项成为一个 route bean——
以条目名命名,持有自己的 target(addr 或 service-name)与装配好的 transport——由路由表
收集。韧性/故障策略不在这里配置——进程级 `govern.*`(见 starter-governance)。

---

## 1. 完整工程示例

一个客户端以两种方式调用后端——直连地址 + 服务发现/负载均衡——并带熔断保护与
trace 传播,与冒烟验证过的 [example/](example/) 同构。文件树:

```
demo/
├── go.mod
├── main.go
├── backend.go
└── conf/
    └── app.properties
```

**go.mod**(关键依赖):

```
module demo

require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-governance latest
    go-spring.org/starter-http-client latest
    go.opentelemetry.io/otel          v1.45.0
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-http-client"
    // 由 gs-http-gen 从 .idl 生成;只持有一个 Target
    "demo/proto"
)

func main() { gs.Run() }
```

**backend.go** ——进程内后端 + 静态 discovery 注册(真实场景换成 starter-registry-etcd
等注册中心 starter):

```go
package main

import (
    "fmt"
    "net/http"

    "go-spring.org/cloud/discovery"
)

const (
    addrBackendA = "127.0.0.1:9471"
    addrBackendB = "127.0.0.1:9472"
    serviceName  = "greet-svc"
)

func init() {
    // 注册名 "static" 与配置里 discovery=static 对应;被发现模式客户端经
    // discovery.Loader 读取它。
    gs.Provide(func() (discovery.Discovery, error) {
        return discovery.NewStaticDiscovery(
            discovery.Endpoint{Addr: addrBackendA, Healthy: true},
            discovery.Endpoint{Addr: addrBackendB, Healthy: true},), nil
    }).Name("static")
    startBackend(addrBackendA, "backend-A")
    startBackend(addrBackendB, "backend-B")
}

func startBackend(addr, servedBy string) {
    mux := http.NewServeMux()
    mux.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, `{"message":"Hello, %s!","servedBy":%q}`,
            r.URL.Query().Get("name"), servedBy)
    })
    go func() { _ = http.ListenAndServe(addr, mux) }()
}
```

调用侧——生成式客户端——自动骑在已安装的 transport 上,无需注入:

```go
// 生成式客户端(gs-http-gen 产物):Target 即路由 key。
direct := &proto.Client{Target: "127.0.0.1:9471"}   // 直连模式
lb     := &proto.Client{Target: "greet-svc"}          // 发现模式
```

**conf/app.properties** ——上面用到的完整注释配置面:

```properties
# --- 声明式 HTTP 客户端 --------------------------------------------------------
# 每项向覆盖 httpclt.DoRequest 的进程级 transport 贡献一条路由;生成的
# proto.Client 只持有 Target,切换寻址方式纯靠改配置。

# (1) 直连:钉死到单个后端实例,不走发现。
spring.http-client.instances.direct.addr=127.0.0.1:9471

# (2) 服务发现 + 负载均衡:按逻辑服务名路由。
# LB 策略与端点剔除是该 entry 的治理规则(govern.rules[N].balancer / .outlier-threshold),
# 不是这里的 key —— 见下文。
spring.http-client.instances.discovered.service-name=greet-svc
spring.http-client.instances.discovered.discovery=static

# (3) 韧性守卫路由:策略在进程级 govern.* 下。
# 内置 DefaultDriver 连续失败 2 次后熔断,持续 30s。
spring.http-client.instances.guarded.addr=127.0.0.1:9473
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.error-threshold=2
govern.default.open-duration=30s

# 本 demo 不需要入站 HTTP server。
spring.http.server.enabled=false
```

**验证**(在 `demo/` 下):

```bash
go run . &
# 直接探活两个健康后端:
curl -s '127.0.0.1:9471/greet?name=Ada'   # {"message":"Hello, Ada!","servedBy":"backend-A"}
curl -s '127.0.0.1:9472/greet?name=Ada'   # 同上,servedBy backend-B
kill %1
```

仓库自带的自断言冒烟:`cd starter/experimental/starter-http-client/example && ./check.sh`
(额外断言轮询打散、trace 传播与熔断打开)。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-http-client
  └─ gs.Module(gs.OnProperty("spring.http-client.instances"))       [starter.go:60]
        │
gs.Run()
  ├─ 配置绑定:conf.BindEach(${spring.http-client.instances}) → 每个 <name> 一个 Config
  │    validate() 快速失败:addr|service-name 二选一;service-name 必须配
  │    discovery(config.go:79)
  ├─ installDispatch:conf.BindEach 逐条 provide 一个 route bean
  │    (assembleTransport() → route{target, rt});路由表将其收集起来,并替换
  │    httpclt.DoRequest 为一个按调用方 Metadata.Target 取 route 的 hook。
  │    日志:"http client initialized, routes=N"
  │    (Rooter bean → 即使无人注入也会实例化)
  ├─ 运行期:请求经已安装的 transport 分派
  └─ 停机:每条路由的 Close() 释放各自的 discovery watch 与 executor
```

无论配置多少条目,进程内只有**一个** DoRequest 替换:配置多维、路由表单一,每个条目是
一条按 `addr` 或 `service-name` 索引的路由(routeKey,starter.go)。

### 2.2 transport 链 ——精确嵌套顺序与理由

每条路由从外到内(assembleTransport,starter.go:129;httpx.NewTransport,httpx.go:132):

```
自定义 driver 包裹(最外层:嵌 DefaultDriver 后包裹装配产物)
  → trafficTransport        (逐请求注入 X-LoadTest 标记 header)
  → resilience roundTripper (executor:每次调用 observe( fault( rawExec ) ))
  → balancedTransport | fixedHostTransport   (LB 选点 + host 改写 | 钉到 addr)
  → otelhttp.NewTransport(http.DefaultTransport | tls.enabled 时为配置过 TLS 的克隆)
  → 网络
```

设计理由(均引自源码注释,已核实):

- **otelhttp 包在 base 之上(由 httpx 完成)**:trace 传播由 `httpx.NewTransport` 自己
  叠加——可观测(trace、fault 注入、observe 包装)实现在 `starter-http-client/httpx`,starter 只做
  配置绑定。每次尝试——含重试——都被 trace 并注入 `traceparent`。
- **discovery/LB 在 resilience 之下**:"a retry re-picks a fresh endpoint and the breaker
  keys on the logical service name"(httpx.go:32-34)。熔断按逻辑调用计数,重试在实例级
  失败后会换实例。
- **resilience 包住 balanced transport**:`resilience.NewRoundTripper(base, exec, nil)`
  的默认 resource func 是 `r.URL.Host`——在该层深度仍是逻辑 target,因为 host 改写
  发生在更下一层(httpx.go:180-192,resilience/roundtripper.go:70)。
- **traffic 在 resilience 之上**:每次重试尝试都携带标记(header 设在原始请求上,重试
  循环复用它);在用户 middleware 之下以便 wrapper 仍可覆盖(httpx.go:207-213)。
- **executor 栈序 `observe(fault(rawExec))`**(httpx.go 默认包装):fault 注入在真实
  executor 重试循环**之内**——"a fault flows through retry/breaker/timeout/Fallback
  exactly as a real downstream failure would"(fault/executor.go:28-34)——observe 在外层
  记录最终结果。
- **熔断按逻辑调用计数而非按尝试**(历史 bug 已修):executor 在重试循环结束后对整个
  `Execute` 只记录一次熔断结果,"rather than once per attempt … which would trip the
  circuit far faster than the configured ErrorThreshold … implies (the 'resilience on =>
  breaker trips instantly' symptom)"(resilience/executor.go:210-218)。限流器仍按尝试
  计费——每次尝试都是真实的下游请求。

### 2.3 一次请求逐层走读

`proto.Client{Target: "greet-svc"}.Greet(ctx, req)`:

1. 生成式客户端构造 `httpclt.Metadata{Target: "greet-svc", ...}` 调
   `httpclt.ObjectResponse` → `doRequest` 设 `req.Host = req.URL.Host = Target`、
   `req.URL.Scheme = Schema`(httpclt.go:132-139)→ `httpclt.DoRequest`——唯一发送
   扩展点,默认实现是 `http.DefaultClient`(httpclt.go:94)。
2. starter 的替换实现按 `meta.Target` 查路由表,再用该条目的 transport 驱动请求;**该 target
   无路由 → 请求时报错 `http-client: no transport for target <target>`**。查表放在这一层而不是
   RoundTripper 里,是因为 net/http 只把 `*http.Request` 交给 transport——在这一层路由才能让
   声明的 Target 直接当键,同时也让重定向的每一跳都留在调用起始的那条路由上。
3. trafficTransport 当且仅当 `traffic.IsLoadTest(ctx)` 时注入压测标记 header。
4. executor 把整个 round-trip 作为一次受保护调用执行:限流 → 熔断闸门 → 每次尝试
   超时(`attempt-timeout`)→ 带退避的重试循环,整体受 `MaxDuration` 约束;熔断对
   整个调用只记录**一次**结果(§2.2)。
5. balancedTransport `pool.Pick()` 选一个存活实例(round_robin 等),克隆请求并把
   `URL.Host`/`Host` 改写为实例地址,经 `res.Done` 回报结果——least-conn 统计与
   outlier suspension 因此能看到每次调用(httpx.go:245-262)。
6. otelhttp transport 打开 client span、注入 `traceparent`、建连。
7. 响应回卷:span 结束(executor 包过则带 `resilience.outcome`)、计数
   `resilience.calls{outcome=...}`、按 `observability.level` 出访问日志。
8. 直连模式下第 5 步换成 `fixedHostTransport` 把每个请求钉到 `addr`——链上同一位置,
   两种寻址模式下 resilience 与调用侧行为完全一致(httpx.go:264-277)。

### 2.4 扩展点:driver bean(driver.go)

传输装配由 `Driver`(接口,driver.go)负责。每条配置项都经其自身选择的 Driver 装配(在
`assembleTransport` 内解析)。Driver 是**可选容器 bean**,其构造函数返回
`StarterHTTPClient.Driver`;未提供时 starter 回退到内置的 `DefaultDriver`(标准
starter-http-client/httpx 装配)。选择与其它 client starter 一样分两级解析:
`spring.http-client.instances.<name>.driver = <bean 名>` 指定该实例的 Driver bean,缺省回退到
家族级 `spring.http-client.default.driver = <bean 名>`,再缺省则按类型注入唯一 Driver bean。
指定的 bean 不存在则启动失败;因此不同实例可以走不同 driver:

```go
func init() {
    gs.Provide(func() StarterHTTPClient.Driver {
        return authDriver{}   // 嵌 DefaultDriver + 包 RoundTripper 以"增强",或自建 httpx.NewTransport 以"整体替换"
    })
}
```

- **增强默认**:override 嵌 `DefaultDriver`(其内委托 `httpx.NewTransport`),包裹返回的
  `RoundTripper`——加 auth header、自定义 metric、请求过滤。
- **整体替换**:完全自建 transport——如带自定义 `Base` 的 `httpx.NewTransport`(代理、
  连接池调优)。返回的 teardown 归你管。

---

## 3. 逐 key 行为参考

`spring.http-client.instances.<name>.*` 下的 key(已用 `grep -rhoE 'value:"[^"]+"'` 双向核对,
两边均无多余项):

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `addr` | string | "" | 直连模式:`fixedHostTransport` 把每个请求钉到该 host:port。可与 `service-name` 同配,此时 service-name 只是纯治理 label(不触发发现)。 | 都不配 → 快速失败 "one of addr or service-name is required"。 |
| `service-name` | string | "" | 发现模式:经指定后端解析的逻辑名;只要设置了就同时是治理 resource label(发现与直连模式皆是)。配了 `addr` 时它不是发现目标。 | 都不配 → 快速失败;只配 service-name 不配 `addr`/`discovery` → 快速失败。 |
| `discovery` | string | "" | 发现后端 bean 名(bean 名=标签，由 registry starter 注册)。`service-name` 未配 `addr` 时必填(config.go validate)。 | 缺失 → 快速失败。名字未知 → 装配期报错并列出已注册 bean(starter.go newRoute)。 |
| `observability.level` | string | brief | executor 包裹层的访问日志开关:off / brief / detailed(observe/config.go:50)。 | brief 默认**开**——预期每次受保护调用一条日志。 |
| `observability.maxArgBytes` | int | 512 | 日志中参数截断长度。 | 大 body 被静默截断。 |
| `observability.skipOps` | []string | "" | 不记访问日志的操作名。 | |
| `tls.enabled` | bool | false | 打开该 entry 的 TLS 配置面。 | 不开则 https URL 也走系统默认(无自定义证书)。 |
| `tls.cert-file` / `tls.key-file` | string | "" | 呈交给对端的客户端证书对(mTLS)。只配其一 → 装配期加载错误。 | |
| `tls.ca-file` | string | "" | 校验对端的 PEM CA bundle;空 = 宿主机根证书集。 | bundle 坏/空 → 装配期报错。 |
| `tls.server-name` | string | "" | 校验对端证书时检查的名字——按 IP 拨号或经发现 label 时有用。 | 名字错 → 每请求 TLS 校验失败。 |
| `tls.insecure-skip-verify` | bool | false | 跳过对端校验(仅本地测试)。 | 生产打开 = 未认证 TLS。 |

**label 稳定 —— `service-name` 优先**:治理 resource label 是
`resilience.ResourceLabel("http", ServiceName, Addr)` → 只要设置了 `service-name`(发现模式,
或直连模式下作为纯 label 保留)就是 `http:<service-name>`,只有完全没有 service-name 的
entry 才回退 `http:<addr>`(httpx.go `Config.resource`)。因此按 `http:<service-name>`
配的 `govern.rules[].resources` 在直连/发现两种寻址间切换时始终匹配——切换时保留
service-name 即可;只有彻底删掉 service-name 才会换 key。
另:直连模式下熔断按 host:port 计——同一后端两种 `addr` 写法得到两个熔断器。

**不在这里**(`spring.http-client.*` 下没有):超时、重试、熔断、限流、故障注入。所有
策略是进程级 `govern.*`(starter-governance)——`govern.enabled`、`govern.driver`、
`govern.default.<策略字段>`(rate-limit / burst / error-threshold / open-duration /
breaker-strategy / error-rate-threshold / min-requests / max-concurrent / max-retries /
initial-interval / multiplier / max-interval / attempt-timeout / max-duration),故障注入在
`govern.fault.*`。按 target 定策略 = 一条 `govern.rules[n]`,resource 指向上面的 label。
该配置面详见 starter-governance 的 USAGE。

`min-requests`:本 starter 对解析到其 `http:*` 资源的 error-rate 策略强制 **5** 的下限
(httpx.go `minRequestsFloor`)——resilience 自身的零值下限是 1,低流量下单次失败即熔断,
过于敏感。govern rule 显式配更高的 `min-requests` 时以显式值为准;consecutive 策略不受影响。
⚠ 重试会重发非幂等 POST(roundtripper 每次尝试回卷 body,
resilience/roundtripper.go:83-94)——相应约束 `max-retries`。

---

## 4. 验证与故障演练

均基于仓库 example(`cd starter/experimental/starter-http-client/example`)。

### 4.1 冒烟(四项结果自断言)

```bash
./check.sh
# 预期:direct OK / discovery+LB OK: round-robin hit map[backend-A:2 backend-B:2] /
#       trace propagation OK / resilience OK: breaker open on attempt N /
#       all declarative HTTP client checks passed
```

### 4.2 发现打散演练(请求落到所有实例)

```bash
go run . -manual &     # 两个健康后端在 :9471/:9472,static discovery
# 进程内 runTest 已打印:discovery+LB OK: round-robin hit map[backend-A:2 backend-B:2]
curl -s '127.0.0.1:9471/greet?name=x'; curl -s '127.0.0.1:9472/greet?name=x'  # 确认双活
kill %1
```

把 conf/govern.properties 里规则的 `balancer` 从 `round_robin` 换成 `least_conn`,或加第三个
endpoint,看打散变化——池是原地换策略的,不用重启;用真实注册
中心注销实例后,后端快照会丢掉它,绑 loader 的 `Pool` 不再选它(httpx.go:200-221)。

### 4.3 熔断演练 ——打开、快速失败、恢复

`guarded` 路由指向 :9473 上恒返 500 的后端,配置 `govern.default.error-threshold=2`、
`open-duration=30s`:

1. 跑 example:`./check.sh`。守卫客户端最初几次调用打网络失败;连续 2 次失败后熔断
   打开,输出 `resilience OK: breaker open on attempt N, fast-failed in <µs>`——错误为
   `resilience.ErrCircuitOpen`,微秒级返回(无网络往返)。
2. 用自己的 binary 做等价手工演练:杀掉后端、循环调用,观察错误从网络/500 错误变为
   `circuit open`;恢复后端后等 `open-duration`(30s)——half-open 试探成功、熔断闭合。
3. 观察跳变:`resilience.breaker.state_change{from=...,to=...}` 计数器递增并输出状态
   迁移日志(cloud/governance/resilience/observe.go:98-110)。

### 4.4 故障注入(免重启)

fault 挂在同一治理 source 上;`govern.fault.*` 热加载(见 example-load 的 conf 注释)。
在配置文件里翻转:

```properties
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=timeout    # 或 generic / reset
```

注入的故障流经 resilience executor **之内**(§2.2),因此重试会触发、熔断会计数、
`resilience.calls{outcome=timeout|error}` 会记录——演练验证的是整个栈,不只是注入器。

### 4.5 观测项

- 指标(starter-otel 的 prometheus exporter,如 example-otel 的 `:9090/metrics`):
  `resilience.calls` 计数器,属性 `resilience.system="http"`、
  `resilience.resource="http:greet-svc"`、`resilience.outcome` ∈
  {success, rate_limited, circuit_open, bulkhead_full, timeout, error};
  `resilience.breaker.state_change` 带 from/to。
- 访问日志:由 `observability.level` 门控(默认 brief = 开),tag
  `_app_http_resilience`(`log.RegisterAppTag("http", "resilience")`)。
- trace:httpx 内置的 otelhttp 层每请求一个 client span(命名遵循
  [otelhttp 语义](https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp)),
  外加一个带 `resilience.outcome` 的内部 executor span。未装 OTel provider 时均为静默
  no-op。example-otel 自带 docker-compose Jaeger(:4317)可直接查看。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 请求报 `http-client: no transport for target "x"` | 没有 addr/service-name 等于客户端 `Target` 的配置项(错配在请求期而非装配期暴露——starter.go:188-195) | 补一条配置或修正 Target;必须精确匹配。 |
| 容器启动失败:"one of addr or service-name is required" / "discovery is required" | validate() 规则(config.go) | 至少配一种寻址;只配 service-name 时必须配 `discovery`。 |
| 一切正常但没有熔断/限流 | 未设 `govern.enabled`——治理 executor 在其未启用时是透明 no-op | 启用 starter-governance 并配策略。 |
| 单次失败即熔断 | `breaker-strategy=error-rate` 且 `min-requests` 低于 starter 下限 | starter 已把 `min-requests` 下限提到 5;可在 govern rule 调高 `error-rate-threshold`/`min-requests`。 |
| addr ↔ service-name 切换后策略"失效" | ⚠ label 耦合:resource key 变了(§3) | 把 `govern.rules[].resources` 更新为新 label。 |
| 客户端无 trace / 指标 | 未 import starter-otel;otelhttp 与 meter 依赖 OTel globals | 空导入 starter-otel 并配 `spring.observability.*`。 |
| TLS/https 不通 | 该 entry 未设 `tls.enabled` | 打开 entry 的 `tls.*` 配置块(`ca-file`/`server-name`/...);它会在 otel base 下接入 TLS 配置过的 transport。 |
| 加入本 starter 后进程内其他 httpclt 使用方异常 | starter 替换了进程级 `httpclt.DoRequest`——所有 httpclt 调用都经它路由 | 确保进程内每个 httpclt Target 都对应一条配置路由。 |
| 同一后端两个实例拿到两个熔断器 | 直连模式按 `addr` 字面量计 key | 归一化地址,或改用发现模式统一 service-name。 |

---

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 15(路由 6 + observability 3 + tls 6) |
| 其中必填 | tag 层 0;"至少其一" + 条件性 discovery 由 validate 强制 |
| quickstart 前置外部依赖 | 0(发现模式才需要注册中心) |
| "注意/坑"条数 | 8 |

设计嫌疑清单(供裁决台账):

1. ~~README bean 面过时~~ —— 已修 2026-08-27:README(中/英)围绕进程级
   `httpclt.DoRequest` transport 重写;虚构的注入示例已删。
2. ~~虚构的 `resilience.*` key~~ —— 已修 2026-08-27:README/conf 注释改指 `govern.*`。
3. ~~死 key `timeout`~~ —— 已修 2026-08-27:已从 Config 与 README 移除。
4. ~~熔断按尝试计数("开 resilience => 100% 失败/秒开")~~ —— 已修:executor 现在按逻辑
   Execute 只记录一次熔断结果(resilience/executor.go:210-218);限流器有意仍按尝试计费。
5. 静默全局接管 `httpclt.DoRequest`;未知 target 在请求期而非装配期失败。
6. ~~治理 label 随寻址模式静默变化~~ —— 已修 2026-08-28:只要设置 service-name,label 就是
   `http:<service-name>`(允许与 addr 同配作纯 label);完全没有 service-name 才是 `http:<addr>`。
7. ~~error-rate 熔断下 `min-requests` 默认 0——单次失败即跳闸~~ —— 已修 2026-08-28:starter 对
   其资源的 error-rate 策略把 `min-requests` 下限提到 5(显式更高值优先)。
8. ~~孤立的 `ResilienceConfig` 注释~~ —— 已修 2026-08-27。
9. ~~完全没有 TLS key~~ —— 已修 2026-08-28:新增 `tls.*` 配置块(enabled/cert-file/key-file/
   ca-file/server-name/insecure-skip-verify),接入 otel base 之下克隆的 `http.Transport`;
   https 路由从此有了证书配置面。

# starter-gateway 使用说明 — 参考手册

详细使用文档,概览见 [README_CN.md](README_CN.md)。以下行为声明全部对照源码
(`gateway.go`、`server.go`、`compile.go`、`route.go`、`predicate.go`、`filter.go`、
`proxy.go`、`metrics.go`、`otel_tracing.go`)与可运行的 [example/](example/)
(`example/check.sh` 为自断言冒烟脚本)核对。Route/Predicate/Filter 模型参照
[Spring Cloud Gateway](https://docs.spring-cloud-spring-cloud-gateway/reference/) —
本文只写 Go-Spring 的增量。

**激活条件**：仅当配置 `spring.gateway.server.addr` 时才注册 `gatewayServer` bean
(gateway.go:40,`gs.OnProperty("spring.gateway.server.addr")`)——该 key 即总开关。路由表、
metrics、health bean 无条件注册;不配 server key 时路由照样绑定但永远不会对外服务。

---

## 1. 完整工程示例

一个贴近真实的网关:两套路由(path+method 断言、header 注入、前缀剥离)、健康探针、
指标、tracing 与限流。文件树:

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
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-gateway   latest
    go-spring.org/starter-actuator  latest   // 可选:探针 + 管理端口
    go-spring.org/starter-otel      latest   // 可选:真实 trace 导出
)
```

**main.go** —— 网关的全部对外面;路由是纯配置:

```go
package main

import (
	_ "demo/conf" // 通过 gs 配置或 -D 标志;普通 properties 亦可

	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-actuator"
	_ "go-spring.org/starter-gateway"
	_ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

没有必需的应用 bean:空导入 starter-gateway 加配置即是一个能工作的网关([example/](example/)
正是如此)。应用代码只通过 §3.3 的扩展点进入。

**conf/app.properties** —— 上面用到的完整注释配置:

```properties
# --- 网关监听端口(激活 key;端口冲突启动即失败)--------------------------------
spring.gateway.server.addr=:9440

# --- 路由表(可经任意支持刷新的配置源热更新)------------------------------------
# 路由 "orders":/api/orders/** -> 剥掉 /api 前缀、盖 X-From 章、转发。
spring.gateway.routes.orders.predicates.path=/api/orders/**
spring.gateway.routes.orders.predicates.methods=GET,POST
spring.gateway.routes.orders.filters=stripPrefix(1),addRequestHeader(X-From,gw),requestId()
spring.gateway.routes.orders.upstream.target=http://127.0.0.1:19000

# 路由 "users":仅 POST、按客户端 IP 限流 50 rps、要求带 header。
spring.gateway.routes.users.predicates.path=/api/users/**
spring.gateway.routes.users.predicates.methods=POST
spring.gateway.routes.users.predicates.headers=X-Client-Id:demo
spring.gateway.routes.users.filters=rateLimit(rate=50,key=ip)
spring.gateway.routes.users.upstream.target=http://127.0.0.1:19000

# --- resilience:仅作名字注册表;策略值来自 ${govern} --------------------------
spring.gateway.resilience.orders=

# --- 可观测 ---------------------------------------------------------------------
spring.gateway.tracing.enabled=true     # 默认即开;span 需 starter-otel

spring.actuator.addr=:9370
spring.observability.service-name=demo-gateway
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
```

**验证**(任选一个 `:19000` 上的 HTTP 上游,如 `python3 -m http.server 19000`,或
example/example.go 的进程内后端):

```bash
go run .
curl -i :9440/api/orders/42                 # 200;上游收到 /orders/42、X-From: gw
curl -i :9440/api/users/1 -X POST           # 超过 50 rps/客户端 IP 后 429
curl -i :9440/nope                           # 404 Not Found(网关自身返回)
curl -s :9370/gateway/metrics | grep gateway_requests_total
curl -i :9370/healthz                        # 路由表编译成功后 gateway 指示器 UP
```

前置依赖:直连 `http(s)://` 目标无需任何外部系统。`lb://` 路由另需 discovery 后端
(starter-registry-nacos 等);跨实例 `rateLimit` 需 redis 限流器;tracing 需 OTel
collector(见 example-otel/conf)。

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-gateway
  ├─ gs.Provide(newMetrics)                                   // 共享计数器
  ├─ gs.Provide(newRouteTable).Destroy(RouteTable.Destroy)    // 编译后的路由表
  ├─ gs.Provide(newGatewayServer).Name("gatewayServer")
  │      .Export(gs.As[gs.Server]())
  │      .Condition(gs.OnProperty("spring.gateway.server.addr"))
  ├─ gs.Provide(newMetricsEndpoint).Export(endpoint.Endpoint) // GET /gateway/metrics
  └─ gs.Provide(newGatewayHealth).Export(health.Indicator)    // "gateway"
        │
gs.Run()
  ├─ 配置绑定:${spring.gateway} → gatewayConfig(routes 走 gs.Dync、resilience map、
  │            discovery 名、tracing 开关)
  ├─ 字段注入到路由表 bean:Wrappers map[string]FilterWrapper
  │            (autowire "?",compile.go:78)——bean 型 filter 在此解析
  ├─ GatewayServer.Run(server.go:79):
  │    1. tbl.warmup() —— 首次编译;初始配置错误直接启动失败
  │    2. tlsConfig() —— BuildServer();证书文件坏启动失败
  │    3. net.Listen —— 端口冲突启动失败
  │    4. <-sig.TriggerAndWait() —— 应用就绪后才开始对外服务
  ├─ 就绪:health 翻 UP(路由表已编译,metrics.go:150-157)
  └─ SIGTERM:Stop → http.Server.Shutdown 排空在途请求;
              RouteTable.Destroy 停掉所有 discovery watch(proxy.go:139)
```

设计理由(源码注释):编译推迟到 warmup,是因为 `Wrappers` 在构造之后才被字段注入
(compile.go:76-78);server "先 listen、就绪后再 serve",对齐框架的优雅排空编排
(server.go:44-46)。

### 2.2 路由编译管线 —— 每路由精确顺序(compile.go:221-263)

按 **priority 降序、同优先级按 id 升序**逐路由处理(compile.go —— 匹配顺序确定;
`priority` 是可选的每路由 key,不配则保持历史上的 id 排序默认):

1. **Predicate 编译**(`buildPredicates`,predicate.go:29)——每个非空字面量变成一个
   `Predicate`;`headers`/`queries` 键值对畸形或 `after` 非 RFC3339 返回 parse error
   → 整次重编译中止。
2. **Upstream 解析**(`parseUpstream`,compile.go:377)——`lb://svc` 变成 discovery 支撑的
   `Upstream`;其余必须能解析为 `http(s)://host`,否则报错。
3. **Executor 解析**(compile.go:232)——非空 `resilience.policy` 必须指向
   `spring.gateway.resilience` 下的某个 key;未知名字报错。Executor 来自
   `resilience.ExecutorFor("gateway:<name>")`(compile.go:211)——策略值全部归治理中心
   所有并可热更;治理未开时得到透明的 no-op executor。
4. **代理 handler**(`newProxyHandler`,proxy.go:159)——见 §2.3。
5. **Filter DSL 解析**(`buildFilters` → `splitFilters`,compile.go:268-373)——只在括号
   深度 0 处按逗号切分;未知 filter 名、参数错误、或 `jwt-auth`/`lua` 引用的 bean 名不在
   `Wrappers` 里都会报错。
6. **链组装**(compile.go:253-260)——filter 按声明顺序由外向内包住 proxy,再套
   `instrument`(路由 id 写入 context + metrics),启用时最后套 tracing span 包装。

任一阶段出错,已编译的旧表原样保留(keep-last-good,见 §4.3)。

### 2.3 一次代理请求逐层走读

路由 `orders`(filters 为 `stripPrefix(1),addRequestHeader,requestId`)上的
`GET /api/orders/42`:

1. **Match** —— `GatewayServer.ServeHTTP`(server.go:60)按 priority→id 顺序遍历路由;第一个断言
   全部接受的路由胜出(`Route.match`,route.go:78 —— AND 组合)。无匹配 → 网关自身返回
   `404 Not Found`,绝不联系上游。
2. **Tracing 包装** —— 提取入站 trace context,开 server span `gateway orders` 与子
   client span `proxy orders`,并向出站 header 注入 context(otel_tracing.go:38-79)。
   无 starter-otel 时空操作。
3. **instrument** —— in-flight gauge +1,路由 id 写入请求 context(metrics.go:86)。
4. **Filters,由外向内** —— `stripPrefix(1)` 把路径改写为 `/orders/42`;
   `addRequestHeader` 设置 `X-From: gw`;`requestId` 在缺失时生成 `X-Request-Id`。
5. **代理 handler**(proxy.go:184)——目标在这里选一次而不是在 Director 里,因此选点
   失败(`lb://` 后无存活实例)是干净的 `503` 而不是向空 host 拨号。直连 upstream 的
   目标就是固定的路由 URL。
6. **转发** —— `httputil.ReverseProxy`,Transport 是 `http.DefaultTransport` 的克隆再包
   `resilience.NewRoundTripper`(proxy.go:165-166):重试/熔断作用于转发这一跳;resilience
   重试复用同一目标。Director 应用选定的 scheme/host(且当 `preserveHostHeader` 标记过
   context 时保留入站 Host)。
7. **响应** —— status 由 `statusWriter` 捕获;转发的 5xx 会以失败上报负载均衡 done
   回调(outlier 统计,proxy.go:194-198);传输层错误渲染为 `502 Bad Gateway`
   (proxy.go:178-181)。回程记录状态码分类计数并关闭两个 span。

---

## 3. 逐 key 行为参考

### 3.1 顶层 —— `spring.gateway.*`

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `routes` | map `id→RouteRaw` | 空 | **可热更新**(`gs.Dync`,compile.go:52)。每个 `<id>` 是一条路由;id 是 map key,唯一性由结构保证。 | 空 → 网关对一切请求 404(表编译为 0 条路由,仍算 UP)。 |
| `resilience` | map 名→(忽略) | 空 | **仅名字注册表**:key 是路由可引用的 executor 名;value 是空结构体——策略值在 `${govern}` 下、按 `gateway:<name>` 键控(route.go:114-128)。 | `resilience.<name>.*` 下的子 key 被绑定器静默忽略(遗留兼容)。 |
| `discovery` | string | "" | `lb://` 路由未配 `upstream.discovery` 时的默认后端。⚠ 两者都缺 → 编译错误(proxy.go:98)。 | 路由编译失败(启动报错 / reload 保留旧表)。 |
| `tracing.enabled` | bool | true | 每个匹配请求包一层 gateway server+client span。 | 需 starter-otel 才真实导出;否则静默空操作。 |

### 3.2 每路由 —— `spring.gateway.routes.<id>.*`

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `predicates.path` | string | "" | ant 风格:`*` 匹配单段(段内亦可,`v*`→`v1`),`**` 匹配任意段;`/api/**` 匹配前缀及其下所有路径(predicate.go:65-225)。 | 一个断言都不配 → catch-all 路由。 |
| `predicates.methods` | string | "" | 逗号列表,大小写不敏感(`GET,POST`)。 | |
| `predicates.host` | string | "" | 精确 host,或 `*.suffix`(也匹配裸 `suffix`);剥掉端口。 | |
| `predicates.headers` | string | "" | `K:V;K2:V2` —— **全部**键值须精确匹配。畸形对(缺分隔符)→ parse error。 | 编译期报错。 |
| `predicates.queries` | string | "" | `k=v;k2=v2` —— 全部须精确匹配。 | 同上。 |
| `predicates.after` | string | "" | RFC3339;仅该时刻及之后匹配。非 RFC3339 → parse error。 | 同上。 |
| `priority` | int | 0 | 匹配顺序:priority 大的先被检查;并列(含全不配)回落到 id 升序——历史默认。随路由一起热更新。 | 重叠路径解析到 priority 更大(其次 id 更小)的路由;配了 priority 后改名 id 不再改变优先级。 |
| `filters` | string | "" | filter DSL,见 §3.3;按声明顺序由外向内生效。 | 编译期(而非绑定期)报错。 |
| `upstream.target` | string | — | **事实必填**:`lb://<service>` 或 `http(s)://host[:port]`。缺失/畸形 → `parseError`(compile.go:379-392)。 | 路由编译失败。 |
| `upstream.balancer` | string | `round_robin` | 取值 `round_robin`、`least_conn`、`consistent_hash`、`weighted`(cloud/loadbalance)。未知名字 → 编译错误。 | 每 upstream 独立;同一服务的两条路由可用不同策略并共享同一条 discovery watch。 |
| `upstream.discovery` | string | "" | 覆盖顶层 `discovery` 的每路由后端。 | |
| `upstream.suspend-threshold` | int | 0(关) | lb:// upstream 实例连续失败多少次被停牌(outlier suspension,cloud/loadbalance `Tracker`);0 不启用。 | 僵尸实例(活着但持续失败)在冷却期内不再收流量,不再周期性产出 502。 |
| `upstream.suspend-for` | string | `""` | Go duration(如 `30s`);停牌多久后半开试探放回。空/0 用 tracker 的 5s 默认。 | 值非法 → reload 时编译错误,保留旧路由表。 |
| `resilience.policy` | string | "" | 必须指向已存在的 `spring.gateway.resilience.<name>` key。⚠ 耦合:未知名字 → `unknown resilience policy` 编译错误(compile.go:236)。 | 启动失败 / reload 保留旧表。 |

⚠ **优先级耦合**:没有任何路由配 `priority` 时,匹配顺序按路由 id 排序(compile.go)。
改 id 会改变优先级;重叠路径解析到字典序靠前的 id。配 `priority` 可让优先级显式化、
与 id 无关。

### 3.3 Filter DSL 参考(解析器:compile.go:309-373)

一条逗号分隔的字符串;括号内逗号是参数分隔符——切分只发生在括号深度 0,因此
`addRequestHeader(X-From,gw)` 完整保留。`name(a,b)` 的参数会做 trim。参数值不能包含
`,`、`(`、`)`——DSL 刻意**不提供转义语法**;这类值在路由编译期报错,错误信息会指出
替代方案:用 `${...}` 配置占位符在绑定期解析取值,或把值放进经 `RegisterFilter` 注册的
自定义 filter。内置注册(filter.go:73-86):

| Filter | 参数 | 行为 |
|--------|------|------|
| `stripPrefix(n)` | n ≥ 0 | 剥掉前 n 段路径(`/api/orders/42` →n=1→ `/orders/42`);n 超深 → `/`。 |
| `prefixPath(p)` | 前缀 | 前置 `p`(斜杠归一)。 |
| `addRequestHeader(k,v)` / `setRequestHeader(k,v)` / `removeRequestHeader(k)` | 2 / 2 / 1 | 转发前对请求 header Add/Set/Del。 |
| `addResponseHeader(k,v)` / `setResponseHeader(k,v)` / `removeResponseHeader(k)` | 2 / 2 / 1 | 对响应 header 同样操作(proxy 写出前生效)。 |
| `rewriteHost(h)` | host | 覆写出站 Host。 |
| `preserveHostHeader()` | — | 保留入站 Host 而非上游的;设置 Director 认领的标记(proxy.go:174)。 |
| `requestId([header])` | 可选 | 确保存在 `X-Request-Id`(或指定 header);缺失时生成随机 128-bit hex。 |
| `rateLimit(k=v,…)` | 见说明 | `rate`(req/s,**必填** >0)、`burst`、`driver`(默认 `default`;redis driver 提供跨实例预算)、`algorithm`(`token-bucket`/`sliding-window`)、`key`(默认 `route` / `ip`)。超限 → `429 Too Many Requests`;限流后端出错时 **fail open** 并打 Warn 日志(filter.go:269-277)。 |

bean 型 token(从注入的 `Wrappers` map 解析,不走注册表):

| Token | 行为 |
|-------|------|
| `jwt-auth(beanName)` | 调用以 `gateway.FilterWrapper` 导出的 `beanName.Wrap`(如 starter-security-jwt 的 Authenticator)。恰好一个非空参数。未知 bean 名 → 编译错误并提示导出 bean(compile.go:283)。 |
| `lua(beanName)` | 同一接缝,接 starter-lua-filter 的 Filter。 |

扩展点:

- `gateway.RegisterFilter(name, factory)` —— 自包含工厂的全局注册表;空名 / nil / 重名
  panic(filter.go:51-64)。
- `gateway.FilterWrapper` —— 任意实现 `Wrap(next http.Handler) http.Handler` 的 bean 以该
  类型导出,DSL 中按 bean 名引用。

### 3.4 服务端 —— `spring.gateway.server.*`

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `addr` | string | — | **激活 key**(gateway.go:40);端口冲突在 `net.Listen` 失败(server.go:92)。 | 缺失 → 路由绑定但无人服务;无任何告警。 |
| `tls.enabled` | bool | false | 启用 TLS 监听。 | |
| `tls.cert-file` / `tls.key-file` | string | "" | 服务端证书;经 `tlsconf.BuildServer` 构建——文件缺失/不可读启动失败。 | |
| `tls.ca-file` | string | "" | **mTLS 开关**:出现即 `RequireAndVerifyClientCert`(server.go:69-75)。⚠ 与 echo starter 不同,这里 mTLS 是接通的。 | |
| `tls.server-name` / `tls.insecure-skip-verify` | string/bool | — | 共享 tlsconf 结构的客户端侧 key —— 在服务端**绑定了但无效**。 | 静默空操作。 |

---

## 4. 验证与故障演练

以下演练均假定 §1 工程以 `go run .` 运行、`:19000` 有上游(`python3 -m http.server 19000`
或任意 HTTP server)。

### 4.1 路由、断言、filter

```bash
curl -i :9440/api/orders/42            # 200,上游日志显示 GET /orders/42
curl -i :9440/api/orders/42 -X POST    # 仍 200(methods=GET,POST)
curl -i :9440/api/users/1 -X POST      # 带 header 才 200:
curl -i -H 'X-Client-Id: demo' :9440/api/users/1 -X POST
curl -i :9440/nope                     # 404 Not Found —— 无路由匹配
```

### 4.2 热更新路由(免重启)

`spring.gateway.routes` 是 `gs.Dync` 字段(compile.go:52):任何支持刷新的配置源
(starter-config-file 卷 watch、starter-config-nacos……)换掉 map 后,**下一个请求**惰性
重编译路由表(`RouteTable.current`,compile.go:116 —— 快路径只有一次原子读 + 指针比较)。

1. 向在线配置源新增路由:`spring.gateway.routes.ping.predicates.path=/ping/**` +
   `spring.gateway.routes.ping.upstream.target=http://127.0.0.1:19000`。
2. 触发该源的刷新(文件 watcher mtime / nacos 推送)。无重启,成功时也无日志——新表
   直接在下一请求生效:`curl -i :9440/ping/anything` → 上游返回 200。
3. 修改现有路由(如 `stripPrefix(1)` 改 `stripPrefix(2)`)再刷新——上游收到的路径相应变化。

注意:没有刷新源的静态 `app.properties` 永远不会换 map——热更新必须搭配动态配置 starter。

### 4.3 reload 出错保留旧表

注入一条坏路由(parse error),如 `filters=stripPrefix(abc)`、
`upstream.target=ftp://x` 或 `resilience.policy=nonexistent`:

1. 旧表继续服务:`curl -i :9440/api/orders/42` → 依旧 200。
2. 新(坏)路由不存在:`curl -i :9440/ping/x` → 404。
3. 错误高声暴露 —— grep 应用日志:
   `grep 'route reload failed, keeping previous table'`(compile.go:123,Error 级,tag AppDef)。
4. 指标递增:`curl -s :9370/gateway/metrics | grep gateway_route_reload_errors_total` → 1。
5. 坏 map 的指针被采纳(compile.go:125),同一份坏编辑不会每请求重试;修好配置再刷新即
   正常重编译。

对比:启动时的**首次**编译失败会终止整个应用(server.go:82)——只有启动后的 reload 才
keep-last-good。

### 4.4 限流演练

```bash
for i in $(seq 1 100); do curl -s -o /dev/null -w '%{http_code}\n' -X POST :9440/api/users/1 -H 'X-Client-Id: demo'; done | sort | uniq -c
# rate=50 时:200 与 429 混合;换第二个源 IP 预算独立(key=ip)
```

### 4.5 可观测

- 指标(actuator `GET /gateway/metrics`,Prometheus 文本,metrics.go:110-139):
  `gateway_requests_total{route="orders",status="2xx"}`、`gateway_in_flight_requests`、
  `gateway_route_reload_errors_total`。
- 健康:`curl :9370/healthz` —— `gateway` 指示器在路由表已编译时 UP(metrics.go:150-157)。
  `lb://` 路由无存活实例时仍 UP —— 这是每路由的局部问题(503 + Warn 日志),不是整个
  网关的失败。
- trace(配 starter-otel):server span `gateway <routeId>`(带 `gateway.route` 属性)+
  子 client span `proxy <routeId>`;5xx 会把 client span 标为 Error。可在 tracing 后端按
  路由 id 过滤。

### 4.6 上游故障演练

停掉上游 → 每个请求得 `502 Bad Gateway`(proxy.go:180),伴随 Warn 日志
`route %q upstream error`。`lb://` 路由无存活实例 → `503 Service Unavailable`
("no upstream",proxy.go:187-189)。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 网关不监听 | 缺 `spring.gateway.server.addr` | 配上——它是激活 key。只配路由不配它现在会在启动时打 WARN 点名孤儿路由(RouteTable.Init,compile.go)——这些路由不会被服务。 |
| 启动失败 `route %q: gateway: invalid …` | 初始路由表有 parse error(predicate/upstream/filter) | 修字面量;首次编译按设计即致命(server.go:82)。 |
| 日志出现 `route reload failed, keeping previous table` | 热编辑坏了;旧表仍在服务 | 修字面量再刷新;观察 `gateway_route_reload_errors_total`。 |
| 永远 404 | 没有路由断言能匹配(path 拼写错、methods/host/headers 断言拒绝) | 记住按 priority→id 顺序取首个匹配;id 靠后的更具体路由赢不了靠前的重叠路由——配 `priority` 可覆盖。 |
| `unknown resilience policy` | `resilience.policy` 指向的名字不在 `spring.gateway.resilience` 下 | 把名字加为(无值的)key;策略值来自 `${govern}`、资源标签 `gateway:<name>`。 |
| 遗留 `resilience.<name>.max-retries` 等不生效 | 设计如此——value 是空结构体;绑定器忽略子 key(route.go:121-128) | 把策略迁到治理中心,资源标签 `gateway:<name>`。 |
| `no FilterWrapper bean named …` | `jwt-auth(x)`/`lua(x)` 引用的 bean 未以 `gateway.FilterWrapper` 导出 | 用 `.Export(gs.As[gateway.FilterWrapper]())` 导出(bean 注入发生在 warmup 之前)。 |
| `lb:// upstream cannot resolve … mesh mode active` | lb 路由缺 discovery 后端,或 mesh 模式开启 | 配 `upstream.discovery`/`spring.gateway.discovery`;mesh 模式下改路由到服务的稳定地址(proxy.go:115)。 |
| 莫名 429 / 限流不跨实例 | `rateLimit` key 默认按路由 id;driver 默认 `default`(本地) | 按客户端用 `key=ip`;跨实例预算用 `redis` driver。 |
| 上游收到错误 path/Host | stripPrefix 段数不对;Host 默认被改写 | 调 `stripPrefix(n)`;加 `preserveHostHeader()` 或 `rewriteHost(h)`。 |
| `tracing.enabled=true` 却没有 trace | 未导入 starter-otel | 加上;否则 OTel 全局是静默空操作。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数(生效) | 22(顶层 4 + 路由 12 + server 2 + tls 生效 4;tls 死 key 2) |
| 其中必填 | 2(`server.addr`、`upstream.target`) |
| quickstart 前置外部依赖 | 0(discovery/redis/collector 可选) |
| "注意/坑"条数 | 7 |

设计嫌疑清单(待设计裁决):

1. ~~17 个遗留策略 key 能绑定但从不读取~~ —— 已演进:`policyRaw` 现为空结构体
   (route.go:128),遗留子 key 连绑定都不再发生(被静默忽略)。残留风险:迁移旧配置的
   用户得不到任何"策略已迁往 ${govern}"的反馈。
2. ~~filter DSL 是字符串语法——`,`/`()` 无法转义~~——已按设计裁决(2026-08-28):不加
   转义语法;含 `,`/`(`/`)` 的值在路由编译期被拒,错误信息指向 `${...}` 占位符 /
   `RegisterFilter` 自定义 filter 替代方案。
3. ~~只配路由不配 `spring.gateway.server.addr` 时静默无行为~~——已修(2026-08-28):
   `RouteTable.Init` 启动时打 WARN 点名孤儿路由。
4. ~~example-otel 过期 tracing key~~——已修(2026-08-27)。另发现主 example 的冒烟在
   `gs.Run()` 之前执行(server 未起)——也改为后台 gs.Runner,已修。
5. ~~路由优先级隐式由 id 排序决定——没有显式 order key~~——已修(2026-08-28):新增
   可选每路由 `priority` key,大者优先,并列回落 id 排序。
6. ~~`schema.json` 是空壳;example-otel 只有配置没有 main.go~~——已修(2026-08-28):
   schema.json 已覆盖真实配置面;example-otel/main.go 已补齐可运行。
7. `tls.server-name` / `tls.insecure-skip-verify` 是共享 tlsconf 结构的客户端侧 key——
   在服务端绑定但无效(与其他 starter 同款模式)。

# starter-elasticsearch 使用说明 — 参考手册

详细使用参考。概述见 [README_CN.md](README_CN.md)。所有行为声明均对照 starter 源码
（`starter.go`、`config.go`、`client.go`、`command.go`、`driver.go`、`health/health.go`、
`resilience_test.go`）与可运行示例核对（[example/](example/) 自校验冒烟；
[example-otel/](example-otel/)、[example-cloudnative/](example-cloudnative/)、
[example-load/](example-load/)）。Elasticsearch 语义与 go-elasticsearch v8 API 属于
[客户端官方文档](https://www.elastic.co/guide/en/elasticsearch/client/go-api/current/index.html)
——本文只写 go-spring 增量。

**激活条件**：出现任意 `spring.elasticsearch.instances.*` key（模块为
`gs.Module(gs.OnProperty("spring.elasticsearch"))` 前缀匹配）。每个
`spring.elasticsearch.instances.<name>` 条目注册一个名为 `<name>` 的 `*StarterElasticsearch.Client`
bean，外加名为 `elasticsearch:<name>` 的健康指示器。

---

## 1. 完整工程示例

一个服务同时持有直连实例与 discovery 实例，并接入 actuator + otel。文件树
（对照 example/ + example-otel/）：

```
demo/
├── go.mod
├── main.go
├── discovery.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖，版本随 example/go.mod）：

```
require (
    github.com/elastic/go-elasticsearch/v8 v8.19.6
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-elasticsearch latest
    go-spring.org/starter-actuator     latest   // 可选：readiness :9370
    go-spring.org/starter-otel         latest   // 可选：真实 trace/metric 导出
    go-spring.org/starter-governance-file   latest   // 可选：resilience/fault 策略
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    StarterElasticsearch "go-spring.org/starter-elasticsearch"
    _ "demo/service"
)

func main() { gs.Run() }
```

**discovery.go** —— 注册 `service-name` 实例所用的 discovery 后端（真实部署在这里接
Consul/Nacos/k8s；见 example/discovery.go）：

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    gs.Provide(func() (discovery.Discovery, error) {
        return discovery.NewStaticDiscovery(    discovery.Endpoint{Addr: "127.0.0.1:9200", Healthy: true}), nil
    }).Name("default")
}
```

**service.go** —— 注入 wrapper，执行真实的 index/get/search：

```go
package service

import (
    "context"
    "strings"

    "go-spring.org/spring/gs"
    StarterElasticsearch "go-spring.org/starter-elasticsearch"
)

const indexName = "demo-docs"

type Service struct {
    // 恒为 wrapper 类型 *StarterElasticsearch.Client。它内嵌裸
    // *elasticsearch.Client，API 树（Index/Get/Search/Info、...）与
    // 生命周期方法因此全部原样提升。
    Main *StarterElasticsearch.Client `autowire:"main"`
    Disc *StarterElasticsearch.Client `autowire:"disc"` // discovery 解析节点
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) {
            // 就绪探测：对集群发一次 Info 请求。
            if err := StarterElasticsearch.HealthCheck(ctx, s.Main); err != nil {
                panic(err) // 集群不可达——快速失败
            }
            body := `{"title":"hello","views":1}`
            _, _ = s.Main.Index(indexName, strings.NewReader(body),
                s.Main.Index.WithDocumentID("1"), s.Main.Index.WithRefresh("true"))
        }
    }).Export(gs.As[gs.Rooter]())
}
```

**conf/app.properties** —— 上面用到的完整配置面（风格复制自
example/conf/app.properties + example-otel/conf/app.properties）：

```properties
# --- 直连实例 ---------------------------------------------------------------
spring.elasticsearch.instances.main.addresses=http://127.0.0.1:9200

# --- discovery 实例 ---------------------------------------------------------
# 即使 service-name 会覆盖 addresses，校验仍要求其非空——下面的哑地址故意不可解析，
# 用以证明 discovery 生效。
spring.elasticsearch.instances.disc.addresses=http://nonexistent.invalid:9200
spring.elasticsearch.instances.disc.service-name=es-cluster
spring.elasticsearch.instances.disc.discovery-scheme=http

# --- actuator + otel（example-otel 风格）-----------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090
spring.observability.metrics.path=/metrics
```

**验证**（先起 ES——见 example/docker-compose.yml；ES 8.13 单节点、关安全，首启最长
120 秒；看 trace 则按 example-otel/docker-compose.yml 再起 Jaeger）：

```bash
go run .                          # 集群不可达时启动直接失败
curl -s :9370/readyz | jq .       # components 含 elasticsearch:main 与 elasticsearch:disc
curl -s :9090/metrics | grep -E 'db.client.*elasticsearch'   # duration + in-flight 指标
curl "http://127.0.0.1:16686/api/traces?service=demo&limit=1" # Jaeger 中的 span（example-otel 同款检查）
grep _app_elasticsearch_access app.log | tail -3              # 每请求一条访问日志
```

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-elasticsearch
  └─ gs.Module(OnProperty("spring.elasticsearch")) 在出现任意 spring.elasticsearch.instances.* key 时触发
        └─ conf.BindEach("${spring.elasticsearch}") → 每个 <name> 条目一份 Config
              ├─ Provide(newClient, IndexArg(1, ValueArg(c)),
              │            IndexArg(2, ?Driver)).Name(<name>)
              │      .Destroy((*Client).Destroy).Caller(1)
              └─ Provide health.Indicator，名为 "elasticsearch:<name>"
                     .Export(gs.As[health.Indicator]())

gs.Run()
  ├─ 构造 newClient：
  │    ├─ 设置了 service-name 且 !mesh.Enabled() → resolveAddresses(c, backend)：
  │    │      读快照 → "scheme://host:port" 覆盖 c.Addresses
  │    │      （快速失败：无 `${discovery}` 后端 bean、或该服务无端点）
  │    ├─ 可选 Driver bean（无则用内置 DefaultDriver；多个并存时实例可按名指定：
  │    │      `spring.elasticsearch.instances.<name>.driver = <bean 名>`，留空 = 按类型注入
  │    │      唯一 Driver bean，指定的 bean 不存在则启动失败）→ driver.CreateClient(ctx, c, backend, params)：
  │    │      DefaultDriver 安装 dynamicTransport 间接层，并返回 *Client 包装体；
  │    │      NewClient 定身份、派生 service label 并置
  │    │      exec = params.ExecutorFor("elasticsearch", service)
  │    │      （fault(受治理 executor)，零值 params 则降级为只观测的 Unmanaged），
  │    │      然后装入传输层：声明层在最外层，resilience 在其内
  │    └─ fail-fast 探测：HealthCheck(ctx, client)——一次直连裸 client 的 Info；
  │        失败则释放 client（Destroy）并中止启动
  ├─ 就绪：指示器转 UP（经 HealthCheck 执行裸 client.Info）
  └─ SIGTERM → Destroy：exec.Close → client.Close
```

集群不可达、service-name 无端点，都会让启动失败——进程不会带着坏的
ES 连接进入"服务中"状态。

### 2.2 transport 链——精确顺序与理由

每个请求依次穿过以下层。构建后的顺序：

```
elasticsearch API（Index/Get/...）
  → elastictransport 重试循环（MaxRetries / DisableRetry）
  → dynamicTransport（RWMutex 间接层；包装体换入前直通 http.DefaultTransport）
  → declareTransport：把操作（db.system／db.operation + URL 路径）放到请求 context 上——
      在 executor 之外，故 resilience 层在 Execute 入口才读得到它
  → resilience roundTripper：executor = fault(emit(限流/熔断/bulkhead/重试))：
      注入的 fault injector 在最外，executor 自带的发射层（每次 Execute 的 span +
      call 级/attempt 级两个 duration 直方图 + outcome 计数 + 访问日志）在其内，
      治理核心在最内
  → http.DefaultTransport → 网络
```

设计理由（源码注释，[command.go] 与 [client.go]）：

- **starter 只声明，resilience 层发射。** 链上只有一个发射点。声明传输层（[command.go]）
  把 method + URL 路径变成 `observability.Operation` 放到请求 context 上；executor 在
  Execute 入口读取它，发射唯一的 span、两个 duration 直方图、in-flight gauge 与访问日志。
- **声明层在 executor 之外，而非它的 base。** executor 在被保护调用运行**之前**就读走操作，
  所以放在它内部的声明没人读。因此传输栈是「声明层最外 → resilience round-tripper →
  `http.DefaultTransport`」——这与旧分层相反：旧的那个负责发射的传输层是 executor 的
  base，逐 attempt 运行。
- **不启用 elastic transport 自带的 OTel 插桩。**
  `elasticsearch.Config.Instrumentation` 过去装 `NewOtelInstrumentation`，会在 esapi 层
  每请求开一个 call 级 client span——同一调用第二个 span，与 resilience 层现在发射的那个
  重复。已删除，使 resilience 层保持唯一发射点。（与 starter-go-redis 同一处置：那边砍掉
  redisotel 的逐命令 span、只留非逐调用的池指标；这里的插桩本就是逐调用的，故一并不留。）
- **用 dynamicTransport 而非固定 transport**：ES 的 transport 在构造期固定、事后无法
  在 client 上替换；这层间接让 resilience 策略（Dync）保持可热更，尽管 transport 实例
  本身不可换。槽位用 RWMutex 而非 atomic.Value，因为活跃 round-tripper 是若干不同具体
  类型之一——atomic.Value 版本在第二次 Swap 时 panic，resilience_test.go 正是钉住这
  一点的回归测试。
- **自定义 driver 可能完全没有这些**：只有 DefaultDriver 构建的 client 才拿到
  dynamicTransport；自定义 driver 自带 transport 会静默绕过换入——该实例的声明层与
  resilience 层均不可用。

### 2.3 一次请求走读：带 match 查询的 `Search`

1. 生成的 API 构造 `POST /demo-docs/_search` 并交给传输链。
2. 重试循环（至多 `max-retries`，默认 3）把请求交给 dynamicTransport。
3. resilience executor 以 service label 为作用域申请许可，例如
   `elasticsearch:es-cluster` 或 `elasticsearch:http://127.0.0.1:9200`（取首个地址，
   在 `NewClient` 里经 `resilience.ServiceLabel` 派生）——按集群而非按请求。
   治理关闭时 executor 是只观测的 `resilience.Unmanaged`。
4. declareTransport（在 executor 之外）把操作放到请求 context 上：span 名与 `db.statement`
   是 `POST /demo-docs/_search`（其 URL 路径），`db.operation` 是 `POST`（有界的那部分），
   `db.system` 是 `elasticsearch` [command.go]。
5. 完成时 executor 发射唯一的 span、call 级/attempt 级 duration 直方图与访问日志。
   调用方拿到的与裸 go-elasticsearch 完全一致——包括 4xx 的 `res.IsError()` 响应体；
   只有传输层错误进入异常路径（5xx 由 resilience round-tripper 映射为可触发熔断、
   可重试的错误，cloud/resilience/roundtripper.go:99-104）。

**每次调用都要传 context。** 它承载这次调用的取消与截止时间，resilience 层打开的操作 span
也继承它——`HealthCheck` 与健康指示器都显式传 context [starter.go, health.go]；
用户代码应使用 `es.Search.WithContext(ctx)` 等。

### 2.4 discovery 寻址——种子 + 活的节点集

设置 `service-name` 且 mesh 模式关闭时，端点在构造函数里解析一次并固化进 `c.Addresses`；loader 是
纯快照函数，无资源、无后台 watch，所以不保活也无需在停机时 Stop [starter.go, driver.go]。
这次读是**fail-fast 闸门兼种子**——真正持续供数的是 driver 用同一个 resolver
装上的自定义 `ConnectionPoolFunc` [pool.go]：transport 的节点集会从命名服务重读，所以节点加入/离开
**不用重启**即可用。传播预算是 1 秒（`refreshInterval`，在请求路径上检查），注册中心抖动或空快照
则保留上一份可用节点集，而不是把片刻前还正常的流量打黑。

节点选择未被动过：活池内部保留由库构建的池（transport 自己的选择器与 live/dead 记账），只在节点集
**真的**变化时重建（已按地址排序，所以快照换序不算变化）。mesh 模式下由 sidecar 负责发现+LB，静态
Addresses（或 CloudID）原样使用。

---

## 3. 逐 key 行为参考

所有 key 位于 `spring.elasticsearch.instances.<name>.` 下——构造参数 Config 走 `conf.BindEach`
的实例前缀绑定。没有 observability key——观测无条件开启（见 §3.4 插桩）。

### 3.1 寻址与发现

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `addresses` | list | — | 节点 URL，如 `http://127.0.0.1:9200`（逗号分隔）。校验非空（`len($) > 0`）。⚠ 即使被 `service-name` 覆盖也必填——example 故意带一个不可解析的哑地址。⚠ 设置 `cloud-id` 时被忽略（客户端侧优先级）。 | 空 → BindEach 报错；首探不可达 → 启动报 "failed to reach elasticsearch cluster"。 |
| `service-name` | string | — | 经已注册 discovery 后端解析节点地址，并让 transport 的节点集持续跟随它（pool.go）；覆盖 `addresses`。mesh 模式忽略。⚠ 与 `scheme`/`discovery`/`discovery-scheme` 成组。 | 启动期服务无端点 → 启动报 `discovery %q returned no endpoints`；运行期空快照 → 保留上一份可用集合。 |
| `scheme` | string | — | 将 discovery 收敛到单一传输 scheme 的端点。仅在设置 `service-name` 时生效。 | — |
| `discovery` | string | — | 用哪个已注册后端解析 `service-name`。未配置时回退 `${spring.elasticsearch.default.discovery}`。 | service-name 已设但两层都未配置或名字无对应 bean → 启动报错。 |
| `discovery-scheme` | string | `http` | 拼到发现的 `host:port` 端点前的 URL scheme（`http`/`https`）。 | scheme 错 → 启动首探失败。 |
| `cloud-id` | string | — | Elastic Cloud 部署 ID；设置后客户端优先于 `addresses`。 | — |

### 3.2 认证与 TLS

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `username` / `password` | string | — | HTTP Basic 认证。 | 错误 → 启动 Info 探测报错。 |
| `api-key` | string | — | base64 API key；设置后优先于 username/password。 | 与 basic auth 并设 → API key 静默胜出。 |
| `service-token` | string | — | 服务账号 token 认证。 | — |
| `certificate-fingerprint` | string | — | CA 证书 SHA256 hex——无需 CA 文件即可固定自签 HTTPS。⚠ 要求 `https://` 地址（或 CloudID）；本 starter **没有 `tls.*` 块**。 | 指纹不匹配 → 启动 TLS 报错。 |

### 3.3 传输行为

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `max-retries` | int | 3 | elastictransport 重试次数。⚠ 与治理 executor 的重试（`spring.governance.client.default.max-retries`）叠加——两层重试相乘放大次数与时延。 | 大值 + 治理重试 → 尝试次数成倍。 |
| `disable-retry` | bool | false | 彻底关闭客户端重试循环。 | — |
| `compress-request-body` | bool | false | 请求体 gzip 压缩。 | — |
| `enable-metrics` | bool | true | 客户端内建 elastictransport 指标开关（⚠ schema.json 误写默认 false——以代码为准）。 | — |
| `enable-debug-logger` | bool | false | elastictransport 调试日志。 | 开启 → 日志极多，含请求/响应体。 |

### 3.4 插桩

本模块**没有插桩配置 key**（没有 level、没有跳过名单）：starter 无条件**声明**每个请求的
身份（[observe.go]），resilience 层据此**发射**全部信号。它们都搭乘 starter-otel 安装的
OTel 全局（`spring.observability.*`）。

---

## 4. 验证与故障演练

### 4.1 经 actuator 看健康

```bash
curl -s :9370/readyz | jq .           # components 含 "elasticsearch:main"
docker stop starter-elasticsearch     # 指示器执行 client.Info → 组件转 DOWN
curl -s :9370/readyz                  # 503 OUT_OF_SERVICE
docker start starter-elasticsearch
```

### 4.2 可观测到底发什么

- **指标**（OTel，经 starter-otel 全局），由 resilience 层发射：call 级
  `db.client.operation.duration` 与 attempt 级 `db.client.attempt.duration` 两个直方图，
  以及 in-flight `db.client.active_requests` gauge——属性 `db.system=elasticsearch`、
  `db.operation=<METHOD>`（有界）、`status` = ok/error，均不含 URL 路径。同一层还发
  `resilience.client.calls` 计数器，带 `status` ∈ {ok, error}，以及保护拒绝该调用时的
  `resilience.outcome` ∈ {rate_limited, circuit_open, bulkhead_full, retry_budget_exceeded,
  timeout}。
- **span**：每请求一个 span，由 resilience 层打开（internal 类型），形如
  `POST /demo-docs/_search`，带 `db.system`/`db.operation` 属性及 `db.statement` = URL 路径。
  验证：`curl "http://127.0.0.1:16686/api/traces?service=demo&limit=1"`
  并 grep `"data":[{`（与 example-otel 自测同款）。
- **访问日志**：每请求一行，由 resilience 层写入，tag 为 `_app_elasticsearch_access`
  （`log.RegisterAppTag("elasticsearch", "access")`），按原生级别——失败 → Warn；带捕获
  URL 路径（截断至 512 字节）的成功 → Debug；普通成功 → Info。

```bash
curl -s "127.0.0.1:9090/search" >/dev/null  # 或由应用驱动
curl -s :9090/metrics | grep -E 'db.client.operation.duration|active_requests'
grep _app_elasticsearch_access app.log | tail -1
```

### 4.3 resilience / fault 演练（example-load 风格）

```properties
# NOTE: governance RULES go in conf/governance.properties, referenced by spring.governance.source.file.path in app.properties (see starter-governance-file USAGE).
spring.governance.enabled=true
spring.governance.driver=default
spring.governance.client.default.rate-limit=5          # 并发 > 5 → ErrRateLimited 拒绝
spring.governance.client.default.error-threshold=20
spring.governance.client.default.open-duration=5s
spring.governance.client.fault.enabled=false           # 置 true + rate=0.5 + error=timeout 即"放火"
```

运行 [example-load/](example-load/)（`go run . -concurrency=16 -duration=5s`）：打印的
错误分解会显示限流/熔断 outcome；resilience outcome 计数与熔断状态变更日志随之出现，
无需重启。策略可热更（Dync）——改 spring.governance.* 后 executor 自动生效。

### 4.4 discovery 接线

```properties
spring.elasticsearch.instances.disc.addresses=http://nonexistent.invalid:9200  # 哑地址，被覆盖
spring.elasticsearch.instances.disc.service-name=es-cluster
```

启动成功且 `HealthCheck` 通过即证明地址来自 discovery 后端而非配置（example/ 特性 5）。
注意解析是一次性的：之后迁移实例需要重启（§2.4）。

### 4.5 服务端宕机行为

- 启动期：fail-fast——进程以 "failed to reach elasticsearch cluster" 退出。
- 运行期：请求带完整链路（span + 访问日志 + 熔断计数）返回传输错误；`/readyz` 在一个
  探测周期内转 DOWN。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动报 "failed to reach elasticsearch cluster" | 地址不可达 / 凭据错误 / 指纹不匹配 / ES 未起完（最长 120 秒） | 修连通性；等 `curl http://127.0.0.1:9200` 通后重启。 |
| 启动报 `discovery ... returned no endpoints` | service-name 在后端不存在，或后端未注册 | gs.Run 前注册后端 bean（命名 discovery.Discovery bean）；核对服务名。 |
| 请求内 nil context panic | `http.Request.WithContext` 对 nil context 会 panic | 每次调用传 `WithContext(ctx)`；不用无 context 的 API 变体。 |
| 已设 service-name 仍在 `addresses` 校验失败 | `addresses` 无条件必填（`len($) > 0`） | 保留哑地址（example 的做法）——反正会被覆盖。 |
| 代码正确却无 span/指标 | 未导入 starter-otel | resilience 层的信号挂 OTel 全局；导入 starter-otel 并配置 exporter。 |
| 没有访问日志 | 日志 tag 被 logger 配置过滤 | 检查 `_app_elasticsearch_access` 的 logger 配置。 |
| 自定义 driver 实例没有熔断/指标 | 只有 DefaultDriver 安装 dynamicTransport | 用 DefaultDriver，或在自定义 driver 里自行安装 声明+resilience transport。 |
| 重试次数疑似翻倍 | 客户端 `max-retries` 与治理 `max-retries` 同时 > 0 | 一侧归零 / disable-retry。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 16 个实例 key |
| 其中必填 | 1（`addresses`，校验非空） |
| quickstart 前置外部依赖 | 1（Elasticsearch） |
| "注意/坑" 条数 | 5 |

设计嫌疑清单（审计台账；前三条沿用旧版文档）：

- 设置 `service-name`（或 `cloud-id`）后 `addresses` 被静默忽略，但校验仍要求非空 →
  哑地址写法是权宜而非设计（候选：service-name/cloud-id 存在时放宽 expr）。
- 与兄弟 starter 不同没有 `tls.*` 块——TLS 分散在三个 key 加 URL scheme
  （`https://` 地址、`cloud-id`、`certificate-fingerprint`）。
- 自定义 driver 静默丢失 governance + resilience 声明传输层换入——无告警、无 hook。
- schema.json 的 `enable-metrics` 默认值（`false`）与代码（`true`）不一致——schema 非
  生成物，会漂移。
- 健康指示器无 opt-out key（与 go-redis 同款家族不对称；redigo 有 `health.enabled`）。

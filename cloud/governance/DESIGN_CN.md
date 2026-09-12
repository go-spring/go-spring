# 集中式治理中心（govern）设计说明

## 1. 它解决什么问题

在 govern 出现之前，每个 client starter 各自背一份韧性（resilience）配置：

```
redis   : gs.Dync[resilience.Config]  ← ${spring.redigo.instances.0.resilience}     + 自己的 OnChanged
gorm    : gs.Dync[resilience.Config]  ← ${spring.gorm.mysql.instances.resilience}   + 自己的 OnChanged
mongo   : gs.Dync[resilience.Config]  ← ${spring.mongodb.instances.*.resilience}      + 自己的 OnChanged
... × 11
```

这带来三个痛点：

- **配置散落**：改 redis 的超时要编辑一个 key，改 gorm 又是另一个 key，没有全局视图，也没有"一处下发、处处生效"的能力。
- **样板重复**：11 个 starter 各写一份 `gs.Dync` + `OnChanged` + 重建 executor 的几乎相同代码。
- **后端选型各自为政**：每个 starter 有自己的 `${...driver}` 开关，无法一处切换全进程的韧性后端（default / sentinel）。

govern 把这 11 份 Dync 收敛成 **全进程唯一一份治理配置**——经 `governance.Source` 契约进入中心，不再挂 gs 的通用属性管道。

## 2. 整体拓扑

```
              规则来源（file / http / 控制台 / SetSource）
                        │  governance.Source 契约
                        ▼
                governance.center.adopt(cfg) → refresh(cfg) + fault SetConfig
                        │
            ┌───────────┼────────────┬───────────┬───────────┐
            ▼           ▼            ▼           ▼           ▼
     Register(label₁  Register(label₂ Register(  Register(  Register(
        ,cb₁)          ,cb₂)         cb₃)        cb₄)       cb₅)
            │           │            │           │           │
        仅当 label₁  仅当 label₂   ... 各自只在"属于自己的 policy 变了"时回调
        的 policy 变  的 policy 变
        时回调         时回调
```

- **根**：`cloud/governance` 里的 center 单例，持有唯一的活跃 `governance.Source`（规则来源）。来源由 starter-governance 接线注入。
- **叶**：每个 client starter 在自己的 setup 阶段调一次 `governance.Register(label, cb)`，把自己登记为某个资源 label 的订阅者。
- **分发**：配置变化时 center 的 `refresh` 重算每个订阅者所属 label 的 policy，**只在变化时**回调。

### 2.1 感知层：Source 契约（2026-08-15 起）

治理规则的**生效链**（快照原子替换 → label diff → executor 原地 Refresh）从第一天起就是治理中心自己的设计；感知层直接消费治理自己的 `Source` 接口（[source.go](source.go)）——`Snapshot() Config` + `Subscribe(cb)`，两个方法。对照业界（Sentinel-Golang 的 ext/datasource、dubbo-go 的 DynamicConfiguration）：治理规则的模型与生效链自建、传输层做成可插拔适配，是主流共识；Spring Cloud 把治理深耦合进通用刷新机制（@RefreshScope）恰是被验证的弯路。

治理配置**不挂 gs 的通用属性刷新管道**：规则是它自己的一份文档（本地文件、远程控制台、配置中心），经 Source 契约进入中心，改一条规则只刷新治理，不触发全应用属性重绑。

- **来源**：`governance.SetSource(任意实现)`，或 bean 注入（必须 `Export(gs.As[governance.Source]())`，否则接口注入找不到它、治理静默 disabled）。本模块内置 file / http 两种具体来源。优先级 SetSource > bean。
- **没有内置默认源**：不配置任何来源时，治理保持 disabled（`ExecutorFor` 返回透传 noop）。
- **单源替换、不内置 merge**：`Rules` 是整体替换语义（0 = disabled），合并策略属于组合 Source 实现的职责。
- **换源安全**：换源靠活跃源守卫（handle 指针比较，规避接口值 `==` 的 panic 风险），旧源回调自动失效。
- **边界**：普通业务配置（如 `spring.dubbo.consumer`）继续用 `gs.Dync` 字段绑定——数据性质决定刷新机制：实例列表是高频运行态（discovery 的 watch），治理规则是低频配置态（Source 快照替换），普通开关是散配置（dync 字段绑定）。

> ⚠️ 看到代码里"到处都是 Register"是正常现象——这不是重复配置，而是 fan-out 拓扑的叶子节点。治理不再消费任何 `gs.Dync`，Register 数多恰恰是"精确分发"的实现方式。

> **client 怎么拿到 executor（2026-08-14 重构后）**：client starter **不注入治理中心**，而是调中立函数 `resilience.ExecutorFor(系统名, 资源label)` 拿到自己的 executor——零 govern 耦合。starter-governance 在启动时（wiring bean 的 Init → `GoLive`）把治理中心注册成 `ExecutorFor` 背后的 provider；上面的 `Register(label, cb)` 扇出由 provider 内部按 label 自动完成，client 不感知。`ExecutorFor` 返回的 executor 每次 Execute 时 lazy resolve（全局 memoize），与 provider 注册先后无关；无 provider 时返回透传 noop。resolve 时会顺带把 observe 层应用到**尚未发布**的 executor 上，所以 client 拿到的已是组装好的 executor，不再自己调 `resilience.WrapExecutor`。唯一例外是 **dubbo**（URL-param 模型，直接走门面 `PolicyFor` 读策略）。seam 代码见 [cloud/governance/resilience/provider.go](resilience/provider.go)。

> **client 怎么拿到选点策略（2026-09-12）**：端点选择走的是**同一个形状的第三条中立 seam**。`loadbalance.RegisterSelectionProvider` 由治理中心在 `GoLive` 时注册（与 `ExecutorFor` / `InjectorFor` 并列），client 只调 `pool.BindSelection(label)`——同样不 import `cloud/governance`。差别在落点：executor 的 sink 是一个**对象**（按 label memoize 后由 client 持有），选点的 sink 是 **caller 的 `*Pool`**，而同一个 label 可能合法地对应多个池（两个 entry 共用一个 service-name、client 被重建），所以这里**不 memoize**：每次 bind 一条独立订阅，返回的 stop 就是它的 Cancel。`loadbalance` 侧只认值（`Selection{Balancer, OutlierThreshold, OutlierSuspendFor}`），不认 policy 模型；未知策略名照旧忽略、保留上一个可用值。seam 代码见 [cloud/loadbalance/selection.go](../loadbalance/selection.go)。

## 3. 为什么是 per-label Register，而不是一个全局回调

因为不同资源可以有**不同的 policy**（通过 `Rules` 列表按 label 匹配）。redis 可能配了专属 Rule，gorm 走 default。如果用一个全局回调，任何一次配置改动都会把所有资源的 executor 都重建一遍。

per-label Register 配合"上次交付值"（`subscriber.last`）做**选择性扇出**：

- 只改了 redis 命中的那条 Rule 时，gorm / mongo 等算出来的 policy 与上次相同 → 被跳过，不触发无谓的 executor 重建。
- 这要求 `policyEqual` 能判断"是否真的变了"。`resilience.Policy` 含 `RetryPredicate` 函数字段不能直接 `==`，但 center 产出的 policy 都来自 `resilience.Config.Policy()`，该函数字段恒为 nil（value tag 绑不出函数），所以 `reflect.DeepEqual` 在这里是精确的。

## 4. 核心不变量

| 不变量 | 如何保证 | 代码位置 |
|---|---|---|
| 热路径无锁 | `cfg atomic.Pointer[Config]`，`PolicyFor` 只做一次原子 load | `PolicyFor`（门面 → center） |
| 选择性分发 | 每个订阅者记 `last`，`Refresh` 时 DeepEqual 比对，未变不回调 | center.`refresh` |
| 回调在锁外执行 | 锁内只收集 `todo`，锁外逐个调用；回调里再调门面不会自死锁 | center.`refresh` |
| Disabled 即透传 | center 未启用时 `PolicyFor` 返回零值 `Policy{}`，executor 成为透明直连 | `PolicyFor`（门面 → center） |
| 不导入 starter-governance 也是 no-op | center 是无条件单例，未绑定任何 source 时 `Enabled()==false` | `cloud/governance` |

## 5. 公共 API

```go
type Config struct {
    Enabled bool                // 总开关；false 时 PolicyFor 恒返回零值
    Driver  string              // 韧性后端："default" / "sentinel"，全进程统一
    Default resilience.Config   // 兜底策略：没有 Rule 命中的资源都用这份
    Rules   []Rule              // 逐资源策略列表，first-match
}

type Rule struct {
    Resources []string          // 该 Rule 匹配的资源 label（逗号分隔多个）；空=不匹配
    resilience.PolicyConfig     // 内嵌：策略字段直接绑在 rules[N].*（gs 提升内嵌 value tag）
                                // 含保护类（timeout/max-retries/error-threshold/…）
                                // 与端点选择类（balancer/outlier-threshold/outlier-suspend-for）
}

// center 纯内部：类型系统不设导出，全仓只有包级门面
func Enabled() bool
func Driver() string
func PolicyFor(label string) resilience.Policy   // 读路径，无锁；遍历 Rules 找首个命中，否则 Default
func Register(label string, cb func(resilience.Policy)) Subscription
                                                 // Subscription.Policy = 置入时用的策略
                                                 // Subscription.Cancel() = 摘除（重建型消费方必须调）
func SetSource(s Source) / BindDefault(src Source) / GoLive() / CloseActiveSource() error
func OnReady(cb func()) / Arm(cfg Config) / Reset()
```

**为什么是 `Rules` 列表而不是 `map[label]`**：资源 label 用冒号分段（`gorm:mysql:orders-db`）。若 label 做 map key，冒号进到 YAML key 位置会让映射解析错乱（每个 key 都得加引号、漏一个就静默解析错）。改成列表后，label 退到 `resources` 值的位置——冒号在值里，properties 不转义、YAML 不加引号，两种格式都干净。`PolicyFor` 遍历 Rules（每进程就几条，O(n) 可忽略）找首个 `Resources` 含 label 的，找不到回落 Default。

**关于 Rule 是"整体替换"而非"字段合并"**：因为 `resilience.Policy` 字段为 0 表示"禁用"，字段级合并无法区分"显式设为 0"和"未设置"。给某资源配 Rule，意味着你要一份与 Default 完全不同的、自包含的策略（要保留的 default 字段得抄进 Rule）。

## 6. 资源标签（label）约定

label 是 `PolicyFor` / `Register` 的 key，决定一条策略归属哪个资源。所有 starter 通过 [`resilience.ResourceLabel`](resilience/config.go) 统一拼接：第一个非空 name 拼到 prefix 后，都没有则只用 prefix。

| starter | label 格式 | 示例 |
|---|---|---|
| starter-redigo | `redigo:<service-name\|addr>` | `redigo:cache-svc` |
| starter-go-redis | `redis:<service-name\|master-name\|addr>` | `redis:primary` |
| starter-gorm-mysql | `gorm:mysql:<service-name\|addr>` | `gorm:mysql:orders-db` |
| starter-gorm-postgres | `gorm:postgresql:<service-name\|host>` | `gorm:postgresql:primary` |
| starter-gorm-sqlserver | `gorm:sqlserver:<service-name\|host>` | |
| starter-gorm-clickhouse | `gorm:clickhouse:<service-name\|addr>` | |
| starter-mongodb | `mongodb:<service-name\|uri>` | |
| starter-elasticsearch | `elasticsearch:<service-name\|cloud-id\|addr>` | |
| starter-neo4j | `neo4j:<service-name\|uri>` | |
| starter-bigcache | `bigcache:<name>` | |
| starter-memcached | `memcached:<name>` | |
| starter-gin（入站） | `gin:<address>` | `gin::8080` |
| starter-echo（入站） | `echo:<address>` | `echo::8080` |
| starter-grpc（入站） | `grpc:<addr>` | |
| starter-thrift（入站） | `thrift:<addr>` | |
| starter-http-server（入站，中间件库） | `http-server:<address>` | `http-server::9090` |
| starter-grpc（客户端内置 balancer） | `grpc:client`（进程级默认，不指向具体服务） | |
| starter-http-client | `http:<service-name\|addr>` | `http:user-svc` |
| starter-oauth2-client | `oauth2:<client-id>` | |
| starter-kafka / kafka-sarama | `kafka:<brokers>` | |
| starter-rabbitmq | `rabbitmq:<vhost\|url>` | |
| starter-nats | `nats:<name\|url>` | |
| starter-mqtt | `mqtt:<broker>` | |
| starter-pulsar | `pulsar:<url>` | |
| starter-gateway（端点选择） | `gateway:<route-id>`（每条路由一个） | `gateway:api/v1` |
| starter-gateway（保护策略） | `gateway:<resilience-policy-name>`（按 policy 共享） | `gateway:strict` |
| starter-dubbo | `dubbo:<app>` / `dubbo:<interface>:<version>:<group>` | 见 §7 |

**给一个资源配置策略**，把上表中的 label 作为 `govern.rules[N].resources` 的值。具体写法见 [CONFIG_CN.md §3](./CONFIG_CN.md#3-多-starter-项目给不同资源配不同策略)。

**同一个 label 也管端点选择**：凡是走发现模式建了 `loadbalance.Pool` 的 client（`http` / `gateway` /
gorm 四方言 / `redigo` / `redis` / `mongodb`），都用上表里它自己那一行的 label 去 `BindSelection`，
所以一条 Rule 同时决定这条资源的保护策略**和**选点策略（`balancer` / `outlier-threshold` /
`outlier-suspend-for`）。`gateway` 的保护与选点用两个不同 label（policy 名 vs route id），是表里已
注明的既有例外。

三类刻意的缺口，判据都是"**这条 client 每次请求/每次建连会重新挑端点吗**"：

- 直连（固定 addr/host）：没有候选集可选。
- **只挑一次**（`neo4j`）：端点被固化进启动 URI，没有"每次挑选"可管，治理只到保护策略。
- **成熟库自持选择**（`elasticsearch` / `memcached` / MQ 族 / `s3`）：选择权在库内部（ES 的节点
  选择器、memcached 的按 key 一致性哈希）。这不违反主线——**库已有成熟选择器时不自己造一个**，
  只把实时地址喂给它：ES 与 memcached 都装了自己的活池/活选择器，地址集持续跟随命名服务
  （另外两条轴，见 [CONFIG_CN.md §3.1](./CONFIG_CN.md#31-端点选择负载均衡策略--剔除) 的表）。

`grpc` 是个半口子：选点标签是进程级的 `grpc:client`（service config 仍是它的 per-client 选择，治理是
叠在上面的进程级默认）；要按服务隔离策略，用 `RegisterBalancer` 注册一个自定义名字——它按设计豁免
进程级覆盖。逐条口径见 [CONFIG_CN.md §3.1](./CONFIG_CN.md#31-端点选择负载均衡策略--剔除)。

## 7. dubbo 的特殊适配

dubbo 有自己的 URL-param 治理模型（timeout / retries / loadbalance / cluster 等是 provider/consumer URL 上的参数），不直接走 resilience Executor 这条路。govern 对它的适配方式：

- **label**：应用级 `dubbo:<app>` + 每个 reference 的 `dubbo:<interface>:<version>:<group>`，由 `dubboResourceLabels` 产出。
- **桥接**：[`starter-dubbo/dync.go`](../../starter/starter-dubbo/dync.go) 的 `poll` 把 center 的 `PolicyFor` 翻译成 dubbo 参数——`Policy.Timeout` 写成毫秒数的 `timeout`，`Policy.MaxRetries` 写成 `retries`（cluster-failover 级别，不是 resilience 层 retry）。
- **热更新**：对每个 dubbo label 调门面 `governance.Register`，回调里重新 `poll` 并 `RefreshOverrideRules` 推给 dubbo-go 的动态配置层。
- **dubbo 专属旋钮**（loadbalance / cluster / serialization）留在 dubbo 自己的配置段，不进 govern。

> Register 的回调里会重入 `poll`，所以 dubbo 在锁**外**收集待注册 label、锁**内**去重登记，避免持锁跨 `Register` 自死锁。这是回调在锁外执行这一不变量（§4）的一个具体应用。

## 8. 与 fault 注入的关系（已落地）

fault（"放火"）已**收进治理中心**,和 resilience 共用同一个 `governance.Config`——也就是同一份规则文档。`governance.Config` 嵌入 `fault.Config`(`value:"${fault:=}"`,绑成 `govern.fault.*`),center 在启动时从当前快照**无条件**建一个全进程共享的 `*fault.Injector`(Enabled=false 即 no-op),注册到中立 seam。每次 source push(`adopt`)在 resilience 的 `Refresh` 之外,额外 `injector.SetConfig(new.Fault)` 原地热更。

**集中形态比 resilience 更简单——没有 per-label 解析,只有一个全局 injector。** 区别在于:

- `resilience.Policy` **没有**内置的多资源定向能力,redis 和 gorm 是两个独立 Policy 对象,所以 center 必须按 label 解析出"属于你的那一个"。
- [`fault.Config`](fault/config.go) **天生带**多资源定向:`Rules []Rule` 每条有自己的 `Resources / Rate / Latency / Error`,一份 Config 即可描述"redis 打 0.5 错误、gorm 加延迟、其余全量慢调用"。所以 fault 不需要 per-label injector,一个全局 injector 在 `maybe(resource)` 时按 Rules 分发即可。

starter 侧通过中立 seam 接入,零耦合 cloud/governance:

- **client 侧**:`fault.WrapExecutor(resilience.ExecutorFor(系统名, r))`。`ExecutorFor` 已自带治理与 observe 组装,`WrapExecutor` 只在最外层做放火;不传 injector 时**惰性**在每次 `Execute` 解析 `InjectorFor()`——这镜像了 `resilience.ExecutorFor` 的 call-time 延迟解析,使 starter 能在 `Init`(早于 centerHolder 的 `Run` 注册)就装配 fault 层而不丢失注入器。
- **server 侧**(gin/echo/hertz/grpc/trpc/dubbo):中间件/拦截器 per-call 调 `fault.Apply(ctx, fault.InjectorFor(), label, ...)`。`Apply` 对 nil injector 透明直通,所以这些中间件现在**无条件安装**(旧的"启动时必须 Enabled 才装"限制随之消除)。

附带影响(已接受):`Injector` 的 `MaxAffected` / `MaxDuration` 安全熔断计数器从"每资源各算"变成"全进程合计"。作为安全保险,全局语义更正确(不会因资源数被乘 N)。

顺带修复了集中前的潜在缺陷:旧版 Pattern A starter 的 `OnChanged` 仅在启动时 `Enabled==true` 才注册,运行时热更打开 fault 完全无效(gin 注释直言"toggle via restart")。集中化后 fault 可在任意时刻热开关。

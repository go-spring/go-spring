# cloud/governance — 服务治理家族

本目录收拢 go-spring 的**运行期服务治理**能力。治理家族的成员各自代表治理的一个维度，彼此是**平级**关系，而非父子从属：

| 子包 | 角色 | 说明 |
|------|------|------|
| `.` (package governance) | **控制面** | 集中式治理中心：单一可热更 `Config` → 一次 fan-out。`center` 纯内部，client 只调包级门面 `PolicyFor`/`Register`。 |
| `resilience/` | **原语** | 防御原语：熔断 / 限流 / 重试 / 退避 / `Executor` 接缝。叶子包，不依赖治理家族其他成员。 |
| `fault/` | **混沌面** | 故障注入（fault injection），借用 `Executor` 接缝注入失败。与 resilience 互为攻防。 |
| `traffic/` | **流量识别面** | 压测/灰度流量标识的识别与传播。 |

## 配置感知：Source 契约

治理中心自成体系：它消费自己的 `Source` 接口（[source.go](source.go)）——一个配置快照加一个变更订阅，两个方法。整条生效链（label diff → executor 原地 Refresh → client 无感）只依赖这个契约，不感知配置从哪来。

```
治理文件 / 远程控制台 ──┐
governance.Source bean ──┼──→ center（单一活跃源）→ adopt → Refresh + fault.SetConfig
governance.SetSource ────┘
```

文档格式由 `rules.Parse(name, data, format)`（starter-governance 的 `rules` 子包，spring/conf 与容器无关治理核心之间的共享解析胶水）统一：任何后端送来的规则文档（文件字节 / HTTP body / 配置中心 value）都用同一套解析与键语义（`govern.*` 键），规则文件跨后端逐字节可移植。解析成功但没有任何 `govern.*` 键的文档一律报错（防截断静默关治理）。

优先级：`SetSource` > bean 注入。**没有内置默认源**——不配置任何来源时治理保持 disabled（`ExecutorFor` 返回透传 noop）。治理配置不挂 gs 的通用属性刷新管道。

自定义 Source 的三种接入方式：

```go
// ① 推送式（治理控制台 / 定时拉取 / 测试）：PushSource 开箱即用
src := governance.NewPushSource(governance.Config{})
governance.SetSource(src)
// 每次上游事件：
src.Push(newCfg) // resilience 策略与 fault 演练配置一起热更

// ② bean 注入（Source 需要自己的依赖与生命周期时）——Export 不可省略，
//    否则接口注入找不到它，治理静默 disabled
gs.Provide(newConsoleSource).Export(gs.As[governance.Source]())

// ③ 直接实现 Source 接口（如监听配置中心专用 key）
type nacosRuleSource struct{ ... }
func (s *nacosRuleSource) Snapshot() governance.Config          { ... }
func (s *nacosRuleSource) Subscribe(cb func(governance.Config)) { ... }
```

设计要点：单源替换、不内置 merge（`Rules` 是整体替换语义，合并策略属于组合 Source 实现的职责）；换源靠活跃源守卫（stale guard），旧源回调自动失效；`SetSource` 在 Init 前后调用均安全（后者为 late-arm，新源快照立即生效）。普通业务配置（如 `spring.dubbo.consumer`）与治理规则的动态化是两套机制：前者继续用 `gs.Dync` 字段绑定，后者走 Source 契约。

开箱即用的自建链路适配器已覆盖多种后端：

| 后端 | 所在 starter | 一行接线 |
|---|---|---|
| 独立规则文件（fsnotify） | starter-governance | `govern.source.file.path=...` |
| 治理控制台 / 规则 API（轮询拉取） | starter-governance | `govern.source.http.url=...` |
| Nacos 直连（专用 dataId，ListenConfig 推送） | starter-governance-nacos | `govern.source.nacos.server=...` + `data-id=...` |
| etcd 直连（专用 key，Watch 推送） | starter-governance-etcd | `govern.source.etcd.endpoint=...` + `key=...` |

## 为什么是平级伞包，而非挂到 resilience 下

`resilience` 是叶子原语，`governance`（控制面）、`fault`（混沌面）、`traffic`（识别面）都**依赖** resilience，是它的消费方。依赖方向为：

```
resilience   ← 叶子原语（不 import 家族其他成员）
governance   ──→ resilience   (PolicyFor 返回 resilience.Policy)
fault        ──→ resilience + traffic
traffic      ← 纯叶子
```

若把 govern/fault 塞进 `resilience/` 当子包，等于宣称"控制面是原语的一种""混沌是原语的一种"——这是假层级，且颠倒了控制方向（govern 驱动 resilience）。中性伞包 `governance` 聚拢它们，既显式化家族关系，又不制造从属。

## 不属于治理家族（仍在 cloud/ 下平级存在）

- `cloud/loadbalance`、`cloud/discovery` — 端点选择与服务注册，与 discovery 成对。
- `cloud/actuator`、`cloud/mesh`、`cloud/tlsconf` — 运维/网格/TLS，独立关注点。
- `cloud/experimental/loadtest` — 测试工具；`cloud/experimental/transaction` — 分布式事务（仍在孵化）。
- resilience 的插桩在本包自身（`resilience/observe.go` 的 `WrapExecutor`，由
  `ExecutorFor` 在 resolve 时应用到尚未发布的 executor 上，client 不直接调用）。

## 为什么定时任务（cloud/scheduling）刻意不接治理（拍板：2026-09-15）

逐项审过治理五件套对定时任务的价值，结论是**不接 `resilience.ExecutorFor`，也不给
scheduling 资源标签**：

- **限流**无意义——触发频率由 cron / fixed-rate 自己定，限一个自己发起的固定频率没有对象。
- **重试**有害——定时任务几乎全是副作用型（刷库、发消息、对账），executor 重试等于短时间
  内同一任务跑两遍；失败后下个周期自然再跑，这才是它的正确重试语义（Skip/Queue 策略已表达）。
- **熔断**负收益——定时任务打下游是低频的，打不坏下游；连续失败时 breaker 打开的效果
  = 任务静默不跑，比"跑失败 + 错误日志可见"更糟（静默跳过正是定时任务最危险的故障模式）。
- **超时**唯一有用，但 scheduling 已有本地 `WithTimeout`（且 Replace 策略可取消卡死运行）。
- **剔除/选点**无端点选择，不适用。

本质差异：请求路径治理的目标是"保护调用方不被下游拖垮"；定时任务自己是调用方、频率自控、
天然有"下周期重跑"这个内置恢复机制——治理中心收敛的那 11 份 per-starter Dync 痛点在这里
不存在。**要治理的是任务体内发起的那次调用**（如任务里用 starter-http-client 发请求——
那已经走了 `http:` 标签的治理），而不是任务本身。定时任务自身的健康由
`scheduling.runs` 的 outcome（ok/error/panic/skipped_*）+ lag 指标与报警覆盖，
"连续 error/panic"报警比任何 breaker 都适合这个场景。

## 配置

治理规则使用 `govern.*` 键命名空间（如 `govern.enabled`、`govern.default.*`、`govern.rules`），与 Go 包名 `governance` 独立。规则是**独立文档**，经 Source 契约进入中心，不写进 `app.properties`——本地文件用 `govern.source.file.path` 引导（见 [starter-governance](../starter-governance/README.md)）。

本包**容器无关**（不 import spring/gs）：gs 接线（绑定注入的 Source、seam 注册、`GoLive`）在 **starter-governance** 的常驻 wiring bean 里。应用侧：

```go
import _ "go-spring.org/starter-governance" // 接线治理中心 + 装载规则来源
```

非 gs 运行时直接用门面：`governance.Arm(cfg)` 或 `governance.SetSource(src)` + `governance.GoLive()`。

---

# 治理中心设计说明

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
func NewExecutor(p resilience.Policy) (resilience.Executor, error)
                                                 // 按当前配置的 driver 建 executor（httpx 中心路径用）
func SetSource(s Source) / BindDefault(src Source)
func BindDrivers(m map[string]resilience.Driver)  // 接线期装驱动目录，须先于 GoLive
func GoLive() / CloseActiveSource() error
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

**给一个资源配置策略**，把上表中的 label 作为 `govern.rules[N].resources` 的值。具体写法见 [配置指南 §3](#3-多-starter-项目给不同资源配不同策略)。

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
  （另外两条轴，见 [§3.1](#31-端点选择负载均衡策略--剔除) 的表）。

另一个刻意缺席：**cloud/scheduling（定时任务）不在本表**——它不接治理，判据见前文
"为什么定时任务刻意不接治理"。

`grpc` 是个半口子：选点标签是进程级的 `grpc:client`（service config 仍是它的 per-client 选择，治理是
叠在上面的进程级默认）；要按服务隔离策略，用 `RegisterBalancer` 注册一个自定义名字——它按设计豁免
进程级覆盖。逐条口径见 [§3.1](#31-端点选择负载均衡策略--剔除)。

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

---

# 治理中心配置指南

本文讲**怎么写治理配置**。治理配置是**它自己的一份文档**——本地规则文件（本模块的 file source 盯着它）、远程控制台，或配置中心——经 `governance.Source` 契约进入治理中心。它**不写进 `app.properties`**：改一条规则只刷新治理，不触发全应用的属性重绑。

设计原理（为什么是 Source 契约、per-label Register 怎么分发）见下文[设计说明](#1-它解决什么问题)；资源标签（label）格式表见[设计说明 §6](#6-资源标签label约定)。

适用于一个项目里**同时用多个 starter**（redis + gorm + http-client + gin 入站……）的场景。

---

## 1. 前置：引入 starter-governance + 指定规则来源

治理不是自动生效的，两步：

1. 程序入口 blank import `starter-governance`——它注册接线 bean，把规则来源交给治理中心并启动它。
2. 在 `app.properties` 里用**一个引导 key** 告诉它规则从哪来（本地文件为例）：

```properties
# app.properties —— 治理的“引导”配置，只有这一行
# 规则内容不在这里，在 conf/govern.properties
govern.source.file.path=conf/govern.properties
```

```go
import (
    _ "go-spring.org/starter-governance" // 启动时接线治理中心，装载规则来源
    _ "go-spring.org/starter-redigo"     // 你的业务 starter
    // ... 其他 starter
)
```

> **client starter 不需要 import（也不注入）治理。** 每个 client（redis/gorm/http/…）只调中立函数 `resilience.ExecutorFor(系统名, 资源label)` 拿到它的 executor——不知道治理的存在。`governance.Source` 契约把治理核心和“规则从哪来”解耦，因此控制台推流、专用配置中心 listener、静态注入都能驱动它，而本地文件只是其中一种。

**不 import starter-governance 时**：没有 provider 注册，`ExecutorFor` 返回透传的 noop executor，resilience 完全旁路（直连后端），不会报错。所以“没配治理”和“不能用 starter”是两回事。

**其它规则来源**：

- `govern.source.http.*`（轮询远程控制台/规则 API，见 [starter-governance README](../../starter/starter-governance/README.md)）；
- config 中心的 source 适配器（nacos/etcd）各自独立成模块（`starter-governance-nacos`、`starter-governance-etcd`），内容同样是这份 `govern.*` 文档；
- 代码里 `governance.SetSource(...)` 静态注入或推流。

优先级：显式 `governance.SetSource` > 由 bean 注入的 source（如上面 file/http 的）。

**例外：两个直接走门面的消费方**（center 类型本身不导出，它们只用门面函数）：

- **starter-dubbo**：走自己的 URL-param 治理模型（timeout/retries 是 dubbo 参数，不走 resilience executor），所以直接调 `governance.PolicyFor` 读策略字段。
- **starter-gateway**：它的路由池是**每次路由表重编译重建**的，订阅必须能随旧池一起撤销——用 `governance.Register` 拿回 `Subscription`，重建时 `Cancel()`。保护策略仍走中立的 `resilience.ExecutorFor` seam（同其它 client），只有**端点选择**这一半走门面，因为选择策略没有对应的中立 seam，为单一实现硬造一个不划算。

---

## 2. 最小可用：一份默认策略管全部

最常见用法——全进程所有资源共享同一份韧性策略。规则写在**规则文件**里（下面是 `properties`，YAML 键同构）：

```properties
# conf/govern.properties —— 治理规则，独立文档
govern.enabled=true
govern.driver=default          # 或 sentinel；全进程统一后端，一处切换处处生效

govern.default.enabled=true
govern.default.timeout=500ms
govern.default.max-retries=1
govern.default.rate-limit=100       # ops/s，0 表示不限流
govern.default.error-threshold=20   # 连续失败 20 次熔断
govern.default.open-duration=5s     # 熔断持续 5s 后半开试探
```

配完这份文件（加上 §1 的 `govern.source.file.path`），项目里的 redis、gorm、mongo、http-client……全部自动套用这套超时/重试/限流/熔断，且**热重载**——file source 盯着这个文件，改完不用重启（远程 source 同样的效果）。

> 规则文件里的键就是 `govern.*` 命名空间，与过去的 `${govern}` 属性一字不差。`govern.default.*` 下可用字段是 `resilience.Config` 的全部旋钮：`timeout` / `max-retries` / `rate-limit` / `burst` / `error-threshold` / `open-duration` / `breaker-strategy`(consecutive|error-rate) / `error-rate-threshold` / `min-requests` / `breaker-window`，以及端点选择类的 `balancer` / `outlier-threshold` / `outlier-suspend-for`（见 §3.1）。字段含义见 [cloud/governance/resilience/config.go](resilience/config.go)。

---

## 3. 多 starter 项目：给不同资源配不同策略

真实项目里 redis 和 gorm 的容忍度不一样。用 `govern.rules[N]` 给特定资源单独配——**资源 label 写在 `resources` 值里**（不是 key），所以冒号随便写、properties 不转义、YAML 不加引号：

```properties
# conf/govern.properties
govern.enabled=true
govern.driver=default

# 默认策略：兜底，大部分资源用这个
govern.default.enabled=true
govern.default.timeout=1s
govern.default.max-retries=2

# redis 单独收紧：缓存快失败，少重试
govern.rules[0].resources=redigo:cache
govern.rules[0].enabled=true
govern.rules[0].timeout=200ms
govern.rules[0].max-retries=0

# mysql 放宽：数据库慢查询多，超时给宽
govern.rules[1].resources=gorm:mysql:orders-db
govern.rules[1].enabled=true
govern.rules[1].timeout=3s
govern.rules[1].max-retries=1

# http 下游服务按服务名
govern.rules[2].resources=http:user-svc
govern.rules[2].enabled=true
govern.rules[2].timeout=800ms
```

YAML 规则文件里同样干净（冒号在值里，不是 key）：

```yaml
# conf/govern.yaml
govern:
  enabled: true
  driver: default
  default:
    enabled: true
    timeout: 1s
    max-retries: 2
  rules:
    - resources: redigo:cache
      enabled: true
      timeout: 200ms
      max-retries: 0
    - resources: gorm:mysql:orders-db
      enabled: true
      timeout: 3s
    - resources: http:user-svc
      enabled: true
      timeout: 800ms
```

### 为什么是 `rules[N]` 而不是 `override.<label>`

早期版本曾用 `govern.override.<label>.<field>`，把资源 label 当 map key。但 label 用冒号分段（`gorm:mysql:orders-db`），冒号进到 YAML key 里会让映射解析错乱（`gorm:mysql:orders-db:` 被当成嵌套映射），每个 key 都得加引号、漏一个就静默解析错。所以改成列表形式：**label 退到 `resources` 值的位置**，key 永远是 dot-safe / colon-safe 的数字索引，两种配置格式都自然。

### 3.1 端点选择（负载均衡策略 + 剔除）

除了"调用怎么被保护"，同一个资源的"调用发给谁"也在这份文档里：`balancer` 选策略，
`outlier-threshold` / `outlier-suspend-for` 决定一个反复失败的实例多久被摘出候选集。

```properties
# 发现模式下路由到 user-svc 的 client：改用最少连接，且连续失败 5 次摘除 10s
govern.rules[3].resources=http:user-svc
govern.rules[3].enabled=true
govern.rules[3].balancer=least_conn
govern.rules[3].outlier-threshold=5
govern.rules[3].outlier-suspend-for=10s
```

- **`balancer`**：`round_robin`(默认) / `least_conn` / `consistent_hash` / `weighted` / `zone_aware`（包内还注册了 `random` / `p2c`，公司策略名如 `luohua` 同理）。留空 = 保持该 client 的默认（round_robin）。写错策略名会被**忽略并沿用当前策略**，不影响调用——治理文档没有错误通道（"你推什么，你担保什么"）。
- **`outlier-threshold`**：与 `error-threshold` 是同一套语义（连续失败 + 半开试探），区别在作用对象——`error-threshold` 熔断的是**整个资源**，`outlier-threshold` 摘的是**单个实例**。0 表示不摘除。
- 两个旋钮都**原地生效**：改完 push，下一次请求就走新策略/新阈值，不用重启，也不会重建 transport。（策略自身的状态不跨切换保留——`least_conn` 的在途计数、`consistent_hash` 的哈希环、p2c 的延迟模型都会重来。）

**覆盖到的客户端**：所有走发现模式的 `loadbalance.Pool` 消费者——`http`（starter-http-client）、
`gateway`（每条路由）、`gorm` 全部方言、`redigo`、`redis`（go-redis）、`mongodb`。它们用
**与保护策略相同的资源标签**绑定（`resilience.ResourceLabel`，见 §6 表），所以一条 Rule 同时管住
一条资源的超时/重试/熔断和它的选点策略。

**覆盖不到的客户端，三条理由，都是有意为之**：

- **直连（固定 addr / host）**：没有候选集可选，这条路整体旁路。`neo4j` `memcached`
  `elasticsearch` 等只要不配 `service-name` 就属于这一类。
- **只挑一次的客户端——`neo4j`**：启动时挑一个端点并把主机名固化进 URI，之后没有"每次挑选"，
  订阅了也影响不了任何一次决策。它的治理只到保护策略为止。
- **成熟客户端自持选择的——`elasticsearch`、`memcached`**：选择权在库内部——ES 有自己的节点选择器，
  memcached 的选择就是按 key 做一致性哈希（按 key 亲和正是它的语义，把一个可重排的池套上去会直接
  破坏它）。**这不等于没得配**：这类客户端的选点规则在它自己的配置里（如 ES 的节点选择器、
  `servers` 顺序），govern 不插手——与 MQ/broker 族（kafka/pulsar/rocketmq/nats/mqtt/rabbitmq）
  以及 `s3` 一样，它们的"发给谁"是协议层/集群层的事。判据见设计说明 §6 的中立层边界：**库里已经有
  成熟选择器时不自己造一个**。

> **别把"选择权归属"和"地址新鲜度"混成一件事**——它们是两条轴：
>
> | | 谁挑节点 | 地址集跟随命名服务 |
> |---|---|---|
> | `http`/`gateway`/gorm/`redigo`/`redis`/`mongodb` | `loadbalance.Pool`（可配 `balancer`） | ✅ 每次建连/每请求重读 |
> | `elasticsearch` | ES transport 的选择器 | ✅（1s 传播预算，见 starter-elasticsearch USAGE） |
> | `memcached` | 库的按 key 一致性哈希 | ✅ 每次 key 查找重读 |
> | `neo4j` | driver | ⚠️ `bolt://` 不重读；`neo4j://` 靠 `AddressResolver` 在种子主机消失后重找集群 |
>
> 所以"新实例不生效"对 ES/memcached 已经不是问题；`balancer` 对它们仍然无效，因为挑节点的不是我们。

> 一句话自查：**这条 client 每次请求/每次建连会重新挑端点吗？** 会 → 它的资源标签就能配 `balancer`；
> 不会（只挑一次、或交给库内部挑） → 别指望 `govern.rules[N].balancer` 对它生效。

**两个语义边界**（写规则前值得知道）：

- **`grpc` 客户端**的策略选择天然是 gRPC service config（`grpc.WithDefaultServiceConfig(LoadBalancingConfig(s))`）。
  治理的 `balancer` 是**叠在它上面的进程级默认**，标签固定为 `grpc:client`——所以给 `grpc:client` 配
  `balancer=` 会同时改掉**所有**内置 `gs_*` balancer 的策略（与它原本就管的摘除阈值行为一致）。不做
  进程级覆盖时，service config 的选择原样生效；`RegisterBalancer` 注册的自定义名字不受影响（它们存在
  的意义就是保留自己的策略）。
- **DB/缓存客户端的剔除粒度是"连接"而非"查询"**：gorm / redigo / go-redis / mongodb 的挑选发生在
  建连时，能喂给 `Tracker` 的成败信号只有 dial 结果本身。所以这些 client 上 `outlier-threshold` 摘的是
  **反复连不上**的实例；单条查询/命令的失败由它们各自的 resilience executor 管，不参与点数。

### 3.2 让某一条资源不上治理

客户端一律会挂 executor，**没有 per-resource 的治理开关**——开关是进程级的（`govern.enabled`）。
想让某条资源事实上不受治理（裸调用），给它配一条**所有旋钮都为 0 的 Rule** 即可：Rule 是整体替换
default，全零 Rule 算出来的就是零 Policy，executor 退化为透传。

```properties
# kafka:<brokers> 这条 client 不上治理
govern.rules[0].resources=kafka:10.0.0.1:9092
# 字段全不写 = 全零 = 透传
```

> 早期版本里 kafka / mqtt / pulsar / rabbitmq 各有一个 per-instance `${governance:=true}` 开关，
> 已删除：它表达的就是"这条资源不上治理"，而这件事 Rule 已经能表达，且可热更。

### 几条规则

- **一条 Rule 可匹配多个资源**：`govern.rules[0].resources=redigo:cache,redigo:session`（逗号分隔），这几个资源共享同一份策略。
- **Rule 是整体替换，不是字段合并**：给 `redigo:cache` 配了 Rule，它就**完全不继承** `govern.default`——漏写的字段按零值处理（零值=禁用该能力）。只想微调一个字段的话，把 default 里要保留的字段也抄进 Rule。
- **多条 Rule 命中同一 label 时，前面的赢**（first-match）。所以具体的 Rule 放前面。
- **`resources` 留空不匹配任何资源**——兜底请用 `govern.default`，不要用空 resources 的 Rule。

### 怎么知道某个资源的 label 是什么？

查[设计说明 §6](#6-资源标签label约定)的表。标签由 starter 用 `resilience.ResourceLabel(prefix, names...)` 拼接——取第一个非空的 name。所以 label 的取值取决于你配置里填的是 `service-name` 还是 `addr`：

- 你配了 `spring.redigo.instances.cache.service-name=cache-svc` → label 是 `redigo:cache-svc`
- 没配 service-name、只有 `spring.redigo.instances.cache.addr=10.0.0.1:6379` → label 是 `redigo:10.0.0.1:6379`

**建议**：给每个资源配一个稳定的 `service-name`，让 label 可读、不随地址漂移。

---

## 4. 入站流量的治理（gin / grpc）

gin / grpc 是**入站**侧——策略作用在“处理一个进来的请求”上，label 用监听地址：

```properties
# conf/govern.properties
govern.enabled=true
govern.driver=default

govern.default.enabled=true
govern.default.timeout=2s             # 单个请求处理超时
govern.default.rate-limit=1000        # 入站限流 1000 QPS

# gin 监听 :8080 → label = gin::8080
govern.rules[0].resources=gin::8080
govern.rules[0].enabled=true
govern.rules[0].rate-limit=500
```

入站和出站可以共用同一个 `govern.default`，也可以用 Rule 把 API 网关的限流和数据库的超时分开。

### gateway 有两个 label（注意）

gateway 的一条路由对应**两个** label，因为它们作用在两个不同粒度的对象上：

| 作用对象 | label | 说明 |
|---|---|---|
| 保护策略（timeout / retries / 熔断 / 限流） | `gateway:<resilience-policy-name>` | 即 `spring.gateway.resilience.<name>` 的名字。多条路由引用同一 policy **共享** breaker 状态，所以按 policy 而非按路由。 |
| 端点选择（balancer / outlier-*） | `gateway:<route-id>` | 即 `spring.gateway.routes.<id>` 的 id。每个路由有自己的池（各 upstream 的候选集），所以按路由。 |

```properties
# 保护：所有引用 policy "strict" 的路由
govern.rules[0].resources=gateway:strict
govern.rules[0].enabled=true
govern.rules[0].timeout=2s
govern.rules[0].max-retries=1

# 选择：只有路由 api/v1 的上游
govern.rules[1].resources=gateway:api/v1
govern.rules[1].enabled=true
govern.rules[1].balancer=least_conn
```

---

## 5. dubbo 的治理

dubbo 走的是 URL-param 模型，govern 只覆盖它的 `timeout` 和 `retries`（cluster-failover 级别），label 是：

- 应用级：`dubbo:<app-name>`
- 每个 reference：`dubbo:<interface>:<version>:<group>`

```properties
# conf/govern.properties
govern.rules[0].resources=dubbo:com.example.UserService:1.0.0
govern.rules[0].enabled=true
govern.rules[0].timeout=300ms
govern.rules[0].max-retries=2
```

dubbo 专属旋钮（loadbalance / cluster / serialization）不进 govern，留在 dubbo 自己的配置段。

---

## 6. fault（放火）——随 govern 集中化，同一个 source 驱动 ⚠️

fault 注入已**收进治理中心**，和 resilience 共用同一个 `governance.Config`——也就是同一份规则文档（参见[设计说明 §8](#8-与-fault-注入的关系已落地)）。所以放火的 key 是 `govern.fault.*`（不再是顶层 `fault.*`）：

```properties
# conf/govern.properties
govern.fault.enabled=true
govern.fault.rate=0.5
govern.fault.error=generic
```

这一条会**同时给全进程所有 starter 放火**——redis、gorm、http-client、gin 入站……全部以 50% 概率注入错误。这是集中化的预期效果：starter 侧通过中立的 `fault.InjectorFor()` seam 拿到唯一的进程级 injector，starter 自己不再绑 fault 配置、也不 import 治理中心。

### 想只给某个资源放火

用 `govern.fault.rules[]` 做定向（catch-all 之外的细分）：

```properties
govern.fault.enabled=true

# 默认（catch-all）：不实际注入错误，只让框架进入“fault 模式”
govern.fault.rate=0

# 只给 redis 放火
govern.fault.rules[0].resources=redigo:cache
govern.fault.rules[0].rate=0.5
govern.fault.rules[0].error=timeout
```

> 注意 `govern.fault.rules[N].resources` 里的值要和该 starter 实际传给 injector 的 resource label 对上（client 侧是 `redigo:cache` 这类）。server 侧（gin/grpc/echo/hertz/trpc/dubbo）的 fault 走中间件/拦截器，per-call 解析 injector，label 规则见各 server starter 文档（grpc 的 label 是 `grpc:<FullMethod>`）。

### fault 的安全保险

放火忘了关很危险，fault 内置两个自愈上限：

```properties
govern.fault.max-duration=10m     # 放火 10 分钟后自动停（从第一次生效算）
govern.fault.max-affected=1000    # 累计影响 1000 次调用后自动停
```

**强烈建议**生产环境放火时必设其一，set fire and walk away 也不会烧到天荒地老。

> 注意：集中化后 `max-duration`/`max-affected` 是**进程级计数**（不再是 per-resource）。详见设计说明 §8 的取舍说明。

### fault 与真实流量

用 `govern.fault.scope` 限定只烧压测流量、不碰真实请求（依赖 cloud/governance/traffic 的压测标记）：

```properties
govern.fault.scope=loadtest   # 只给带压测标记的流量放火；真实流量不受影响
                              # 反向：real = 只烧真实流量；空 = 全烧（默认）
```

### 运行时热更

规则文档随 file/http/config-center source push 到中心，**改配置 push 即可在不重启进程的情况下开关 fault**——starter 通过 `fault.InjectorFor()` per-call 惰性解析，中心 `SetConfig` 原地热更。

---

## 7. 一份完整的多 starter 项目配置示例

一个同时用 gin（入站）+ redigo（缓存）+ gorm-mysql（DB）+ http-client（调下游）的项目。**业务配置**留 `app.properties`，**治理规则**独立一份：

```properties
# ============ conf/app.properties：业务 starter 配置 + 治理引导 ============
spring.gin.api.address=:8080
spring.redigo.instances.cache.service-name=cache
spring.redigo.instances.cache.addr=10.0.0.1:6379
spring.gorm.orders.driver=mysql
spring.gorm.orders.dsn=orders:pwd@tcp(10.0.0.2:3306)/orders
spring.http.user.service-name=user-svc
spring.http.user.addr=10.0.0.3:8081

# 治理规则不在这里，只给它指个文件
govern.source.file.path=conf/govern.properties
```

```properties
# ============ conf/govern.properties：治理规则，一处下发，处处生效 ============
govern.enabled=true
govern.driver=default

# 默认策略
govern.default.enabled=true
govern.default.timeout=1s
govern.default.max-retries=1
govern.default.rate-limit=200
govern.default.error-threshold=10
govern.default.open-duration=10s

# redis 收紧：缓存要快
govern.rules[0].resources=redigo:cache
govern.rules[0].enabled=true
govern.rules[0].timeout=100ms
govern.rules[0].max-retries=0

# DB 放宽：慢查询容忍
govern.rules[1].resources=gorm:mysql:orders
govern.rules[1].enabled=true
govern.rules[1].timeout=3s
govern.rules[1].max-retries=2

# fault：默认关，需要时翻开关
govern.fault.enabled=false
# 演练时打开：
# govern.fault.enabled=true
# govern.fault.scope=loadtest
# govern.fault.rules[0].resources=redigo:cache
# govern.fault.rules[0].rate=0.3
# govern.fault.rules[0].error=timeout
# govern.fault.max-duration=5m
```

入口：

```go
import (
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-gin"
    _ "go-spring.org/starter-redigo"
    StarterGormMysql "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-http-client"
)
```

---

## 8. 常见误区

| 误区 | 正解 |
|---|---|
| 把 `govern.enabled` / `govern.default.*` 写进 `app.properties` | 治理规则是独立文档，经 source 进入中心。`app.properties` 里只放 `govern.source.*` 引导 key。 |
| 在每个 starter 自己的配置段写 `resilience.*` | 已废弃。resilience 现在只认治理规则文档，starter 段里的 resilience 配置不生效。 |
| 在 client 自己的配置段写 `balancer` / `suspend-threshold` / `suspend-for` | 已废弃。端点选择也是按资源的治理策略，写进 `govern.rules[N]`（键为 `balancer` / `outlier-threshold` / `outlier-suspend-for`）。 |
| `govern.rules[N]` 只写一个字段想“微调” | Rule 是整体替换 default，漏写字段=禁用该能力。要保留的 default 字段得抄进 Rule。 |
| 用 `govern.override.<label>` 旧写法 | 已改为 `govern.rules[N].resources=<label>`。label 放值里，别再当 key（冒号会废掉 YAML）。 |
| 同时开着 govern 的 `max-retries` 和 client 自己的 retry 旋钮 | **重试次数是相乘的**。客户端级的重试留在客户端（它们的语义不同，见下），所以两边都开 = 双重退避。二选一。 |
| 以为 govern 的 `timeout` 能替代 client 的 `read-timeout` 之类 | 两者管的层次不同：`timeout` 是**单次调用**的整体预算（executor 层），client 的 dial/read/write timeout 是**传输层**的。client 的传输超时留在 client（构造期参数，改不了不用重启的假象）。 |
| 不知道资源 label 是什么 | 配 `service-name` 让 label 稳定可读；查设计说明 §6 表。 |
| 多 starter 项目写 `govern.fault.enabled=true` 以为只烧一个 | fault 是全进程共享开关，会烧所有 starter。用 `govern.fault.rules[].resources` 定向。 |
| 没 import starter-governance | 门面未生效，resilience 完全旁路，不报错但也不生效。 |
| 配了 `govern.*` 但忘了 `govern.source.file.path`（或其它 source） | 治理 disabled——没有 source 就没有规则来源。 |
| 改了规则文件没生效 | 确认 file source 在盯它（`govern.source.file.path` 指向的目录未变）；远程 source 确认 push 成功。规则文档本身热重载。 |

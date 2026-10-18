# cloud/runtime/controlplane/governance — 控制面

`cloud/runtime/controlplane/governance` 是 go-spring 的**控制面**。它只做三件事——**[送达契约 `Source`](source.go) + [期望态 schema `Config`](config.go) + [分发 `Center`](center.go)**——外加[一个规则文档解析入口 `Parse`](rules.go),除此之外不含任何策略实现。它持有进程内唯一一份可热更的 `Config` 文档，经 `Source` 接收下发，再一次性 fan-out 给各**域**的 authority。

被它治理的各**域**都位于 [`runtime/dataplane/`](../../dataplane/) 下：

| 域 | 角色 | 被中心分发到 |
|------|------|------|
| [resilience](../../dataplane/outbound/resilience/) | **出站原语** | `*resilience.Manager` —— 熔断 / 限流 / 重试 / 退避 / `Executor` 接缝 |
| [admission](../../dataplane/inbound/admission/) | **入站原语** | `*admission.Manager` —— 每实例的 `Guard`，限流 / 隔板 / 熔断 / 放火 |
| [fault](../../dataplane/kernel/fault/) | **混沌面** | 中立内核 + 两个方向各自的管理器 —— 出站借用 `Executor` 接缝、入站借用中间件 |
| [loadbalance](../../dataplane/kernel/loadbalance/) | **选点域** | `*loadbalance.Manager` —— 端点选择策略 |
| [discovery](../../dataplane/kernel/discovery/) | **不参与** | 命名后端目录；**不出现在规则文档里**，控制面既不持有也不裁决——它由 `runtime.Runtime` 持有并交给调用方 |

目录内另有两个**中立器官**：[breaker](../../dataplane/kernel/breaker/)（熔断状态机）与 [ratelimit](../../dataplane/kernel/ratelimit/)（限流词汇、本地预算、`ClusterLimiter` 接缝与计数算法）。它们不接收中心推送，由 `admission` / `resilience` 各自把自己的策略映射进去使用。

[label](../../dataplane/kernel/label/) 是**服务 label 规约**：两个方向与所有协议适配器都用 `label.ServiceLabel` 拼出同一个 `prefix:name`，治理规则才能按这个字符串同时命中 client executor 与入站 admission。不要在 starter 里重写拼接逻辑。

client 只注入 `*runtime.Runtime` 一个 bean，从它取需要的 authority（`ResilienceClient()` / `FaultClient()` / `FaultServer()` / `Loadbalance()` / `Discovery()`），不调任何包级函数，也从不接触本包。`Center` 是**内部的控制面实现**：它持有文档、把各域策略推给 authority，但不向 starter 暴露任何 authority。discovery 更彻底——它不接收任何规则，所以控制面不持有它，由 `runtime.Runtime` 直接持有并交给调用方。

> **概念**：控制面（本包）与域（resilience / fault / loadbalance …）是**两种东西**。控制面回答"期望态是什么、怎么送到运行时"；域回答"拿到策略后具体怎么做"。`Config` 文档是各域 policy 类型的**聚合体**（`resilience.Policy`、`loadbalance.Selection`、`fault.Config`…），所以本包必然 import 它们——这是控制面的固有职责（持有综合期望态），**不是从属关系**。

## 配置感知：Source 契约

治理中心自成体系：它消费自己的 `Source` 接口（[source.go](source.go)）——一个配置快照加一个变更订阅，两个方法。整条生效链（快照原子替换 → executor 在下次调用时按内容代际自己换代 → client 无感）只依赖这个契约，不感知配置从哪来。

```
治理文件 / 远程控制台 ──────┐
governance.Source bean ─────┴──→ center（构造期绑定，单一源）→ adopt → 三个模块 authority
                                                      res.Apply / lb.Apply / inj.Refresh*
```

文档格式由 `governance.Parse(name, data, format)`（本包的 `rules.go`，解析经 spring/conf 的 value-tag 驱动——这是 `cloud/` 里唯一 import `spring` 的地方）统一：任何后端送来的规则文档（文件字节 / HTTP body / 配置中心 value）都用同一套解析与键语义（`spring.governance.*` 键），规则文件跨后端逐字节可移植。解析成功但没有任何 `spring.governance.*` 键的文档一律报错（防截断静默关治理）。

source 在构造期绑定、终身不变。**没有内置默认源**——Source bean 是中心的**必传**构造参数，一个都没有的进程起不来；“治理关闭”由一份什么都不配的源文档表达——空的 `client`/`server`、零 `Default`、无规则，但仍要带一个 `spring.governance.*` 键（如 `spring.governance.client.default.timeout=1s`）；此时 `resilience.Manager.ExecutorFor` 返回透传 noop。治理配置不挂 gs 的通用属性刷新管道。

自定义 Source 的三种接入方式：

```go
// ① 推送式（治理控制台 / 定时拉取 / 测试）：PushSource 开箱即用
src := governance.NewPushSource(governance.Config{})
ctr, _ := governance.NewCenter(resClient, resServer, lb, injClient, injServer, disc, src) // 或把 src 导出为 bean
// 每次上游事件：
src.Push(newCfg) // 保护策略、选点策略与 fault 演练配置一起热更

// ② bean 注入（Source 需要自己的依赖与生命周期时）——Export 不可省略，
//    否则接口注入找不到它，治理静默 disabled
gs.Provide(newConsoleSource).Export(gs.As[governance.Source]())

// ③ 直接实现 Source 接口（如监听配置中心专用 key）
type nacosRuleSource struct{ ... }
func (s *nacosRuleSource) Snapshot() governance.Config          { ... }
func (s *nacosRuleSource) Subscribe(cb func(governance.Config)) { ... }
```

设计要点：单源、终身绑定、不内置 merge（`Rules` 是整体替换语义，合并策略属于组合 Source 实现的职责）；换源即重启。普通业务配置（如 `spring.dubbo.consumer`）与治理规则的动态化是两套机制：前者是启动配置，后者走 Source 契约。

开箱即用的自建链路适配器已覆盖多种后端：

| 后端 | 谁注册 | 一行接线 |
|---|---|---|
| 独立规则文件（fsnotify） | **本包内置** | `spring.governance.source.file.path=...` |
| 治理控制台 / 规则 API（轮询拉取） | **本包内置** | `spring.governance.source.http.url=...` |
| Nacos 直连（专用 dataId，ListenConfig 推送） | starter-governance-nacos | `spring.governance.source.nacos.server=...` + `data-id=...` |
| etcd 直连（专用 key，Watch 推送） | starter-governance-etcd | `spring.governance.source.etcd.endpoint=...` + `key=...` |

## 控制面与数据面的分层

`resilience` / `admission` / `fault` 等是**数据面**（策略的实现），本包是**控制面**（策略的分发）。两者不是父子，而是 `cloud/runtime/` 下的两个平面：

- `runtime/controlplane/governance`：期望态 schema、送达契约与分发；
- `runtime/dataplane/*`：请求路径上的执行、选点、发现、插桩与共享器官。

依赖方向仍是单向的：

```
controlplane/governance ──→ dataplane/{outbound/{resilience,chain,observe,fault}, inbound/{admission,observe,fault}, kernel/{fault,loadbalance,discovery,breaker,ratelimit,traffic,label}}
dataplane/kernel/fault           ──→ dataplane/kernel/traffic
dataplane/outbound/resilience    ──→ dataplane/kernel/{breaker,ratelimit}, dataplane/outbound/chain
```

控制面**必须** import 各域（因为它持有它们 policy 类型的聚合体 `Config`），但这是"综合期望态"的固有职责，不代表域从属于它。数据面则不得 import 控制面根包——这是两个平面之间唯一可能造环的方向。

## 不被本控制面治理的包

控制面只 fan-out 到 **resilience / fault / loadbalance** 三域。下列包同处运行时、但不受中心治理：

- `cloud/runtime/dataplane/kernel/traffic` — 压测/灰度流量标识的**数据面**接缝（叶子，只依赖 `cloud/propagate`）。无可下发的期望态（`Binding` 是构造期注入的静态值），控制面从不推它；但 `fault` 的 `scope` 门控依赖它的压测标记。
- `cloud/runtime/dataplane/kernel/discovery` — 服务注册与解析（**自注册型**期望态，另一套机制），与 `cloud/runtime/dataplane/kernel/loadbalance` 配对消费。
- `cloud/actuator`、`cloud/mesh`、`cloud/security` — 运维/网格/TLS，留在 `cloud/` 顶层，独立关注点。
- `cloud/experimental/loadtest` — 测试工具；`cloud/experimental/transaction` — 分布式事务（仍在孵化）。
- resilience 的插桩在 `cloud/runtime/dataplane/outbound/resilience/observe.go` 的 `WrapClientExecutor`，由
  `Manager.backing` 在 resolve 时应用到尚未发布到缓存的 executor 上，client 不直接调用。

### cloud/runtime/dataplane/kernel/traffic：域外数据面叶子

`cloud/runtime/dataplane/kernel/traffic` 只依赖 `cloud/propagate`，是数据面叶子；控制面从不向它下发期望态（`Binding` 是构造期注入的静态值），但 `fault` 的 `scope` 门控依赖它的压测标记。它自身的边界规则：

- `IsLoadTest` 既不是裸 var、也无包级门面：它是 `traffic.Propagator` 接口的方法，消费者（业务代码、fault、otel processor、各 starter）各自注入一个 `traffic.Propagator` 再调 `prop.IsLoadTest(ctx)`；包内零包级状态；无人 provide 时 nil 回落到 `NewDefaultPropagator()`。（2026-09-29）
- 接口 4 方法：`IsLoadTest`/`WithLoadTest`/`Extract(ctx, propagate.Carrier)`/`Inject(ctx, propagate.Carrier)`；`Propagate(p, parent, child)` 是包级纯函数。消费者模式 = 构造器尾参 `prop traffic.Propagator` + nil 回落 + `gs.TagArg("?")`，无处默认 provide。
- `Binding` 4 字段：`Key` / `Value` / `Bind` / `Bound`（`WireValue` 已改名 `Value`）；EqualFold 等值语义；构造期 validate 已删，`NewDefaultPropagator(binding Binding) (DefaultPropagator, error)` 恒返回 nil error（签名保留 error 备将来）。别加无参/不返回 error 的第二入口。
- `Extract` 扫 `Keys()` EqualFold 匹配，命中即 `WithLoadTest` 并直接 return；nil carrier 兜底；`Inject` 单写 `Key`。读侧故意比写侧宽：`Accept` 认 `"1"`/`"true"`/`"on"`/`"yes"`/`"t"`（入口常是 k6/PTS/网关）。
- 标识常量是**未导出**的 `canonicalKey = "x-loadtest"` 与 `canonicalValue = "1"`；HTTP header 走 `Header.Get/Set`（textproto 规范化），gRPC metadata 才用裸 Carrier。`canonical` 子包已删，全部并入 traffic。
- 别翻案：别恢复访问器缝、双 key 字段、Accept 谓词、包级门面；别给 traffic 加具体协议类型参数（HTTP 也不行，会破坏「接口对称覆盖全部协议」）。
- `propagate.Carrier` 是接口（`Keys`/`Values`/`Set`），三个通用 map 适配器 `MultiMap`/`StringMap`/`Header` 留在 `cloud/propagate`；协议特化解码适配器放回各 starter 私有（如 dubbo `attachmentCarrier`、rabbitmq `tableCarrier`）。
- 全链路：`loadtest.Run` 默认给 Op ctx 打标（source=`loadtest.Run`）；httpx transport 自动注入出站 header；gin `LoadTest` 中间件 + grpc `LoadTestUnaryInterceptor` / `LoadTestStreamInterceptor` 入站解析（默认开、装最外层/链首）。出站注入=httpx；入站解析=gin/echo/hertz + grpc + trpc + dubbo；MQ 双向=kafka/kafka-sarama/rabbitmq/nats/pulsar。
- redis/gorm 是数据存储客户端，压测标记走 ctx 消费侧（影子 key/表由用户加层），不注入 header。载体受限未做（同 trace 限制）：kitex、thrift、mqtt。
- 兜底点 error 落地：能返回 error 的装配处/叶子构造函数（`httpx.NewTransport`、luohua.apply）传播；返回不了的约 20 处统一 `prop, _ = traffic.NewDefaultPropagator(traffic.DefaultBinding())` + 注释 "DefaultBinding is complete, so this cannot fail"。

## 为什么定时任务（cloud/scheduling）刻意不接治理（拍板：2026-09-15）

逐项审过限流/重试/熔断对定时任务的价值，结论是**不接 `resilience.Manager.ExecutorFor`，也不给
scheduling 服务标签**：

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

治理规则使用 `spring.governance.*` 键命名空间（如 `spring.governance.client.default.*`、`spring.governance.client.rules`），与 Go 包名 `governance` 独立。规则是**独立文档**，经 Source 契约进入中心，不写进 `app.properties`——本地文件用 `spring.governance.source.file.path` 引导（file / http 两个源就内置在本包，见 [SOURCE_USAGE_CN.md](SOURCE_USAGE_CN.md)）。

本包**自己带一层 gs 接线**:[`starter.go`](starter.go) 注册中心 bean——**中心 bean 的构造就是启动**:`NewCenter` 在构造期分发初始文档、文档不可用则构造失败,`Close` 是 Destroy 钩子,并导出为 `gs.Rooter` 保证无人注入时也会被实例化。三个 authority bean **不在这里** —— 它们由**管这个类型的那个包**注册(`cloud/runtime/dataplane/outbound/resilience` / `cloud/runtime/dataplane/kernel/loadbalance` / `cloud/runtime/dataplane/{outbound,inbound}/fault`),所以注入它们的 client 只要 import 了那个包就已经拿到 bean,和治理无关。应用侧:

```properties
# 无需任何 import:file/http 两个源由本包自己注册,配上引导键即武装
spring.governance.source.file.path=conf/governance.properties
```

接入治理能力(拿 authority bean)只需链接本包;但**中心要求一个 Source bean**(必传构造参数)——所以链接了本包却没有源的进程会在构造中心时启动失败。file/http 是内置的(配上 key 即可);nacos/etcd 那两家要 import 各自的 starter。

非 gs 运行时自己持有中心：`governance.NewCenter(resClient, resServer, lb, injClient, injServer, disc, src)`——`src` **必传且不许为 nil**（构造期绑定，初始文档一律取自 `src.Snapshot()`；只想给一份静态配置就传 `governance.NewPushSource(cfg)`）；各 authority 由调用方自建（测试就是这么注入的）。构造即武装，无需再调任何启动方法。

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
          规则来源（file / http / 控制台 / bean 注入的 Source）
                    │  governance.Source 契约
                    ▼
        governance.Center.adopt(cfg)  ← 一次刷新，分发三个模块 authority
                    │
     ┌──────────────┼──────────────────┐
     ▼              ▼                  ▼
resilience.Manager/ServerManager  loadbalance.Manager  fault.ClientManager/ServerManager
 label → Executor    label → Selection    （进程唯一一份配置）
     │              │                  │
 客户端注入后       客户端注入后         客户端/服务端注入后
 mgr.ExecutorFor()  loadbalance.NewChain(...) clientfault.ClientManager.WrapClientExecutor / Apply
```

- **中心**：`governance.Center`，持有唯一的活跃 `Source` 与 `Config` 快照。它只做一件事——把配置文档按 label 解析成两半，分发给下面三个 authority。它**不持有**执行器缓存、订阅表、驱动目录。
- **模块 authority**：每个都由**管这个类型的那个包**注册为 bean（`cloud/runtime/dataplane/outbound/resilience` / `cloud/runtime/dataplane/kernel/loadbalance` / `cloud/runtime/dataplane/outbound/fault` / `cloud/runtime/dataplane/inbound/fault`），各自持有本领域的运行时状态与 fan-out：`resilience.Manager`（label → Executor 缓存 + 订阅表/路由解析）、`loadbalance.Manager`（label → 池绑定表）、`clientfault.ClientManager` / `serverfault.ServerManager`（两个独立注入器）。
- **分发**：配置变化时中心重算，**只在变化时**通知各 authority 的订阅者。选择性扇出的比对现在按半进行——保护半边用 `reflect.DeepEqual`（`Policy` 全标量，DeepEqual 精确），选择半边用 `==`（`Selection` 全字段可比）。

### 2.1 感知层：Source 契约

治理规则的**生效链**（快照原子替换 → 各 executor 在下次调用时按代际自己换代）从第一天起就是治理中心自己的设计；感知层直接消费治理自己的 `Source` 接口（[source.go](source.go)）——`Snapshot() Config` + `Subscribe(cb)`，两个方法。对照业界（Sentinel-Golang 的 ext/datasource、dubbo-go 的 DynamicConfiguration）：治理规则的模型与生效链自建、传输层做成可插拔适配，是主流共识；Spring Cloud 把治理深耦合进通用刷新机制（@RefreshScope）恰是被验证的弯路。

治理配置**不挂 gs 的通用属性刷新管道**：规则是它自己的一份文档（本地文件、远程控制台、配置中心），经 Source 契约进入中心，改一条规则只刷新治理，不触发全应用属性重绑。

- **来源**：bean 注入（必须 `Export(gs.As[governance.Source]())`，否则接口注入找不到它、启动直接失败在中心的必传参数上）——容器把源**作为 `Center` 的构造参数**交进来，构造期即绑定、终身不变。
- **没有内置默认源，但源是必传的**：容器必须有一个 Source bean（否则启动失败）；「治理关闭」由一份什么都不配的源文档表达（空 `client`/`server`、零 `Default`、无规则，仍带一个 `spring.governance.*` 键；每个 executor 都是透传）。
- **内置两个源适配器**：**file** 与 **http** 由本包自己注册（[source_file.go](source_file.go) / [source_http.go](source_http.go)，见 [starter.go](starter.go) 的 init），链接本包就有，无需额外 import。两者都是条件单例（`OnProperty` 前缀匹配），导出为 `Source` 交给中心构造参数；文档格式与规则文件同一套（`governance.Parse`，properties/yaml/json/toml，按扩展名或显式 `format` 判定）。

  | 键 | 说明 |
  |---|---|
  | `spring.governance.source.file.path` | 要监听的规则文件（fsnotify；监听**父目录**，所以原子重命名的更新不会漏） |
  | `spring.governance.source.http.url` | 控制台/规则 API 的地址（构造期拉一次） |
  | `spring.governance.source.http.interval` | 轮询间隔，默认 5s |
  | `spring.governance.source.http.format` | 覆盖格式推断（`yaml`/`json`/`properties`/`toml`） |
  | `spring.governance.source.http.headers.*` | 随每次请求发送（如 `authorization=Bearer xxx`） |

  远端配置中心（nacos ListenConfig 推送 / etcd Watch 推送）不在本包，各自在 `starter-governance-nacos` / `starter-governance-etcd`。
  逐键行为、启动时序、排障表见 [SOURCE_USAGE_CN.md](SOURCE_USAGE_CN.md)（英文 [SOURCE_USAGE.md](SOURCE_USAGE.md)）。
- **单源替换、不内置 merge**：`Rules` 是整体替换语义（0 = disabled），合并策略属于组合 Source 实现的职责。
- **换源安全**：换源靠活跃源守卫（handle 指针比较，规避接口值 `==` 的 panic 风险），旧源回调自动失效。
- **边界**：普通业务配置（如 `spring.dubbo.consumer`）继续用 `gs.Dync` 字段绑定——数据性质决定刷新机制：实例列表是高频运行态（discovery 的 watch），治理规则是低频配置态（Source 快照替换），普通开关是散配置（dync 字段绑定）。

> **client 怎么拿到治理能力**：client starter **注入它需要的那个模块 bean**，不再有任何进程级 seam。要保护调用就注入 `*resilience.Manager` 调 `mgr.ExecutorFor(系统名, 服务label)`；要选点就注入 `*loadbalance.Manager` 调 `lbMgr.Chain(resolver, label)`；出站放火注入 `*clientfault.ClientManager`、入站放火注入 `*serverfault.ServerManager`。注入参数一律用 `gs.IndexArg(n, gs.TagArg("?"))` 标成**可空**——链接了 `cloud/runtime/controlplane/governance` 就有这个 bean，没链接就是 nil（各 authority 把 nil 当作"未武装"，即透传），所以"治理没装"不会变成"应用起不来"。

> `resilience.Manager.ExecutorFor` 返回的 executor 每次 Execute 时按 label 懒解析（进程内一个 label 恰好一个 executor），与"中心何时武装"无关；resolve 时顺带把 observe 层应用到**尚未发布**的 executor 上，所以 client 拿到的是组装好的 executor，不再自己调 `resilience.WrapClientExecutor`。`loadbalance.NewChain` 则**不是**懒的——但它在中心武装前也会完成注册（先建出基础代、武装后自动重建），所以构造期建链同样安全。

## 3. 为什么是 per-label 订阅，而不是一个全局回调

因为不同服务可以有**不同的策略**（通过 `Rules` 列表按 label 匹配）。redis 可能配了专属 ClientRule，gorm 走 default。如果用一个全局回调，任何一次配置改动都会把所有服务的 executor 都重建一遍。

per-label 订阅配合"上次交付值"（`subscriber.last`）做**选择性扇出**：

- 只改了 redis 命中的那条 ClientRule 时，gorm / mongo 等算出来的策略与上次相同 → 被跳过，不触发无谓的 executor 重建。
- 这要求能判断"是否真的变了"。保护半边 `resilience.Policy` 全标量（分类走 Retryable 接口而非函数字段），`reflect.DeepEqual` 精确；选择半边 `loadbalance.Selection` 全字段可比，直接用 `==`。

## 4. 核心不变量

| 不变量 | 如何保证 | 代码位置 |
|---|---|---|
| 热路径无锁 | `cfg atomic.Pointer[Config]`，查询只做一次原子 load | `Center.cfg` / `resilience.Manager.backing` |
| 选择性分发 | 每个订阅者记 `last`，Apply 时比对，未变不回调 | `resilience.Manager.Apply` / `loadbalance.Manager.Apply` |
| 回调在锁外执行 | 锁内只收集 `todo`，锁外逐个调用；回调里再调 Manager 不会自死锁 | 两个 Manager 的 `Apply` |
| Disabled 即透传 | 中心未武装时两半都解析成零值，executor 透明直连、池保持原策略 | `clientSide.buildLocked` / `loadbalance.Manager.selectionForLocked` |
| 一个 label 一个 executor | `Manager`/`ServerManager` 内按 label memoize，共享熔断/限流状态 | `resilience.Manager.backing` |
| 没有进程级可变全局 | 三个 authority 与中心都是 bean；`cloud` 侧零包级槽位 | 见下 §5 |
| 不链接 `cloud/runtime/controlplane/governance` 也是 no-op | 注入参数可空，authority 把 nil 当未武装 | 各 starter 的 `gs.IndexArg(n, gs.TagArg("?"))` |

## 5. 公共 API

```go
// —— 配置文档（cloud/runtime/controlplane/governance）——
type Config struct {
    Driver  string        // 韧性后端："default" / "sentinel"，全进程统一（一个 Driver 两个方法）
    Client  ClientConfig  // 出站半边：spring.governance.client.*
    Server  ServerConfig  // 入站半边：spring.governance.server.*
}

type ClientConfig struct {
    Default ClientDefaultPolicy    // 出站兜底：没有 ClientRule 命中的服务都用这份
    Rules   []ClientRule           // 出站逐服务列表；同一 label 重复即拒绝整个文档
    Fault   fault.Config     // 出站放火：spring.governance.client.fault.*
}

type ServerConfig struct {
    Default resilience.ServerPolicy // 入站兜底（限流/并发/熔断/处理预算）
    Rules   []ServerRule      // 入站逐路由列表；同一 label 重复即拒绝
    Fault   fault.Config         // 入站放火：spring.governance.server.fault.*
}

type ClientDefaultPolicy struct {
    resilience.Policy     // 保护半边：timeout / retry / error-threshold / …
    loadbalance.Selection // 选择半边：balancer / outlier-threshold / outlier-suspend-for
}

type ClientRule struct {
    Service string          // 该 ClientRule 匹配的服务 label（单数，一条 ClientRule 一个服务）；空=不匹配
    resilience.Policy     // 同上两半，直接内嵌，字段绑在 rules[N].*
    loadbalance.Selection
}

type ServerRule struct {
    Service string       // 该 ServerRule 匹配的入站 label；空=不匹配
    resilience.ServerPolicy // 入站模型：无 retry / 无 fallback / 无选择（字段上就没有）
}

// Center 只做文档与分发；接线方（本包自己的 starter.go）驱动它
func NewCenter(resClient *resilience.Manager, resServer *resilience.ServerManager,
    lb *loadbalance.Manager, injClient *clientfault.ClientManager, injServer *serverfault.ServerManager,
    disc *discovery.Manager, src Source) *Center   // src 必传且不许为 nil：容器须有 Source bean
func (c *Center) Validate(cfg Config) error                 // dry-run：dispatch 的 phase 1，只读，不碰任何状态
func (c *Center) Close() error                             // 关闭绑定的 source（可选实现时）

// —— 三个模块 authority（各自包内）——
// 驱动目录在**构造期**给定：容器把收集到的 Driver bean 作为构造参数传入；
// 策略则走包级注册表（见 cloud/runtime/dataplane/kernel/loadbalance 的 Register）
// resilience：两个方向各一个 authority，共享同一个 Driver bean（启动期选定）
func NewManager(driver Driver) *Manager
func NewServerManager(driver Driver) *ServerManager
func (m *resilience.Manager) Apply(Settings) error / Close() error
func (m *resilience.Manager) ExecutorFor(system, service string) Executor   // 出站
func (m *resilience.Manager) PolicyFor(label string) Policy
func (m *resilience.Manager) Subscribe(label string, cb func(Policy)) Subscription
func (m *resilience.Manager) Resolve(op chain.Operation) CallPolicy / ResolveLabel(label) CallPolicy
func (m *resilience.ServerManager) Apply(ServerSettings) error / Close() error
func (m *resilience.ServerManager) ServerGuardFor(system, service string) *ServerGuard   // 入站
func (m *resilience.ServerManager) ServerPolicyFor(label string) ServerPolicy / ResolveRoute(label) RoutePolicy
func NewManager() *Manager                           // loadbalance；策略从包级注册表解析
func loadbalance.NewChain(src discovery.Resolver, label string, opts ...ChainOption) *Chain  // standalone，不注册
func (m *loadbalance.Manager) Chain(src discovery.Resolver, label string, opts ...ChainOption) *Chain
func (m *loadbalance.Manager) Chain(src discovery.Resolver, label string, opts ...ChainOption) *Chain
func (m *loadbalance.Manager) Close() error
func (m *loadbalance.Manager) Apply(Settings)
func loadbalance.Validate(Settings) error                 // dry-run：每个非空 Balancer 名走同一个 New
func (m *clientfault.ClientManager) Config() / Refresh(Config) / WrapClientExecutor(inner Executor) Executor  // 出站放火（建壳 + 注册）
func (m *serverfault.ServerManager) Config() / Refresh(Config) / NewServer() *Server                        // 入站壳（接线期建一次）
func (s *fault.Server) Fire(ctx, t Target) (Decision, error)                  // 入站放火
```

**没有包级门面**。`cloud/runtime/controlplane/governance` 以前有一组 `Enabled/PolicyFor/Register/OnReady/…` 的包级函数与一个进程单例，现已全部删除：需要治理能力就注入对应的 bean，需要驱动中心就持有 `*Center`。这是"只有一套机制"的落点——不再有"全局槽位"与"IoC bean"并行。

**为什么保护与选择是两个类型**：`resilience.Policy` 现在是纯保护模型（限流/熔断/舱壁/重试/超时），端点选择的词汇（balancer / outlier）归 `loadbalance.Selection`。两者由**部署方在规则文档里组合**（`ClientRule` / `ClientDefaultPolicy` 同时内嵌两半，所以配置 key 与以前完全一样：`spring.governance.client.rules[0].timeout` 与 `spring.governance.client.rules[0].balancer` 都在 `rules[0]` 下）。这样每个模块只认自己的配置类型，`resilience` 不必知道什么是 balancer。

**为什么是 `Rules` 列表而不是 `map[label]`**：服务 label 用冒号分段（`gorm:mysql:orders-db`）。若 label 做 map key，冒号进到 YAML key 位置会让映射解析错乱（每个 key 都得加引号、漏一个就静默解析错）。改成列表后，label 退到 `service` 值的位置——冒号在值里，properties 不转义、YAML 不加引号，两种格式都干净。`PolicyFor` 遍历 Rules（每进程就几条，O(n) 可忽略）找首个 `Service` 等于 label 的，找不到回落 Default。

**关于 ClientRule 是"整体替换"而非"字段合并"**：因为 `resilience.Policy` 字段为 0 表示"禁用"，字段级合并无法区分"显式设为 0"和"未设置"。给某服务配 ClientRule，意味着你要一份与 Default 完全不同的、自包含的策略（要保留的 default 字段得抄进 ClientRule）。

## 6. 服务标签（label）约定

label 是 `resilience.Manager.ExecutorFor` / `PolicyFor` 与 `loadbalance.NewChain` 注册用的 key，决定一条策略归属哪个服务。所有 starter 通过 [`label.ServiceLabel`](../../dataplane/outbound/resilience/policy.go) 统一拼接：第一个非空 name 拼到 prefix 后，都没有则只用 prefix。

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

**给一个服务配置策略**，把上表中的 label 作为 `spring.governance.client.rules[N].service` 的值。具体写法见 [配置指南 §3](#3-多-starter-项目给不同服务配不同策略)。

**同一个 label 也管端点选择**：凡是走发现模式建了 `loadbalance.Chain` 的 client（`http` / `gateway` /
gorm 四方言 / `redigo` / `redis` / `mongodb`），都用上表里它自己那一行的 label 去 `lbMgr.Chain(resolver, label)`，
所以一条 ClientRule 同时决定这条服务的保护策略**和**选点策略（`balancer` / `outlier-threshold` /
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
叠在上面的进程级默认）；要按服务隔离策略，注册一个自定义命名的策略（见 `loadbalance.Register`）——它按设计豁免
进程级覆盖。逐条口径见 [§3.1](#31-端点选择负载均衡策略--剔除)。

## 7. dubbo 的特殊适配

dubbo 有自己的 URL-param 治理模型（timeout / retries / loadbalance / cluster 等是 provider/consumer URL 上的参数），不直接走 resilience Executor 这条路。govern 对它的适配方式：

- **label**：应用级 `dubbo:<app>` + 每个 reference 的 `dubbo:<interface>:<version>:<group>`，由 `dubboServiceLabels` 产出。
- **桥接**：[`starter-dubbo/dync.go`](../../../../starter/starter-dubbo/dync.go) 的 `poll` 把注入的 `*resilience.Manager` 的 `PolicyFor(label)` 结果翻译成 dubbo 参数——`Policy.Timeout` 写成毫秒数的 `timeout`，`Policy.MaxRetries` 写成 `retries`（cluster-failover 级别，不是 resilience 层 retry）。
- **热更新**：对每个 dubbo label 调 `mgr.Subscribe(label, ...)`，回调里重新 `poll` 并 `RefreshOverrideRules` 推给 dubbo-go 的动态配置层。dubbo 的 poller 把 center 作为构造依赖注入，gs 把依赖方排在 center 之后接线，所以它首次 `poll` 时治理必然已 live。
- **dubbo 专属旋钮**（loadbalance / cluster / serialization）留在 dubbo 自己的配置段，不进 govern。

> Subscribe 的回调里会重入 `poll`，所以 dubbo 在锁**外**收集待注册 label、锁**内**去重登记，避免持锁跨 `Subscribe` 自死锁。这是回调在锁外执行这一不变量（§4）的一个具体应用。

## 8. 与 fault 注入的关系

fault（"放火"）已**收进治理中心**，和 resilience 共用同一个 `governance.Config`——也就是同一份规则文档。两个方向各嵌一份 `fault.Config`：`ClientConfig.Fault`（`value:"${fault:=}"`，绑成 `spring.governance.client.fault.*`）与 `ServerConfig.Fault`（绑成 `spring.governance.server.fault.*`）。`cloud/runtime/dataplane/outbound/fault` 与 `cloud/runtime/dataplane/inbound/fault` 各注册**一个独立 bean**——`*clientfault.ClientManager` 与 `*serverfault.ServerManager`（零配置即 inert）——center 同时持有两者，但**不把它们聚合成第三个类型**：每次 source push（`adopt`）在 `resilience.Manager.Apply` / `loadbalance.Manager.Apply` 之外，额外调 `injClient.Refresh(client.fault)` 与 `injServer.Refresh(server.fault)` 各推一侧，两侧 starter 也各自只注入本方向的 bean。

**为什么 fault 必须按方向拆**：一个方向的故障注入只应作用于**这一侧**的调用（出站或入站），而方向无法从一条规则本身推出来——同一个 label 只出现在出站不假，但一条空 service 的 catch-all 规则在哪一侧都能写。所以配置分成两个块，两个 manager 各自持有独立状态与创建 API；两边只共用中立内核（规则语法、注入序列、护栏模型）。

`fault.Config` 的**结构两侧同构**：它就是一个 `Rules []Rule` 列表（一条规则 = 一个实验，没有方向级的全局效果）。方向落在"**持有它的 manager 类型**"上——`Manager.WrapClientExecutor`（出站）/ `ServerManager.NewServer().Fire`（入站）各自绑定一侧，调用方无从选错边。

**三个 authority 现在是同一副形态：中心推规则数据，模块自己按 label 匹配。** fault 从一开始就是这样，resilience 与端点选择现在也对齐——`dispatch` 把文档翻成各模块自己的规则类型并整体推给对应 manager，中心不再替谁解析 label。区别只在规则数据长在哪：

- `resilience.Policy` / `ServerPolicy` **没有**内置的多服务定向能力，redis 和 gorm 是两个独立对象，所以 center 推一份 default 加一份 `Rules`，由 manager 自己做"属于你的那一个"的首条匹配（找不到回落 Default）。
- [`fault.Config`](../../dataplane/kernel/fault/config.go) 的多服务定向写进类型本身：`Rules []Rule` 每条有自己的 `Service / Rate / Latency / Error`，一份 Config 即可描述"redis 打 0.5 错误、gorm 加延迟、其余全量慢调用"。所以 fault 只推一份带 Rules 的 Config，由 injector 在分发时匹配。

starter 侧注入本方向的 fault bean 接入，零耦合 cloud/runtime/controlplane/governance：

- **client 侧**：`injClient.WrapClientExecutor(mgr.ExecutorFor(系统名, r))`。`mgr.ExecutorFor` 已自带治理与 observe 组装，`WrapClientExecutor` 只在最外层做放火；`injClient` 就是注入的可空（`gs.IndexArg(n, gs.TagArg("?"))`）`*clientfault.ClientManager`，nil 即透传。持有指针就够——shell 持有自己的内核、push 时原地换（`Manager.Refresh`）而不是被替换，所以构造期装配不会丢配置，不需要每次调用再解析。
- **server 侧**（gin/echo/hertz/grpc/trpc/dubbo）：中间件/拦截器在接线期捕获注入的 `*serverfault.ServerManager` 一次，接线期 `srv := injServer.NewServer()` 自注册，per-call 调 `srv.Fire(ctx, t)`，`t` 是适配器构造的 `fault.Target`——入口 + 真实路由/方法（`GET /users/{id}`、`info.FullMethod`），所以入站规则能只给一个接口放火。未挂火时透明直通，所以这些中间件**无条件安装**。

**两个方向的火互不污染**，包括护栏：`MaxAffected` / `MaxDuration` 的计数器是**按方向各算**的。所以"出站烧到熔断自愈"不会顺手关掉入站的演练，反之亦然——这正是拆方向要买的东西。

**入站放火（`Server.Fire`）验证的不是本进程的重试栈**（入站没有重试），而是：本服务自己的错误路径与错误响应、observe 归类、入站熔断器是否把 5xx 计为失败，以及上游客户端面对 503 / 慢响应时的重试与断路器行为。出站放火（`WrapClientExecutor`）才验证本进程的 retry / breaker / timeout / fallback。生态里的混沌工程同样两侧都做（Chaos Mesh 的 HTTPChaos `target: Request`、Istio 的 `SIDECAR_INBOUND` fault filter），所以 `server.fault` 不是可选装饰。

## 9. 家族接缝、注入与治理判据

### 9.1 家族结构与依赖方向

`cloud/` 下这些包**平级**，不再有伞包中间层（早期「收拢进 `cloud/runtime/controlplane/governance/`」的形态已废弃）：

- `cloud/runtime/controlplane/governance` —— 中心 / 规则文档 / 源适配器（本包）。
- `cloud/runtime/dataplane/outbound/resilience` —— 端点保护原语，**叶子**，不 import 家族其他成员。
- `cloud/runtime/dataplane/kernel/fault` —— 中立的内核（规则 / 文档 / 匹配 / 注入序列）。
- `cloud/runtime/dataplane/outbound/fault` · `cloud/runtime/dataplane/inbound/fault` —— 两个方向各自的混沌注入职权。
- `cloud/runtime/dataplane/outbound/chain` —— `Executor` 契约，**最底层**（拒绝哨兵也在这一层）。
- `cloud/runtime/dataplane/outbound/observe` · `cloud/runtime/dataplane/inbound/observe` —— 两个方向的插桩；span 属性载体在中立的 `cloud/observability/attrs`。
- `cloud/runtime/dataplane/kernel/loadbalance` —— 端点选择，与 discovery 成对。

依赖方向严格单向：`chain ← observability ← resilience`。配置根一律 `spring.governance.*`（旧 `${govern}.*` 已废弃）。

### 9.2 注入约定：Runtime 必填、authority 可空

- `*runtime.Runtime` 是治理的唯一注入点：所有 client/server starter 注入它，不再散装注入 `*resilience.Manager` / `*clientfault.ClientManager` / `*serverfault.ServerManager` / `*loadbalance.Manager` / `*discovery.Manager`。
- 注入 `*runtime.Runtime` 一律**按类型必填**，不写 `TagArg("?")` / `IndexArg`。Runtime 的 getter 对 nil receiver 安全（返回 nil authority），但容器里的 bean 本身恒在。
- 恒在性判据：`cloud/runtime` 根包 `init` 注册 Runtime bean，`Source` 是必填构造参数（缺了启动即失败；测试的 nullable 接线模式下 gs 会传 nil，bean 内归一化为空 `PushSource`），且 bean `Export(gs.As[gs.Rooter]())` 强制实例化——任何链接了这些 starter 的应用，启动成功时 Runtime 必然存在。
- 只有 gs 注入的构造函数/接线点收 `*runtime.Runtime`；内部与导出的单 authority 组合子（`buildServerPolicy(cfg,rt)` / `ServerPolicy(label,rt)` / `NewPickPool(...,lbMgr)`）保持收单 authority，由调用处喂 `rt.X()`。
- `runtime.New(resClient, lb, injClient, injServer, disc, source, opts...)`：组合根一次装配。`governance.NewCenter(resClient, admissionServer, lb, injClient, injServer, source)` 只接收策略 authority（没有 discovery），由 Runtime 调用；`source` 必传且不许为 nil（初始文档一律来自 `source.Snapshot()`，静态配置用 `NewPushSource(cfg)` 承载）。

### 9.3 driver 与出站/入站方向

- driver 留根一份：一个 `Manager` 造两侧 executor，`Driver` 接口两个方法（`NewExecutor` 出站 / `NewServerExecutor` 入站），`spring.resilience.driver=sentinel` 一句话选定全进程两侧后端；不再把 driver 下沉到 client。后端是**静态启动选择**（`spring.resilience.driver`，app 配置经 `gs.TagArg` 按 bean 名注入），不在治理文档、不热重载。（2026-09-29；2026-10-11 改为静态注入，去掉 driver 目录）
- 配置按方向拆 block：`spring.governance.client.{default,rules,fault}` 与 `spring.governance.server.{default,rules,fault}`；入站模型 `ServerPolicy` = `Policy` 减 retry 族与 max-duration。（2026-09-29，旧 `govern.default/rules/fault` 已不绑定任何东西）
- rules 不按方向拆：一条 rule 由 label 定位，label 自带方向（`gin::8080` 入站 / `redis:cache` 出站），一份足够；只有 fault 必须拆 block，因其有无 label 的全局旋钮（rate/latency/error/scope/护栏）。（2026-09-29）
- 「模型拆、引擎不拆」；出站与入站各有一个引擎（`defaultExecutor` 带重试 / `serverExecutor` 不带重试，`Timeout` 是整通预算），两者不 embed、不投影——`ServerPolicy.AsPolicy() Policy` **已删除**；`Refresh`/`Refreshable` 与 `PolicyResolverSetter` **已全部删除**，热更换成「manager 发布代际、executor 调用点拉取并整份重建状态」（`PolicySource`，见 resilience 的 README）；库内方向词是 `side`（旧泛型 `lane` 已废弃）。（2026-09-29 提出，2026-10-03 定稿；2026-10-16 改为拉取式代际）
- 命名：真有对偶的概念才加方向前缀且显式带 `Client`/`Server`——出站 `Policy` / `Executor` / `ClientRule` / `ClientDefaultPolicy` / `Manager.ExecutorFor` / `Manager.PolicyFor` / `Driver.NewExecutor` / `outbound/observe.WrapClientExecutor` / `clientfault.ClientManager.WrapClientExecutor` / 指标 `resilience.client.*` 对 `resilience.server.*`；没有对偶的不加（`Driver` / `loadbalance.Selection` / `ClusterLimiter` / `Fallback` / `ServiceLabel` / `Retryable` / fault 的 `Config`/`Rule`）。`admission` 只作概念保留（文档里的「入站准入」；代码里没有对应的导出名）。（2026-09-29）
- Driver 后端 starter 命名归 `governance-` 家族（`starter-resilience` → `starter-governance-sentinel`，对齐 `StarterGovernanceEtcd`/`StarterGovernanceNacos`）；选择键是静态的 `spring.resilience.driver=sentinel`（app 配置，非 per-client key）。容器外用法保留 `NewSentinelDriver()`。（2026-09-13）

### 9.4 Source 契约的 ctx 与 trace_id

- `governance.Source` = `Snapshot() Config` + `Subscribe(cb func(ctx context.Context, cfg Config))`：单 callback（Center 是唯一消费者），契约无 error、无 Close（Destroy 经 `interface{ Close() error }` 断言可选关闭）。ctx 是 2026-10-06 的破坏性加法：一次策略推送从「源发现变更」到「中心应用」共用一个 `trace_id`——两个内置源与两个 starter 在**每次变更**处起 root 铸 `trace_id` + 自身坐标，其周期行（poll/reload 失败）用循环 ctx（不铸）。
- 统一防护语义：启动坏文档 fail-fast；运行期坏推送**整份**保 last good + log；byte-equal/DeepEqual 去重不推。
- 所有源变更走 `Center.adopt(ctx, cfg)` 单一 sink。`dispatch` 是**两阶段**（对齐 `gs_dync` 的 onValid/onCommit）：phase 1 `Center.Validate` 只做检查、不碰任何状态（重复 label + driver 名 + balancer 名），phase 2 才 `c.cfg.Store` + `res.Apply(...)` + `lb.Apply(...)` + `injClient.Refresh(...)` / `injServer.Refresh(...)`——commit 阶段不可能失败，所以快照与三个权威**要么一起换、要么一个都不换**。此前 `Store` 排在 `res.Apply` 之前、且 Apply 的 error 被"捕获后继续"，一份 driver 名写错的推送会让快照 + lb + fault 都换新而 resilience 留在旧的。`Validate` 同时导出为 dry-run：推送方可以「先校验、后 push」。（2026-10-09）
- 预检查把"名字解析不了"统一成整份拒绝：balancer 名（`loadbalance.Validate`，走与 `Chain.build` 同一个 `New`，所以"未注册名"和"策略拒绝的参数袋"都拦得住）。driver 名不再在这里校验——后端是启动期按 bean 名注入的静态选择（注入不到即启动失败），不进文档。（2026-10-09；2026-10-11 driver 移出文档）
- 新 Source 后端接线套路：`OnProperty("spring.governance.source.<name>")` 条件单例 + `Export(gs.As[governance.Source]())`。
- 测试铁律：`gs_init.Beans()` 在 `testing.Testing()` 时克隆全局 bean 定义，服务装配测试**不能走 `gs.RunTest`**（生产不受影响）。

### 9.5 规则、label 与命名

- `governance.ClientRule` / `governance.ServerRule` 与 `fault.Rule` 字段单数，只挂一个 label，键为 `Service`（`${service:=}`）；fault 空 service 仍是 catch-all。（2026-09-28）
- `Center.dispatch` 的预检查阶段校验重复 label、driver 名、balancer 名，任一不过即**拒绝整个文档**（旧快照继续服务）；无 first-match 语义残留。重复 label 的判据见 `validateClientRules` / `validateServerRules`（`config.go`），只拒非空 label、client 与 server 两块各判各的。
- Rule 不是 `map[label]`：label 含冒号（`gorm:mysql:orders`），进 YAML/properties key 位会让映射解析错乱；label 退到 `Service` 值里就 dot/colon-safe。
- label 优先 discovery 的 service-name（治理目标是下游服务身份，配合服务发现），退回实例名；新 starter 一律 `label.ServiceLabel(前缀, service-name, ...回退)`。
- `resource` 一词在治理里一律改叫 `service`（文档键 + 运行时缝 + emit 属性键）：`ServiceLabel`、`Manager.ExecutorFor(system, service)`、`Driver.NewExecutor(service, p)`、`fault.evaluate`（未导出）、`resourceState`→`serviceState`、`Center.clientServiceFor`、httpx `Config.Service`、gorm `module.Spec` / `Options.Service`、metric/log/trace 键 `resource`→`service`、span `resilience.service`。`resource` 只留给无关语义（Go 资源 `no resources to release`、sentinel 自己的 resource name、AT 事务 branch、OTel `sdk/resource`、OAuth2 resource server、k8s `resourcelock`），别误改。（2026-09-28）

### 9.6 什么该收进治理（判据 P1–P4）

- 四条判据须同时满足：**P1 是策略不是身份**（值会因流量/运维变，不是 addr/dsn/uri/密码/db/tls）；**P2 可按 label 寻址**（客户端已算出 `http:user-svc` 这类 label）；**P3 组件能不重建对象就采纳新值**（否则只是「文档集中了、改完还得重启」，最易被忽略）；**P4 跨 starter 重复或与已有 govern 概念重叠**。（2026-09-12）
- 落点判据（第 5 条）：只有**保护 / 选择 / 放火**三类的键才进 `resilience.Policy` / `ServerPolicy`；往里塞连接池尺寸等实现细节会让 Policy 变杂物袋。
- 明确不收（避免反复议）：① 身份/凭据/寻址（addr/dsn/uri/user/password/db/tls）；② 决定 bean 存不存在的装配开关（`health.enabled` / `observe.enabled` / `discovery` / 选择用 `driver`）；③ 中间件开关（cors/gzip/secureHeaders/accessLog）；④ 服务端 read/write/idle timeout（`http.Server` 构造期参数）；⑤ 算法内部常数（`p2c.go` 的 ewmaBeta/ewmaTau/p2cFailurePenalty、consistenthash replicas）；⑥ lock TTL / session IdleTimeout / outbox 轮询退避（lock 已有自己的三层解析）。
- 客户端 dial/read/write timeout、server 构造期参数、池尺寸、gorm slow-threshold、入站准入上限、实例权重：**逐项裁决后不做**（P3 不成立 + 落点不成立）。
- grpc 客户端不再自带实例摘除：内置 balancer 的硬编码 5 次/30s 已删，改治理驱动、**初始禁用**（「不配置=不生效」）；要摘除就在治理文档的 client 默认策略里配 `outlier-threshold`，定向到具体 service 用 `spring.governance.client.rules[N]`。（旧 `govern.*` 键已废弃）
- `Selection.Balancer` / `OutlierThreshold` / `OutlierSuspendFor` 刻意不进 `IsZero`（由 Pool 消费而非 Executor 消费）；变化经 `loadbalance.Chain` **整条重建**生效；未知名策略忽略而非致命（Source 契约无错误通道）。
- 订阅 API 是 `resilience.Manager.Subscribe(label, cb) resilience.Subscription`（`Cancel()` 幂等、可与推送并发、零值安全）；摘掉最后一个订阅者删 map 条目。（`governance.Register` 这个入口不存在。）

---

# 治理中心配置指南

本文讲**怎么写治理配置**。治理配置是**它自己的一份文档**——本地规则文件（本模块的 file source 盯着它）、远程控制台，或配置中心——经 `governance.Source` 契约进入治理中心。它**不写进 `app.properties`**：改一条规则只刷新治理，不触发全应用的属性重绑。

设计原理（为什么是 Source 契约、per-label Subscribe 怎么分发）见下文[设计说明](#1-它解决什么问题)；服务标签（label）格式表见[设计说明 §6](#6-服务标签label约定)。

适用于一个项目里**同时用多个 starter**（redis + gorm + http-client + gin 入站……）的场景。

---

## 1. 前置：指定规则来源

治理不是自动生效的：**配上引导 key** 即可（file/http 源是内置的，不需要 import 任何东西）：

1. 在 `app.properties` 里用**一个引导 key** 告诉中心规则从哪来（本地文件为例）——file 源随之注册，容器把它交给治理中心并启动它。
2. 没有这个 key 就没有源；而中心的 Source 参数必传，所以链接了治理却没有源的进程会启动失败。

```properties
# app.properties —— 治理的"引导"配置，只有这一行
# 规则内容不在这里，在 conf/governance.properties
spring.governance.source.file.path=conf/governance.properties
```

```go
import (
    _ "go-spring.org/starter-redigo"     // 你的业务 starter(它传递链接了 cloud/runtime/controlplane/governance)
    // ... 其他 starter —— 治理没有需要额外 import 的东西
)
```

> **client starter 只注入 authority bean，不调任何包级函数。** 每个 client（redis/gorm/http/…）把自己需要的那一个（`*resilience.Manager` / `*loadbalance.Manager` / 本方向的 `*clientfault.ClientManager` 或 `*serverfault.ServerManager`）作为**可空**构造参数注入——`gs.IndexArg(n, gs.TagArg("?"))`，链接了 `cloud/runtime/controlplane/governance` 就有 bean、没有就是 nil——再调 `mgr.ExecutorFor(系统名, 服务label)` / `lbMgr.Chain(resolver, 服务label)` 拿到自己的能力。它不知道规则文档长什么样，也不知道是哪个 source 送来的：`governance.Source` 契约把治理核心和"规则从哪来"解耦，因此控制台推流、专用配置中心 listener、静态注入都能驱动它，而本地文件只是其中一种。

**容器里没有链接 `cloud/runtime/controlplane/governance` 时**：没有这些 bean，可空注入拿到 nil，各 authority 把 nil 当作"未武装"，`ExecutorFor` 返回透传的 noop executor，resilience 完全旁路（直连后端），不会报错。所以"没配治理"和"不能用 starter"是两回事。

**其它规则来源**：

- `spring.governance.source.http.*`（轮询远程控制台/规则 API，键见上面的内置源表）；
- config 中心的 source 适配器（nacos/etcd）各自独立成模块（`starter-governance-nacos`、`starter-governance-etcd`），内容同样是这份 `spring.governance.*` 文档；
- 代码里把 `PushSource` 等自建源作为 `NewCenter` 的构造参数静态注入或推流。

**例外：两个不走 `ExecutorFor` 的消费方**（都还是注入 authority，只是用法不同）：

- **starter-dubbo**：走自己的 URL-param 治理模型（timeout/retries 是 dubbo 参数，不走 resilience executor），所以注入 `*resilience.Manager` 直接读 `PolicyFor(label)` 的策略字段，用 `Subscribe` 跟热更。
- **starter-gateway**：它的链**按 route id 缓存、跨路由表重编译复用**，并在建链时用 `lbMgr.Chain(resolver, gatewayLabel(routeID))` **一次性**注册好——注册是单向的（见 `loadbalance.NewChain`），所以「每次重编译造新链」会攒下指向废弃链的注册。保护策略仍走 `mgr.ExecutorFor`（同其它 client），只有**端点选择**这一半用 `NewChain` 而不是 executor。

---

## 2. 最小可用：一份默认策略管全部

最常见用法——全进程所有服务共享同一份韧性策略。规则写在**规则文件**里（下面是 `properties`，YAML 键同构）：

```properties
# conf/governance.properties —— 治理规则，独立文档
# 注：后端不在本文件里选，而是 app 配置的静态键 spring.resilience.driver=default|sentinel

spring.governance.client.default.timeout=500ms
spring.governance.client.default.retry.max-retries=1
spring.governance.client.default.rate-limit=100       # ops/s，0 表示不限流
spring.governance.client.default.error-threshold=20   # 连续失败 20 次熔断
spring.governance.client.default.open-duration=5s     # 熔断持续 5s 后半开试探
```

配完这份文件（加上 §1 的 `spring.governance.source.file.path`），项目里的 redis、gorm、mongo、http-client……全部自动套用这套超时/重试/限流/熔断，且**热重载**——file source 盯着这个文件，改完不用重启（远程 source 同样的效果）。

> 规则文件里的键就是 `spring.governance.*` 命名空间。`spring.governance.client.default.*` 下可用字段是 `resilience.Policy` 的全部旋钮：`timeout` / `retry`(节点：`retry.max-retries` / `retry.initial-interval` / …，**默认关闭**，重试由调用方在 `chain.Operation.Retry` 上逐次声明，中心下发节点则整体覆盖) / `rate-limit` / `burst` / `rate-limit-max-wait` / `algorithm`(token-bucket|sliding-window) / `window` / `error-threshold` / `open-duration` / `breaker-strategy`(consecutive|error-rate) / `error-rate-threshold` / `min-requests` / `breaker-window`，以及端点选择类的 `balancer` / `balancer-params`（策略自己的参数，扁平子映射）/ `outlier-threshold` / `outlier-suspend-for`（见 §3.1）。字段含义见 [cloud/runtime/dataplane/outbound/resilience/policy.go](../../dataplane/outbound/resilience/policy.go)。

---

## 3. 多 starter 项目：给不同服务配不同策略

真实项目里 redis 和 gorm 的容忍度不一样。用 `spring.governance.client.rules[N]` 给特定服务单独配——**服务 label 写在 `service` 值里**（不是 key），所以冒号随便写、properties 不转义、YAML 不加引号：

```properties
# conf/governance.properties
# 默认策略：兜底，大部分服务用这个
spring.governance.client.default.timeout=1s
spring.governance.client.default.retry.max-retries=2

# redis 单独收紧：缓存快失败，少重试
spring.governance.client.rules[0].service=redigo:cache
spring.governance.client.rules[0].timeout=200ms
spring.governance.client.rules[0].retry.max-retries=0

# mysql 放宽：数据库慢查询多，超时给宽
spring.governance.client.rules[1].service=gorm:mysql:orders-db
spring.governance.client.rules[1].timeout=3s
spring.governance.client.rules[1].retry.max-retries=1

# http 下游服务按服务名
spring.governance.client.rules[2].service=http:user-svc
spring.governance.client.rules[2].timeout=800ms
```

YAML 规则文件里同样干净（冒号在值里，不是 key）：

```yaml
# conf/governance.yaml
spring:
  governance:
    driver: default
    client:
      default:
        timeout: 1s
        retry:
          max-retries: 2
    client:
      rules:
        - service: redigo:cache
          timeout: 200ms
          retry:
            max-retries: 0
        - service: gorm:mysql:orders-db
          timeout: 3s
        - service: http:user-svc
          timeout: 800ms
```

### 为什么是 `rules[N]` 而不是 `override.<label>`

早期版本曾用 `govern.override.<label>.<field>`，把服务 label 当 map key。但 label 用冒号分段（`gorm:mysql:orders-db`），冒号进到 YAML key 里会让映射解析错乱（`gorm:mysql:orders-db:` 被当成嵌套映射），每个 key 都得加引号、漏一个就静默解析错。所以改成列表形式：**label 退到 `service` 值的位置**，key 永远是 dot-safe / colon-safe 的数字索引，两种配置格式都自然。

### 3.1 端点选择（负载均衡策略 + 剔除）

除了"调用怎么被保护"，同一个服务的"调用发给谁"也在这份文档里：`balancer` 选策略，
`outlier-threshold` / `outlier-suspend-for` 决定一个反复失败的实例多久被摘出候选集。

```properties
# 发现模式下路由到 user-svc 的 client：改用最少连接，且连续失败 5 次摘除 10s
spring.governance.client.rules[3].service=http:user-svc
spring.governance.client.rules[3].balancer=least_conn
spring.governance.client.rules[3].outlier-threshold=5
spring.governance.client.rules[3].outlier-suspend-for=10s
```

- **`balancer`**：`round_robin`（默认） / `least_conn` / `consistent_hash` / `weighted` / `zone_aware`（包内还注册了 `random` / `p2c`），以及部署自己注册的策略（见 `loadbalance.Register`，如 `luohua`）。留空 = 保持该 client 的默认（round_robin）。写错策略名会被**忽略并沿用当前策略**，不影响调用——治理文档没有错误通道（"你推什么，你担保什么"）。**策略参数也在这份文档里配、随 push 热更**，放在 `balancer-params` 这个扁平子映射里，核心不解释键名，由策略自己读取：`replicas`（consistent_hash 虚拟节点数）、`zone-key` / `delegate`（zone_aware 的元数据键与内层策略）。写错参数名会在构造期报错并被忽略，不会静默丢参。
- **`outlier-threshold`**：与 `error-threshold` 是同一套语义（连续失败 + 半开试探），区别在作用对象——`error-threshold` 熔断的是**整个服务**，`outlier-threshold` 摘的是**单个实例**。0 表示不摘除。
- 两个旋钮都**原地生效**：改完 push，下一次请求就走新策略/新阈值，不用重启，也不会重建 transport。（策略自身的状态不跨切换保留——`least_conn` 的在途计数、`consistent_hash` 的哈希环、p2c 的延迟模型都会重来。）

**覆盖到的客户端**：所有走发现模式的 `loadbalance.Pool` 消费者——`http`（starter-http-client）、
`gateway`（每条路由）、`gorm` 全部方言、`redigo`、`redis`（go-redis）、`mongodb`。它们用
**与保护策略相同的服务标签**绑定（`label.ServiceLabel`，见 §6 表），所以一条 ClientRule 同时管住
一条服务的超时/重试/熔断和它的选点策略。

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

> 一句话自查：**这条 client 每次请求/每次建连会重新挑端点吗？** 会 → 它的服务标签就能配 `balancer`；
> 不会（只挑一次、或交给库内部挑） → 别指望 `spring.governance.client.rules[N].balancer` 对它生效。

**两个语义边界**（写规则前值得知道）：

- **`grpc` 客户端**的策略选择天然是 gRPC service config（`grpc.WithDefaultServiceConfig(LoadBalancingConfig(s))`）。
  治理的 `balancer` 是**叠在它上面的进程级默认**，标签固定为 `grpc:client`——所以给 `grpc:client` 配
  `balancer=` 会同时改掉**所有**内置 `gs_*` balancer 的策略（与它原本就管的摘除阈值行为一致）。不做
  进程级覆盖时，service config 的选择原样生效；以自定义名字注册的策略（见 `loadbalance.Register`）不受影响（它们存在
  的意义就是保留自己的策略）。
- **DB/缓存客户端的剔除粒度是"连接"而非"查询"**：gorm / redigo / go-redis / mongodb 的挑选发生在
  建连时，能喂给 `Tracker` 的成败信号只有 dial 结果本身。所以这些 client 上 `outlier-threshold` 摘的是
  **反复连不上**的实例；单条查询/命令的失败由它们各自的 resilience executor 管，不参与点数。

### 3.2 让某一条服务不上治理

客户端一律会挂 executor，**没有 per-service 的治理开关**——治理本就没有开关，进程级的后端选择是静态键 `spring.resilience.driver`（app 配置，不在治理文档里）。
想让某条服务事实上不受治理（裸调用），给它配一条**所有旋钮都为 0 的 ClientRule** 即可：ClientRule 是整体替换
default，全零 ClientRule 算出来的就是零 Policy，executor 退化为透传。

```properties
# kafka:<brokers> 这条 client 不上治理
spring.governance.client.rules[0].service=kafka:10.0.0.1:9092
# 字段全不写 = 全零 = 透传
```

> 早期版本里 kafka / mqtt / pulsar / rabbitmq 各有一个 per-instance `${governance:=true}` 开关，
> 已删除：它表达的就是"这条服务不上治理"，而这件事 ClientRule 已经能表达，且可热更。

### 几条规则

- **一条 ClientRule 只配一个服务**：`spring.governance.client.rules[0].service=redigo:cache`。不同服务永远各写一条——共享一条规则会在改策略时互相波及。
- **ClientRule 是整体替换，不是字段合并**：给 `redigo:cache` 配了 ClientRule，它就**完全不继承** `spring.governance.client.default`——漏写的字段按零值处理（零值=禁用该能力）。只想微调一个字段的话，把 default 里要保留的字段也抄进 ClientRule。
- **label 不应重复**：文档里同一个 label 出现两条 ClientRule 即整体拒绝（旧快照继续服务），没有 first-match 语义。
- **`service` 留空不匹配任何服务**——兜底请用 `spring.governance.client.default`，不要用空 service 的 ClientRule。

### 怎么知道某个服务的 label 是什么？

查[设计说明 §6](#6-服务标签label约定)的表。标签由 starter 用 `label.ServiceLabel(prefix, names...)` 拼接——取第一个非空的 name。所以 label 的取值取决于你配置里填的是 `service-name` 还是 `addr`：

- 你配了 `spring.redigo.instances.cache.service-name=cache-svc` → label 是 `redigo:cache-svc`
- 没配 service-name、只有 `spring.redigo.instances.cache.addr=10.0.0.1:6379` → label 是 `redigo:10.0.0.1:6379`

**建议**：给每个服务配一个稳定的 `service-name`，让 label 可读、不随地址漂移。

---

## 4. 入站流量的治理（gin / grpc / echo / hertz / trpc / http-server）

入站侧是**另一个方向**：策略作用在"处理一个进来的请求"上，读文档的 `server` 块，模型是 `resilience.ServerPolicy`——限流 / 并发上限 / 入站熔断 / 处理预算。它**没有**重试（handler 已产生副作用，不能重放）、**没有** fallback、**没有**端点选择；这些在 `ServerPolicy` 里根本没有对应字段，不是"设成 0 就禁用"。

label 是**本进程这侧入口**的身份（不是被调方）：监听地址（gin / echo / http-server / grpc 的 `grpc::{addr}`），thrift 由调用方自定。同一个 `service` 键在两侧指的都是"starter 交给管理器的那个身份串"，只是入站的值恰好是个地址：

```properties
# conf/governance.properties
spring.governance.server.default.timeout=2s      # 单个请求的处理预算
spring.governance.server.default.rate-limit=1000         # 入站限流 1000 QPS

# gin 监听 :8080 → label = gin::8080
spring.governance.server.rules[0].service=gin::8080
spring.governance.server.rules[0].rate-limit=500
```

出站与入站各自独立：同一份文档里 `client.default` 管所有出站调用、`server.default` 管所有入站请求，**不互相继承、不互相覆盖**。方向不会配错——label 只属于一侧（`gin::8080` 是入站，`redigo:cache` 是出站），而规则所在的那个块把归属再写明一次。

> `server` 块目前只有 `default` / `rules` / `fault`。将来入站专有的能力（如按系统负载剥离）加在这里；**不要**把 `client` 的 `Policy` 搬过来——它的 retry / fallback / 端点选择在入站没有意义。

### gateway 有两个 label（注意）

gateway 的一条路由对应**两个** label，因为它们作用在两个不同粒度的对象上：

| 作用对象 | label | 说明 |
|---|---|---|
| 保护策略（timeout / retries / 熔断 / 限流） | `gateway:<resilience-policy-name>` | 即 `spring.gateway.resilience.<name>` 的名字。多条路由引用同一 policy **共享** breaker 状态，所以按 policy 而非按路由。 |
| 端点选择（balancer / outlier-*） | `gateway:<route-id>` | 即 `spring.gateway.routes.<id>` 的 id。每个路由有自己的池（各 upstream 的候选集），所以按路由。 |

```properties
# 保护：所有引用 policy "strict" 的路由
spring.governance.client.rules[0].service=gateway:strict
spring.governance.client.rules[0].timeout=2s
spring.governance.client.rules[0].retry.max-retries=1

# 选择：只有路由 api/v1 的上游
spring.governance.client.rules[1].service=gateway:api/v1
spring.governance.client.rules[1].balancer=least_conn
```

---

## 5. dubbo 的治理

dubbo 走的是 URL-param 模型，govern 只覆盖它的 `timeout` 和 `retries`（cluster-failover 级别），label 是：

- 应用级：`dubbo:<app-name>`
- 每个 reference：`dubbo:<interface>:<version>:<group>`

```properties
# conf/governance.properties
spring.governance.client.rules[0].service=dubbo:com.example.UserService:1.0.0
spring.governance.client.rules[0].timeout=300ms
spring.governance.client.rules[0].retry.max-retries=2
```

dubbo 专属旋钮（loadbalance / cluster / serialization）不进 govern，留在 dubbo 自己的配置段。

---

## 6. fault（放火）——随 govern 集中化，同一个 source 驱动 ⚠️

fault 注入已**收进治理中心**，和 resilience 共用同一个 `governance.Config`——也就是同一份规则文档（参见[设计说明 §8](#8-与-fault-注入的关系)）。放火的 key 分方向：出站是 `spring.governance.client.fault.rules[]`，入站是 `spring.governance.server.fault.rules[]`：

```properties
# conf/governance.properties
# 出站：让 redis:cache 的调用以 50% 概率失败
spring.governance.client.fault.rules[0].service=redis:cache
spring.governance.client.fault.rules[0].rate=0.5
spring.governance.client.fault.rules[0].error=generic

# 入站：让 grpc 的某个方法以 50% 概率失败
spring.governance.server.fault.rules[0].service=grpc
spring.governance.server.fault.rules[0].action.exact=/demo.Service/Echo
spring.governance.server.fault.rules[0].rate=0.5
```

**一条规则 = 一个实验，没有全局默认。** 规则自带目标（service / action / match / scope）、效果（rate / latency / error）和剂量（max-affected / max-duration）；第一条匹配且 Scope 适用的规则生效，**没有匹配就不放火**。想烧整个方向，必须显式写一条 catch-all：

```properties
spring.governance.client.fault.rules[0].service=   # 空 service = catch-all
spring.governance.client.fault.rules[0].rate=0.5
```

这不是"少写一行"的取舍。方向级默认意味着两件事：忘了关就会烧到进程里所有依赖；而且"默认"并不逐字段继承——命中的规则整体替换效果，只写 `rate` 会丢掉默认的 `latency`。混沌工具（Chaos Mesh 的 selector + mode、ChaosBlade 的目标参数）都不提供默认实验，放火必须指名道姓。

**两个方向各烧各的。** 只配 `client.fault` 时入站零注入，反之亦然——这正是方向拆分要买的东西。两侧各持一个独立的 fault bean：client starter 注入 `*clientfault.ClientManager`、server starter 注入 `*serverfault.ServerManager`，center 对两个对象各调一次 `Refresh`，starter 自己不再绑 fault 配置、也不 import 治理中心。

### 定向

`rules[N]` 的定位字段：`service`（目标/入口标签）、`action`（接口，`action.exact` 精确 / `action.prefix` 前缀）、`match`（metadata 子集匹配，`rules[N].match.tenant=acme` 这类，可叠加多个键）、`scope`（`""` / `real` / `loadtest`）。四者都命中的第一条规则生效。

```properties
# 只给 redis 放火
spring.governance.client.fault.rules[0].service=redigo:cache
spring.governance.client.fault.rules[0].rate=0.5
spring.governance.client.fault.rules[0].error=timeout

# 只烧某个租户的 redis 调用
spring.governance.client.fault.rules[0].match.tenant=acme

# 只烧压测流量（真实流量不受影响）
spring.governance.client.fault.rules[0].scope=loadtest
```

```properties
# 入站只烧一个接口。service 是**入口标签**（gin / echo / hertz / grpc / trpc / dubbo），
# 具体是哪个接口写在 action 里：
spring.governance.server.fault.rules[0].service=grpc
spring.governance.server.fault.rules[0].action.exact=/demo.Service/Echo
spring.governance.server.fault.rules[0].rate=0.2

# HTTP 侧的 action 是 "METHOD /path"（路由模板，如 gin 的 c.FullPath()）：
spring.governance.server.fault.rules[0].service=gin
spring.governance.server.fault.rules[0].action.exact=GET /users/{id}
# 前缀匹配覆盖一族路径：action.prefix=GET /users
```

> `rules[N].service` 里的值要和该 starter 实际传给 injector 的标签对上：出站是目标（`redigo:cache`、`http:user-svc` 这类），入站是本进程的**入口家族**（`gin` / `echo` / `hertz` / `grpc` / `trpc` / `dubbo`）。注意这与 resilience 的 `server.rules[N].service`（入口**地址**标签，`gin:0.0.0.0:8080`）不是同一套取值——两边的规则块各自成篇。
>
> `rules[N].action` 出站取调用方声明的 `chain.Request.Action`（`insert` / `GET /users/{id}`），入站由各 starter 的适配器填真实路由/方法（gin `METHOD + c.FullPath()`、grpc `info.FullMethod`、trpc `CalleeMethod()`、dubbo `MethodName()`）——**入站也能只给一个接口放火**。

### fault 的安全保险

放火忘了关很危险，每条规则自带两个自愈上限：

```properties
spring.governance.client.fault.rules[0].max-duration=10m     # 该实验 10 分钟后自动停（从它第一次生效算）
spring.governance.client.fault.rules[0].max-affected=1000    # 该实验累计影响 1000 次调用后自动停
spring.governance.server.fault.rules[0].max-duration=10m     # 入站规则各算各的
```

**强烈建议**生产环境放火时必设其一，set fire and walk away 也不会烧到天荒地老。

> 注意：`max-duration`/`max-affected` 是**按规则**计数的（按规则身份 `service[index]`），一条规则自愈或耗尽额度不会影响另一条，也不会影响另一个方向。计数跨配置推送保留——一次无关的 push 不会把已经自愈的火重新点着。

### fault 与真实流量

用规则上的 `scope` 限定只烧压测流量、不碰真实请求（依赖 cloud/runtime/dataplane/kernel/traffic 的压测标记）：

```properties
spring.governance.client.fault.rules[0].scope=loadtest   # 只给带压测标记的流量放火；真实流量不受影响
                                                         # 反向：real = 只烧真实流量；空 = 全烧
```

### 运行时热更

规则文档随 file/http/config-center source push 到中心，**改配置 push 即可在不重启进程的情况下开关 fault**——starter 各持本方向注入的 manager（指针不变），中心 `Manager.Refresh` / `ServerManager.Refresh` 原地热更两侧。

---

## 7. 一份完整的多 starter 项目配置示例

一个同时用 gin（入站）+ redigo（缓存）+ gorm-mysql（DB）+ http-client（调下游）的项目。**业务配置**留 `app.properties`，**治理规则**独立一份：

```properties
# ============ conf/app.properties：业务 starter 配置 + 治理引导 ============
spring.gin.server.addr=:8080
spring.redigo.instances.cache.service-name=cache
spring.redigo.instances.cache.addr=10.0.0.1:6379
spring.gorm.mysql.instances.orders.dsn=orders:pwd@tcp(10.0.0.2:3306)/orders
spring.http-client.instances.user.service-name=user-svc
spring.http-client.instances.user.addr=10.0.0.3:8081

# 治理规则不在这里，只给它指个文件
spring.governance.source.file.path=conf/governance.properties
```

```properties
# ============ conf/governance.properties：治理规则，一处下发，处处生效 ============
# 注：后端由 app 配置的静态键 spring.resilience.driver=default|sentinel 选定，不在本文件

# 默认策略
spring.governance.client.default.timeout=1s
spring.governance.client.default.retry.max-retries=1
spring.governance.client.default.rate-limit=200
spring.governance.client.default.error-threshold=10
spring.governance.client.default.open-duration=10s

# redis 收紧：缓存要快
spring.governance.client.rules[0].service=redigo:cache
spring.governance.client.rules[0].timeout=100ms
spring.governance.client.rules[0].retry.max-retries=0

# DB 放宽：慢查询容忍
spring.governance.client.rules[1].service=gorm:mysql:orders
spring.governance.client.rules[1].timeout=3s
spring.governance.client.rules[1].retry.max-retries=2

# —— 入站（gin 监听 :8080）——
# 入站准入：限流 + 并发上限，保护自己不被上游压垮
spring.governance.server.default.rate-limit=1000
spring.governance.server.default.max-concurrent=200
# 公开接口单独收紧（label = gin::8080）
spring.governance.server.rules[0].service=gin::8080
spring.governance.server.rules[0].rate-limit=500

# fault：默认惰性，只要不配 rate 两侧都不注入
# 演练时配上 rate（出站只烧 redis；入站只烧一个 grpc 方法）：
# spring.governance.client.fault.scope=loadtest
# spring.governance.client.fault.rules[0].service=redigo:cache
# spring.governance.client.fault.rules[0].rate=0.3
# spring.governance.client.fault.rules[0].error=timeout
# spring.governance.client.fault.max-duration=5m
```

入口：

```go
import (
    _ "go-spring.org/starter-gin"
    _ "go-spring.org/starter-redigo"
    _ "go-spring.org/starter-gorm-mysql"
    _ "go-spring.org/starter-http-client"
)
```

---

## 8. 常见误区

| 误区 | 正解 |
|---|---|
| 把 `spring.governance.client.default.*` 写进 `app.properties` | 治理规则是独立文档，经 source 进入中心。`app.properties` 里只放 `spring.governance.source.*` 引导 key。 |
| 在每个 starter 自己的配置段写 `resilience.*` | 已废弃。resilience 现在只认治理规则文档，starter 段里的 resilience 配置不生效。 |
| 在 client 自己的配置段写 `balancer` / `suspend-threshold` / `suspend-for` | 已废弃。端点选择也是按服务的治理策略，写进 `spring.governance.client.rules[N]`（键为 `balancer` / `outlier-threshold` / `outlier-suspend-for`）。 |
| `spring.governance.client.rules[N]` 只写一个字段想"微调" | ClientRule 是整体替换 default，漏写字段=禁用该能力。要保留的 default 字段得抄进 ClientRule。 |
| 用 `govern.override.<label>` 旧写法 | 已改为 `spring.governance.client.rules[N].service=<label>`。label 放值里，别再当 key（冒号会废掉 YAML）。 |
| 同时开着 govern 的 `retry.max-retries` 和 client 自己的 retry 旋钮 | **重试次数是相乘的**。客户端级的重试留在客户端（它们的语义不同，见下），所以两边都开 = 双重退避。二选一。 |
| 以为 govern 的 `timeout` 能替代 client 的 `read-timeout` 之类 | 两者管的层次不同：`timeout` 是**单次尝试**的整体预算（executor 层），client 的 dial/read/write timeout 是**传输层**的。client 的传输超时留在 client（构造期参数，改不了不用重启的假象）。 |
| 给 `spring.governance.client.default` 或 `spring.governance.client.rules[N]` 写 `enabled=true` 想按服务开关 | **没有这个键**：`ClientDefaultPolicy` / `ClientRule` 只带策略旋钮；治理也不靠开关，命中规则即生效。绑定按字段走，多余键被静默忽略——不报错，也不生效。想让某条服务不上治理，给它配一条所有旋钮为 0 的 ClientRule（见 §3.2）。 |
| 不知道服务 label 是什么 | 配 `service-name` 让 label 稳定可读；查设计说明 §6 表。 |
| 多 starter 项目写一个全局 `spring.governance.client.fault.rate` 以为只烧一个 | 全局 rate 是全进程共享的，会烧所有 starter。用 `spring.governance.client.fault.rules[].service` 定向。 |
| 链接了治理但没配任何 `spring.governance.source.*` | 中心没有源，而 Source 参数必传——**启动失败**（`cannot find bean ... governance.Source`）。 |
| 只想关掉治理，不想配源 | 发布一份什么都不配、但仍带一个 `spring.governance.*` key（如 `spring.governance.client.default.timeout=1s`）的文档（源是必传的，"没有源"不等于"关闭"）。 |
| 改了规则文件没生效 | 确认 file source 在盯它（`spring.governance.source.file.path` 指向的目录未变）；远程 source 确认 push 成功。规则文档本身热重载。 |

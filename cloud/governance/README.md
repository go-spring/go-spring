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
- `cloud/loadtest` — 测试工具；`cloud/experimental/transaction` — 分布式事务（仍在孵化）。
- resilience 的插桩在本包自身（`resilience/observe.go` 的 `WrapExecutor`，由
  `ExecutorFor` 在 resolve 时应用到尚未发布的 executor 上，client 不直接调用）。

## 配置

治理规则使用 `govern.*` 键命名空间（如 `govern.enabled`、`govern.default.*`、`govern.rules`），与 Go 包名 `governance` 独立。规则是**独立文档**，经 Source 契约进入中心，不写进 `app.properties`——本地文件用 `govern.source.file.path` 引导（见 [starter-governance](../starter-governance/README.md)）。

本包**容器无关**（不 import spring/gs）：gs 接线（绑定注入的 Source、seam 注册、`GoLive`）在 **starter-governance** 的常驻 wiring bean 里。应用侧：

```go
import _ "go-spring.org/starter-governance" // 接线治理中心 + 装载规则来源
```

非 gs 运行时直接用门面：`governance.Arm(cfg)` 或 `governance.SetSource(src)` + `governance.GoLive()`。

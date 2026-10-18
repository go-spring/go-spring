# 任务：把 client 与 server 从 manager 到底层模型严格拆开

> **历史记录**：本文记录的是 2026-10-17 的中间态。此后结构又演进为
> `cloud/runtime.Runtime` 组合根 + `controlplane/registration` 写侧拆分；文中
> 关于 `governance.Center` 是唯一组合根、`NewProcessCenter`、`resilience.ServerManager`
> 的描述**已被取代**，以 `cloud/runtime` 的 README 与代码为准。
>
> 状态：**Step 1、Step 2、Step 3 均已完成（2026-10-17）**。
> Step 1 的基线是 `9df1117af`（fault 自注册改动已提交）。
>
> 本文记录目标、设计结论与执行结果，供上下文重置后继续。

## 1. 目标

1. **fault 先行**：把 `fault.Manager` 拆成 `ClientManager` 与 `ServerManager` 两个互相隔离的
   授权对象；共享的规则词汇与状态引擎下沉为内部实现，**不再有公开的“两方向合一”类型**。
2. **同一套拆分原则推广到其它层**：governance、observability（以及 resilience 中仍然合在
   一个 `Manager` 上的 client/server API）。每个域名下，client 侧与 server 侧是独立对象、
   独立类型、独立 bean；只有中立的词汇（Config/Rule/Target/Decision/状态引擎）可以共用。
3. 保留已经定下的状态模型（见 §3），不要退回“调用时向 manager 查询”。

## 2. 为什么必须拆（结论）

同构的部分：**规则、匹配、剂量、护栏、推送/刷新、状态生命周期**——两侧一模一样。

不同构的部分：**与宿主栈的绑定**，照搬就会撞墙：

| | client | server |
|---|---|---|
| 绑定对象 | `chain.Executor`（必须包在 retry/breaker 里） | 中间件 / 拦截器（没有 executor 概念） |
| Fire 时机 | retry loop 内**每个 attempt** | **每个请求**一次 |
| 记录写回 | 必须写 `chain.Recorder`（observe 据此打标） | 没有 Recorder，也不该有 |
| 失败语义 | 返回 error 给 retry/breaker | 适配器映射成 503 / gRPC status / RPCResult |
| 实例数量 | 每个 client 一个壳（N 个） | 通常一个进程一个入站壳 |
| 生命周期 | 随 client 创建/关闭 | 接线期捕获一次，通常跟进程 |
| 创建 API | `WrapClientExecutor(inner)` | `NewServer()` |

具体反例：
- server 套 client 的 executor/recorder：无意义；
- client 套 server 的“接线期捕获、每请求一次”：丢掉 per-attempt 注入点，放火不再流经
  重试/熔断，验证目的失败；
- 一个 `Manager` 同时暴露两套创建 API：client starter 握着 server 的工厂，反之亦然——
  这就是当前设计的“相互打架”。

## 3. 必须保留的不变量

1. **规则词汇不变**：`Config{Rules []Rule}`、`Rule{Service, MetadataMatch, ActionMatch,
   Scope, Rate, Latency, LatencyJitter, Error, MaxAffected, MaxDuration}`、`Target`、`Decision`。
2. **rules-only**：没有方向级默认效果；没有规则匹配就不放火；“全烧”必须显式 catch-all。
3. **状态模型**：稳定外壳 + 可换内核；**状态自己向 manager 注册**（构造时 `register(self)`），
   refresh 锁内对账一次并把新内核**推给所有已注册壳**；调用路径只做一次 `atomic.Load`，
   不查询 manager；壳 Close 时注销。
4. **对账语义**：按规则身份 `service[index]` 复用计数；新规则从零；消失的规则随旧内核丢弃。
5. **生命周期 = 文档里规则的出现/消失**；不新增 Start/Stop 命令式 API。
6. **server 不依赖** `chain.Executor` / `chain.Recorder` / retry 语义；**client 不采用**
   server 的 per-request 捕获模型。

## 4. 当前实现状态（Step 1 完成时的历史快照）

> Step 2 之后 `NewCenter` / `NewProcessCenter` 的签名已加入 `resClient` / `resServer`，
> 最新签名见 §10.7。下面的 fault 部分仍然准确，验证结果里的 echo 失败已在 Step 3 修复。

- `cloud/fault/core.go`：中立内核（未导出为主）——`Target`、`Decision`、`direction`、
  `coreHolder`、`core`、`ruleState`、`buildCore`/`ruleID`、`evaluate`/`decide`/`match`/
  `scopeApplies`。两个方向共享，但**没有合并的公开 manager 类型**。
- `cloud/fault/client_manager.go`：`ClientManager{dir direction, prop}`；
  `Refresh(Config)` / `Config()` / `NewClient()` / `WrapClientExecutor(inner)`。
- `cloud/fault/server_manager.go`：`ServerManager{dir direction, prop}`；
  `Refresh(Config)` / `Config()` / `NewServer()`。
- `cloud/fault/executor.go`：`Client` 壳 + `faultExecutor`（per-attempt 注入）。
- `cloud/fault/server.go`：`Server` 壳（per-request `Fire`）。
- `cloud/fault/starter.go`：两个独立 bean（`*ClientManager`、`*ServerManager`）。
- `cloud/runtime/controlplane/governance`：`Center` 持有 `injClient` / `injServer` 两个字段，暴露
  `FaultClient()` / `FaultServer()`；`NewCenter(res, lb, injClient, injServer, disc, src)`、
  `NewProcessCenter(res, injClient, injServer)`；`dispatch` 各调一次 `Refresh`。**没有**把两者
  聚合进第三个公开类型。
- 6 个服务端适配器在接线期 `injServer.NewServer()`；dubbo 只保留包级
  `atomic.Pointer[fault.Server]`（删掉了从未被读取的 `injector` 死变量）。
- 客户端：`httpx` 与 `Center.ExecutorFor` 使用 `FaultClient()`。

**验证**：`cloud` 全量 `go build ./... && go test ./...` 通过；受影响的 19 个 starter 模块
`go build ./...` 全通过，测试除以下两项外全通过：
- `starter-echo` 的既有 nil-center panic（任务开始前已存在，与本次无关）；
- `starter-asynq` 的 redis/span 断言失败（依赖本机 redis，与 fault 无关）。

## 5. 目标架构（fault）

```go
// 共享内核（不导出，或放在 fault 内部的中立部分）：
//   Config / Rule / Target / Decision / evaluate / direction / core / ruleState

// 出站
type ClientManager struct { dir direction; prop traffic.Propagator }
func NewClientManager(prop traffic.Propagator) *ClientManager
func (m *ClientManager) WrapClientExecutor(inner chain.Executor) chain.Executor
func (m *ClientManager) NewClient() *Client        // 自定义适配器（不包 executor 时）
func (m *ClientManager) Refresh(Config)
func (m *ClientManager) Config() Config

// 入站
type ServerManager struct { dir direction; prop traffic.Propagator }
func NewServerManager(prop traffic.Propagator) *ServerManager
func (m *ServerManager) NewServer() *Server
func (m *ServerManager) Refresh(Config)
func (m *ServerManager) Config() Config
```

- `Client` 是 `chain.Executor` 的壳：per-attempt `Fire`、写 `chain.Recorder`。
- `Server` 是中间件的壳：per-request `Fire`、返回 `Decision/error`，不碰 chain。
- 两个 manager 各自注册 bean。client starter 只注入 `*clientfault.ClientManager`；server starter
  只注入 `*serverfault.ServerManager`。
- 治理中心持有两个：`Center.FaultClient()` / `Center.FaultServer()`，分别 `Refresh`。

### 推广到其它层

先做一次盘点（Step 2 的产出），确认哪些是“公开类型上合并 client/server”：

- `resilience.Manager`：`ClientExecutorFor` + `ServerGuardFor` 在同一个类型上；
- `observability`：`WrapClientExecutor` / `NewServerObserver`（包装对象不同，但中心侧目前由
  `Center` 统一组合）；
- `governance.Center`：`ExecutorFor`（出站）与 `InboundFor`（入站）在同一个中心上；
- `loadbalance`：只有 client 侧，不涉及；
- `fault`：本任务的主体。

拆分粒度按层决定：能共用的是“策略文档 + 状态引擎 + 推送”；必须分开的是“授权对象 + 创建
API + 宿主绑定”。

## 6. 分步计划

### Step 1：fault 拆分（✅ 已完成）

1. 把共享内核从 `Manager` 上摘出来（→ `core.go`），定义 `ClientManager` / `ServerManager`。
2. 改为两个 bean；`governance.Center` 持有两者，提供 `FaultClient()` / `FaultServer()`。
3. client 侧：`center.ExecutorFor` 改用 `FaultClient().WrapClientExecutor(...)`；httpx 同步。
4. server 侧：6 个适配器改注入 `*serverfault.ServerManager` 并 `NewServer()`。
5. 更新测试、README/DESIGN、memory。
6. 验证：`cloud` 全量 + 受影响 starter。

### Step 2：其它层的盘点与拆分（逐层、可分开评审）

1. 输出一份“当前 client/server 合并点”清单（类型、方法、bean、调用点）。
2. 对 resilience、observability、governance 逐层决定：拆分 or 保留并记录理由。
3. 先拆 API 面（manager/authority 对象），再迁移调用点；每层单独验证。
4. 更新 `ARCHITECTURE*.md` 与各包 DESIGN/README；删除被拆掉的合并 API。

### Step 3：收尾（✅ 已完成）

- 全仓 grep 残留的“两方向合一”公开类型；
- 文档与代码一致（AGENTS.md 要求 "one home per rule"）；
- 更新 memory。

**结果**：非测试代码中同时带 client/server 面的公开类型只剩三类，均为有意保留：
`governance.Center`（组合根）、`resilience.Driver`（一个后端答两个方向）、
`security.TLSConfig`（配置值的两个构建方法）。该规则与例外已写入 `ARCHITECTURE*.md`。

**顺带修复**：`starter-echo` / `starter-http-server` 的 nil-center 测试 panic
（此前测试假设 nil 会归一化成未武装 authority，与 `InboundOver` 的“必须非 nil”契约矛盾）；
现在测试改为构造一个经过 `Apply(空文档)` 的真实 server authority。两个包测试转绿。

**仍标记为环境依赖**：`starter-asynq` 的 redis / span 断言失败（需要本机 redis），
与本次拆分无关。

## 7. 已定决断（Step 1）

1. **两个 bean**：`ClientManager` 与 `ServerManager` 独立注册；Center 同时依赖两者但**不做聚合**。
2. **共享内核位置**：`fault` 用包内 `core.go`，`resilience` 用包内 `settings.go`；
   **不抽跨包中立内核**——两家的规则词汇与匹配模型不同，共享只会制造一个没有第二个使用者的抽象。
3. **命名**：`ClientManager` / `ServerManager`；fault 方法 `Refresh` / `Config`，
   resilience 方法 `Apply` / `Close` + 各自方向 API。
4. **不保留过渡别名**：`Manager`、`Center.Fault()` / `Center.Resilience()`、
   `RefreshClientConfig/RefreshServerConfig`、包级 `NewClient/NewServer` 已全部删除。
5. **治理文档绑定不变**：`spring.governance.client.fault.*` / `server.fault.*` 分别推给两个 manager。
6. **Step 2 的范围已定**：resilience 拆分（✅）；observability 已天然分文件/分类型（不拆）；
   governance.Center 保留组合根（不拆）。

## 8. 验证方式

```bash
cd cloud && go build ./... && go test ./...
# 受影响 starter（client/server 各一批）
cd starter/starter-gin && go test ./...
cd starter/starter-echo && go test ./...        # Step 3 已修复 nil-center
cd starter/experimental/starter-grpc-server && go test ./...
# 唯一仍红：starter/experimental/starter-asynq（需要本机 redis）
```

另加针对性测试：
- client：refresh 推送到已存在的 executor；Close 注销；per-attempt 决策写入 round。
- server：refresh 推送到已捕获的 Server 壳；返回 Decision 供适配器映射。
- 两个 manager 互不可见：client 的 refresh 不影响 server 的壳。

## 9. 关键文件

- `cloud/fault/{core,client_manager,server_manager,executor,server,config,error,starter}.go`
- `cloud/runtime/controlplane/governance/{center,starter,config}.go`
- `starter/starter-{gin,echo,dubbo}/…`、`starter/experimental/starter-{hertz,grpc-server,trpc}/…`
- `starter/starter-http-client/httpx/httpx.go`
- 文档：`cloud/fault/README*.md`、`cloud/runtime/controlplane/governance/README.md`、`ARCHITECTURE*.md`

## 10. Step 2 盘点清单（2026-10-17）

扫描范围：`cloud/` 全部非测试导出类型 + 所有 starter 的注入/调用点。

### 10.1 `resilience.Manager` —— 真正的合并点，应拆（以下为拆分前的历史盘点）

- 类型：`cloud/resilience/manager.go:63`，当前一个 bean
  （`cloud/resilience/starter.go:52`）。
- 一个 `mu` 下并存两套完全独立的字段：
  `clientSettings` / `clientSnap` / `clientSubs` / `clientEntries` / `clientBuildMu`
  与 `serverSettings` / `serverSnap` / `serverEntries` / `serverBuildMu`，
  外加两个方向共用的 `applied` 与 `driver`。

| 方向 | 公开方法 | 宿主绑定 |
|---|---|---|
| client | `ClientExecutorFor`、`ClientPolicyFor`、`Subscribe`；`Resolve` / `ResolveLabel`（实现 `PolicySource`，由 executor/driver 消费） | `chain.Executor` 缓存 + 订阅表 |
| server | `ServerGuardFor`、`ServerPolicyFor`；`ResolveRoute`（实现 `ServerPolicySource`） | `ServerGuard` / `ServerEngine` + 路由解析，无 retry/fallback |
| 共享 | `NewManager(driver)`、`Apply(Settings)`、`Close()` | — |

- 应共享：`Settings` / `ClientSettings` / `ServerSettings`、`ClientPolicy` / `ServerPolicy`、
  `ClientRule` / `ServerRule`、`CallPolicy` / `RoutePolicy`、
  `PolicySource` / `ServerPolicySource`、**`Driver`（保留单一接口）**、`ServerEngine`。
- 应分开：授权对象、创建 API、executor/guard 缓存、订阅表、`Apply`/`Close` 的接收者。
- 调用点：
  - `governance.Center.res` 字段与 `NewCenter` / `NewProcessCenter` 签名
    （`center.go:99,126,155`）、`Center.Resilience()`（`center.go:173`）。
  - `governance.InboundOver`（`center.go:264`）→ `starter-http-server/server_policy.go:57`。
  - `starter-dubbo/dync.go:81-82`（client 侧 `ClientPolicyFor`）。
  - `starter-http-client/httpx/httpx.go:252`（经 `Center.Resilience()`）。
  - `starter-governance-{etcd,nacos}/example/main.go`。
  - 测试：**23 个文件、47 处**引用 `resilience.NewManager` / `*resilience.Manager`。
- 后端不受影响：`starter-governance-sentinel`、`starter-luohua` 只实现 `Driver`
  （`NewClientExecutor` + `NewServerExecutor`），拆分 Manager 不触碰它们。
- 建议 API：
  ```go
  resilience.ClientManager  // NewClientManager(driver)
                            // Apply(ClientSettings) / ClientExecutorFor / ClientPolicyFor
                            // Subscribe / Resolve / ResolveLabel / Close
  resilience.ServerManager  // NewServerManager(driver)
                            // Apply(ServerSettings) / ServerGuardFor / ServerPolicyFor
                            // ResolveRoute / Close
  ```
  两个 bean；`Driver` 与 `Settings` 仍是中立类型。

### 10.2 `observability` —— 已天然分开，不拆

- client：`observability/client.go:94` `WrapClientExecutor` + `wrappedClientExecutor`。
- server：`observability/server.go:83-89` `ServerObserver` / `NewServerObserver` / `Observation`。
- 没有合并的授权对象或 manager；只有 `common.go` 的共享词汇。
- 结论：**无需改动**，在 ARCHITECTURE 里记录理由即可。

### 10.3 `governance.Center` —— 保留组合根，不聚合

- 同一类型上有 `ExecutorFor`（出站，`center.go:235`）与 `InboundFor`（入站，`center.go:256`）。
- 但它同时是治理的**唯一注入点**与组合根；用户已明确 center 层不做特意聚合。
- 结论：**保留单 Center**；只把内部 `res` 字段拆成两个 resilience 权威，getter 相应变为
  `ResilienceClient()` / `ResilienceServer()`（或同类命名），不得引入 `FaultPair` 式聚合类型。
- 连带改动：`InboundOver`、`starter-http-server.ServerPolicy`、`httpx`、examples、测试。

### 10.4 `loadbalance` —— 仅客户端

- `loadbalance.Manager` 只有出站选择；无 server 对应物。**不涉及。**

### 10.5 其它层 —— 扫描后不涉及

- `security/tls.go`：`TLSConfig.BuildClient()` / `BuildServer()` 是同一配置值的两个构建方法，
  不是授权对象/bean；保留。
- `cache` / `lock` / `mesh` / `messaging` / `propagate` / `ratelimit` / `scheduling` /
  `traffic` / `actuator`：未发现同时承载 client/server 创建 API 的公开 manager。
- 结论：记录“已审查、无需处理”。

### 10.6 Step 2 执行顺序（✅ 已完成）

1. ✅ `resilience`：拆 `ClientManager` / `ServerManager`，两个 bean；先 API 面，再迁 Center 与调用点。
2. ✅ `governance.Center`：内部字段与 getter 换成两个 resilience 权威；保留单 Center。
3. ✅ `observability` / 其它层：只记录结论，不改代码。
4. ✅ 更新 `ARCHITECTURE*`、`cloud/resilience/README*`、`cloud/runtime/controlplane/governance/README*`、TASK 文档与 memory。
5. ✅ 验证：`cloud` 全量 + 直接注入 resilience 的 starter（http-server、dubbo、httpx、sentinel、luohua、
   governance-{etcd,nacos}）+ 受 Center 签名影响的全部 starter。

### 10.7 Step 2 实施结果（2026-10-17）

- `cloud/resilience/settings.go`：中立词汇（`Settings` / `ClientSettings` / `ServerSettings` /
  `ClientRule` / `ServerRule` / `ruleKey`）+ 包文档；`server_manager.go`（旧 handle 文件改名为
  `server_handle.go` 让位）。
- `ClientManager`：`NewClientManager(driver)`、`Apply(ClientSettings)`、`ClientExecutorFor`、
  `ClientPolicyFor`、`Subscribe`、`Resolve` / `ResolveLabel`、`Close`。
- `ServerManager`：`NewServerManager(driver)`、`Apply(ServerSettings)`、`ServerGuardFor`、
  `ServerPolicyFor`、`ResolveRoute`、`Close`。
- `starter.go` 注册两个 bean，共享同一个 `Driver` bean；`Driver` 接口不变。
- `governance.Center`：字段 `resClient` / `resServer`，getter `ResilienceClient()` /
  `ResilienceServer()`；`NewCenter(resClient, resServer, lb, injClient, injServer, disc, src)`；
  `NewProcessCenter(resClient, resServer, injClient, injServer)`；`InboundOver(..., *ServerManager)`。
- `ServerGuard.mgr` 改为 `*ServerManager`。
- 调用点：`httpx` / `dubbo dync` / governance examples 改用 `ResilienceClient()`；
  `starter-http-server.ServerPolicy` 改收 `*ServerManager`。
- 测试：resilience 测试用测试专用 `testManager` fixture 同时持有两个权威；所有 starter 的
  `NewCenter` 构造点补上 server manager。
- 验证：`cloud` 全量绿；受影响 starter 除既有 `starter-echo` / `starter-http-server` 的
  nil-center panic 与 `starter-asynq` 的本机 redis 断言外全绿。前两者已在 Step 3 修复。
- **未做**：`observability`（已天然分开）、`loadbalance`（仅客户端）、`security/tls`（非授权对象）
  仅记录结论，无代码改动；`Center` 本身保持组合根，未拆。

## 11. guard 迁移后的最终状态（2026-10-17）

Step 1-3 描述的是“同一 `resilience` 包内拆 ClientManager / ServerManager”的中间态。
此后 server 侧被整体迁到新包 `cloud/runtime/dataplane/inbound/admission`，最终结构是：

- **client / 出站**：`chain.Executor` + `resilience`（`ClientManager`、`Driver.NewClientExecutor`）。
- **server / 入站**：`admission.Guard`（每实例 state、`Admit` + `Done`、无 retry、不 import `chain`）。
- `resilience` 的 server 半边已删除；`Driver` 只答出站；sentinel / luohua 只保留出站实现。
- `governance.Center` 同时持有 `resilience.ClientManager` 与 `admission.Manager`，是唯一组合根。
- `Config.Server` / `ServerRule` 直接使用 `admission.Policy`。
- `ServiceLabel` 下沉到中立叶子包 `cloud/label`；server adapter 不再 import `resilience`。
- server 观测使用 `observability.ServerOperation` / `admission.BreakerState`，不再借用 client 的
  `chain.Operation` / `resilience.BreakerState`。

严格隔离检查的最终例外只剩：

1. `governance.Center` —— 组合根，持有两个方向但不下沉执行模型；
2. `security.TLSConfig` —— 配置值的两个构建方法，不是授权对象。

`resilience.Driver` 不再是“同时答两个方向”的例外。

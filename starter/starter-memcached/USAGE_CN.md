# starter-memcached 使用说明 — 参考手册

详细使用参考。概览见 [README.md](README.md)。所有行为声明均已对照 starter 源码
（`starter.go`、`client.go`、`driver.go`、`config.go`、`health.go`、
`bytecache.go`）与可运行的 [example/](example/)（`check.sh` 拉起 docker memcached
并自证 SET/GET/INCR 往返）。**gomemcache 自身语义（分片、文本协议、Item 字段）见
[gomemcache 文档](https://github.com/bradfitz/gomemcache)** —— 下文只写 go-spring 的增量。

**激活条件**：只有存在任意 `spring.memcached.instances.*` key 时才有 bean —— `gs.OnProperty("spring.memcached")`
是前缀匹配（`starter.go:42`）。仅多实例：每个 `spring.memcached.instances.<name>` map 项生成一个命名
client bean 加一个健康指示器；没有 `__default__` 单例。

---

## 1. 完整工程示例

文件树（对应 `example/`）：

```
demo/
├── go.mod
├── main.go
├── service.go
├── discovery.go        # 注册 discovery 后端（仅使用 service-name 时需要）
└── conf/
    └── app.properties
```

**前置依赖**（唯一外部依赖）：

```bash
docker run -d --name demo-memcached -p 127.0.0.1:11211:11211 memcached:1.6
```

**go.mod**：

```
require (
    github.com/bradfitz/gomemcache/memcache latest
    go-spring.org/spring                    v1.3.x
    go-spring.org/starter-memcached         latest
    go-spring.org/starter-actuator          latest   // 可选：/readiness 折入 memcache 健康
    go-spring.org/starter-governance-file        latest   // 可选：memcached 操作的 resilience/fault
)
```

**main.go**：

```go
package main

import "go-spring.org/spring/gs"

func main() { gs.Run() }
```

**service.go** —— 注入包装类型而非裸 client（见 §2.3）：

```go
package main

import (
    "github.com/bradfitz/gomemcache/memcache"
    "go-spring.org/spring/gs"
    StarterMemcached "go-spring.org/starter-memcached"
)

type Service struct {
    // autowire tag = spring.memcached.instances.<name> 的 map key
    Memcached    *StarterMemcached.Client `autowire:"cache"`
    SessionCache *StarterMemcached.Client `autowire:"session"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}
```

**discovery.go** —— 仅 `service-name` 寻址方式需要（此处为静态后端；真实后端对接
Consul/Nacos 并推送新快照）：

```go
package main

import "go-spring.org/cloud/discovery"

func init() {
    gs.Provide(func() (discovery.Discovery, error) {
        return discovery.NewStaticDiscovery(    discovery.Endpoint{Addr: "127.0.0.1:11211", Healthy: true}), nil
    }).Name("default")
}
```

**conf/app.properties**（与 `example/conf/app.properties` 同面）：

```properties
spring.memcached.instances.cache.servers=127.0.0.1:11211

spring.memcached.instances.session.servers=127.0.0.1:11211
spring.memcached.instances.session.timeout=100ms
spring.memcached.instances.session.max-idle-conns=4

# 服务发现寻址：不配 `servers`；列表来自已注册后端。
spring.memcached.instances.discovery.service-name=memcached-cluster
```

**验证**（example 在 :9090 暴露同样 handler；`go run . -manual` 运行）：

```bash
curl http://127.0.0.1:9090/set     # -> OK
curl http://127.0.0.1:9090/get     # -> value
curl http://127.0.0.1:9090/incr    # -> 1, 2, 3 ...
curl http://127.0.0.1:9090/get     # 删除后："memcache: cache miss"
```

或无头方式：`cd example && ./check.sh` —— 自证 SET/GET/INCR、删除后 miss、discovery client
往返，任一失败即非零退出。

---

## 2. 装配与时序

### 2.1 bean 生命周期

```
import starter-memcached
  └─ init: gs.Module(OnProperty("spring.memcached"), BindEach)       starter.go:42-43
        对每个 spring.memcached.instances.<name> 项：
          r.Provide(newClient, IndexArg(name,c), IndexArg(3,?Driver))  starter.go:47-51
              .Name(name).Destroy((*Client).Destroy)     # 无 InitMethod —— 见下
          r.Provide(健康指示器 "memcache:"+name)                      starter.go:65
gs.Run()
  ├─ 配置绑定：${spring.memcached.instances.<name>} → Config（value tag）       config.go:24-58
  ├─ 构造 newClient [starter.go:87]：校验 → 可选 Driver bean
  │     （无则用内置 DefaultDriver）→ d.CreateClient(c, backend)
  │         └─ 内调 NewClient：身份 + observe 层                      client.go
  ├─ 构造期 Client.applyGovernance(mgr, inj)：resilience/fault executor
  ├─ 构造期 HealthCheck（走裸 client）—— 装配完整后再探测
  ├─ 就绪：健康指示器把 HealthCheck 折入 /readiness                    `health.go`
  └─ 停机：Client.Destroy —— 关连接池、释放 executor                  client.go
```

**装配扩展点**：client 装配由 `Driver`（接口，`driver.go:31-52`）负责。公司/伞包 starter 可把
自己的 `Driver` 作为**可选容器 bean** 提供（`gs.Provide(func() StarterMemcached.Driver{...})`，
因为是 bean，可在装配期注入从配置文件绑定的配置）；`spring.memcached` 下每个实例都经它构建。
没有该 bean 时 starter 在装配内回退到内置 `DefaultDriver`（`driver.go:55-79`）。当容器中存在多个
Driver bean 时，实例可按名指定：
`spring.memcached.instances.<name>.driver = <bean 名>`（留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean；指定的
bean 不存在则启动失败）。

`CreateClient` 返回导出的 `*Client` —— 也就是应用注入的那个类型 —— 而非裸
`*memcache.Client`，这样自定义 driver 参与的正是生态所见的类型。它返回的客户端**身份完整**：
`NewClient` 是唯一构造入口，driver 把条目的 name 与 `c.ServiceName` 交给它，所以 Client 不可能在缺身份
的情况下存在。observe 层也在此安装 —— 它不需要任何外部输入，属于「客户端是什么」，不是后续的装配步骤。
driver 唯一不装配的是治理：executor 由 `Client.applyGovernance` 装备，构造函数紧接着调用它，因为这些
authority 来自容器，driver 不应被迫依赖 `cloud/governance`。

这里**刻意没有 `Init` 钩子**。初始化就是「对象存在之后立刻要做的事」，而构造函数在一处做完：
`NewClient` 定身份与 observe，`applyGovernance` 装治理。把任一步拆成独立的 `gs.InitMethod` 一无所获，
只会给容器留下一个把半成品客户端发出去的机会。

启动 PING 时机（可选，`ping=true`）：它在**构造函数内部**执行、bean 尚不存在 —— 服务器不可达会以
`memcached: startup ping failed` 中止容器装配（`starter.go`）；不是懒加载、不重试。默认关闭时不执行
探测，服务器不可达不会在启动期被发现，而是到首次操作才暴露。
gomemcache 的 `Ping` 探测全部已配置服务器，`servers` 里一个死节点即令整个实例失败。该探测直接走裸
client：它是对连接的启动检查、不是业务流量，因此不开 span、不占用限流/熔断额度。探测失败即放弃该
客户端，构造函数会释放刚装上的治理。启动探测与就绪指示器都委托给 `HealthCheck`（`health.go`），
即该模块唯一的探活实现。

### 2.2 服务发现寻址流程

设置 `service-name`（且非 mesh 模式）时，starter 把 `discovery` 指名的后端（默认 `"default"`）
解析成 bean 并以 `backend` 参数传给 `DefaultDriver.CreateClient`，后者据此、按 `scheme` 过滤构建 discovery resolver（`driver.go:84`、
`driver.go:100-103`）。初始快照在构建期读一次作为 fail-fast 闸门——空快照使启动失败并报
`memcached: discovery returned no endpoints for %q`（`driver.go:90-99`）——随后 client 建在一个
**活的 `ServerSelector`**（`selector.go`）之上，每次 key 查找都重读快照。于是实例的加入/离开在
下一次操作就可见：不用重启，也不用挂代理。Resolver 的新鲜度在 backend 内部，没有服务需要释放。

两个性质是刻意的，因为对 memcached 而言**选择器就是缓存语义**：

- **保持 key 亲和。** 用与 gomemcache 自带 `ServerList` 完全相同的 CRC32-of-key 方案哈希，且快照
  先按地址**排序**——所以集群不变时，无论命名服务以什么顺序上报，key→server 的映射都不变，
  key 始终落在持有它的实例上。
- **集群不可用要报出来，不能糊过去。** 空快照或读失败按该次操作报 `memcache.ErrNoServers` / 原始
  错误，而不是端出一个 key 可能早已不在的陈旧集合。（想加权就按库的表达方式——同一地址写多遍。）

mesh 模式下完全跳过 discovery，`servers` 原样使用（sidecar 负责发现+LB，`driver.go:76-78` 注释）。

### 2.3 一次 Set 调用逐层走读

`Client.Set(ctx, item)`（`client.go`）：

1. `run`/`runErr` 以**调用方的 ctx** 开启模块本地 observer 的 client span（`observe.go`），
   因此 span 会挂进调用方的请求 trace。gomemcache 的网络调用本身不感知 ctx（socket 等待由
   `timeout` 约束），但 ctx 仍治理 resilience 层的取消（限流等待、重试间隔、熔断判定）。
2. `run` 经 `resilience.Run` 在 resilience executor 下执行操作：
   limiter/breaker 以服务 `memcached:<service-name 或实例名>` 隔离（`client.go`）；
   `memcache.ErrCacheMiss` 计为成功，miss 不会触发熔断（`resilience.Tolerate`）；executor 由
   `Client.applyGovernance` 应用 —— `fault.WrapClientExecutor(mgr.ClientExecutorFor("memcached", service), service, inj)`，
   其中 `mgr`/`inj` 是容器注入 `newClient`、再由构造函数转交的 bean，
   observe 层在 resolve 内应用。未应用治理时为透明 no-op。
3. 裸 client 执行实际写入；span 以错误收尾。

全部 17 个操作（get/get_and_touch/get_multi/touch/set/add/replace/append/prepend/cas/delete/
delete_all/increment/decrement/ping/flush_all）同构（`client.go`）。连同 `Close` 一起，它们就是
`*Client` 的全部表面：裸 client 是私有字段，没有方法被提升，也没有任何导出途径能拿到它 ——
调用方不可能"不小心"绕过 observe 与治理层。

### 2.4 缓存抽象 bean

除包装类型外，每实例另提供一个类型化 `cache.Cache` bean，名为
`memcached:<service-name 或实例名>`（`starter.go:61-67`，经 `NewByteCache`，
`bytecache.go`）——用 autowire tag `memcached:<实例名>` 按名注入。
无人注入则不实例化，因此无配置开关。TTL 转换：`toExp` 把 ttl 映射为
int32 秒——**0/负值 = 永不过期**，亚秒向上取整为 1s，避免被静默当成 forever
（`bytecache.go`）。`GetBytes` 把 `ErrCacheMiss` 映射为 `cache.ErrMiss`；
删除不存在的 key 不算错（`bytecache.go`）。

---

## 3. 逐 key 行为参考

每实例 Config 共 8 个 value tag（grep 审计核实；输出中的 `demo.label` 属于 example 应用而非
starter）。

| key（`spring.memcached.instances.<name>` 下） | 类型 | 默认值 | 行为与联动 | 配错后果 |
|---|---|---|---|---|
| `servers` | []string | 空 | 静态 server 列表；请求按其分片（config.go:28）。与 `service-name` 二选一 | 两者皆空 → 构造错误 `one of servers or service-name must be set`（starter.go:92）；地址死 → 启动 ping fail-fast（仅 `ping=true`） |
| `service-name` | string | 空 | 服务发现寻址：server 集合跟随 `discovery` 指名的后端（config.go:39），每次 key 查找重读（selector.go）。设置后（非 mesh）忽略 `servers` | 后端缺失 → 启动报 `discovery resolve %q failed`；启动期空快照 → 启动报错（driver.go:93-99）；运行期空快照 → 该次操作报 `memcache.ErrNoServers` |
| `scheme` | string | 空 | 把 discovery 收窄到单一传输 scheme 的端点；仅在设 `service-name` 时生效（config.go:45） | 过滤过度 → "no endpoints" 启动错误 |
| `discovery` | string | — | 用哪个已注册的 `discovery.Discovery` 解析 `service-name`（config.go:50）。wiring 把该 label 解析成 bean，并以 `backend` 参数传给 driver 的 `CreateClient`。未配置时回退 `${spring.memcached.default.discovery}`。 | service-name 已设但两层都未配置或名字无对应 bean → 启动报错。 |
| `timeout` | duration | 0 | 每请求 socket 读/写超时；0 = gomemcache 默认 100ms（config.go:54） | 过低 → 高压下伪超时 |
| `max-idle-conns` | int | 0 | 每 server 保留的空闲连接数；0 = driver 默认 2（config.go:58）。治理规则里的 `max-conns` 覆盖它（gomemcache 没有"打开连接上限"，规则 sizing 的就是这个空闲上限）。 | 过低 → 重连抖动 |
| `ping` | bool | false | 可选启动探测：`HealthCheck` 对每个已配置 server 单次 ping，不可达则启动失败；默认关，未就绪的 server 到首次使用才暴露。 | 期待 fail-fast 却没开 → 启动"成功"，首个请求失败。 |
| `health` | bool | true | 为实例注册 `memcache:<name>` 健康指示器。 | false → 无指示器 bean，不再上报该 memcached 的就绪。 |

`driver` key 按名指定 Driver bean：留空 = 先回退家族级 `spring.<family>.default.driver`，再按类型注入唯一 Driver bean（见 §2.1），配置
bean 名则显式选定一个；无 `resilience` key：resilience/fault 来自治理中心
（starter-governance-file 的 `spring.governance.*` 配置），按服务 `memcached:<service-name 或实例名>` 隔离。

---

## 4. 验证与故障演练

1. **SET/GET 往返**：`curl :9090/set` → `OK`；`curl :9090/get` → `value`
   （handler 见 `example/example.go`；check.sh 无头断言）。
2. **cache-miss 语义**：删除 key 后 `curl :9090/get` → `memcache: cache miss`；开 governance
   时反复 miss **不会**熔断（ErrCacheMiss 计为成功，`client.go`）。
3. **server-down fail-fast**：设 `ping=true`，停掉 memcached（`docker stop demo-memcached`）再
   启动应用 → 容器装配以 `memcached: startup ping failed` 中止（`starter.go:123`）。坏 `servers`
   地址表现相同。默认关闭时不跑启动探测，失败到首次操作才暴露。
4. **discovery 坏地址演练**：设 `service-name` 但未注册后端 → 启动报
   `memcached: discovery resolve %q failed`（`driver.go:92`）。注册的后端返回空端点集 →
   启动报 `discovery returned no endpoints`（`driver.go:104`）。
5. **成员变更局限演练**：discovery 实例运行中，从后端快照移除端点 —— client 持续拨旧地址
   直到重启（§2.2）。这是 documented 的 gomemcache 约束，不是接线 bug。
6. **健康/就绪**：import starter-actuator；每个实例贡献名为 `memcache:<name>` 的指示器，
   探测即一次真实 `HealthCheck`（`starter.go:65`、`health.go`）。杀掉服务器后
   `curl :9370/readiness` 翻 DOWN。注意探测无 deadline —— gomemcache 的 `Ping` 无 context，
   由 client 超时兜底（`health.go`）。
7. **多实例**：`cache` 与 `session` 实例并存（bean 名 = map key，`starter.go:47-50`）；两个
   项指向同一服务器也是独立 bean、独立 executor（服务标签 `memcached:cache` 与
   `memcached:session`）。
8. **可观测**：import starter-otel 后，每次调用出现以操作名（`get`/`set`/...）命名的
   span，经传入的 ctx 挂进调用方 trace（§2.3）。本 starter 只声明操作的语义身份（`observe.go`），信号由
   resilience 层发射，因此 call 级 `db.client.operation.duration`、attempt 级
   `db.client.attempt.duration` 直方图与 `_app_memcached_access` 访问日志都出自那里。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|---|---|---|
| 启动报 `one of servers or service-name must be set` | 实例块两者皆未配 | 配其一（`starter.go:92`） |
| 启动报 `memcached: startup ping failed` | 服务器宕机/启动期地址错误 | 仅 `ping=true` 时抛出；启动 memcached、修 `servers`（`starter.go:123`）。`ping` 关闭时失败到首次操作才暴露。 |
| 启动报 `discovery resolve "..." failed` | 设了 `service-name` 但 `discovery` 名下无后端 | 启动前注册命名后端 bean |
| 启动报 `discovery returned no endpoints` | 后端健康但服务无实例（或 `scheme` 过滤过度） | 拉起实例/清空 `scheme`（`driver.go:97-98`） |
| 集群扩缩容后 server 列表不更新 | gomemcache 创建即固定 server 集；watch 仅管生命周期 | 重启进程重新解析（`driver.go:61-68`） |
| trace 里 memcached span 与请求 trace 断联 | 调用方传了无 trace 的 ctx（如 `context.Background()`）；span 跟随调用方 ctx | 传请求的 ctx 让 span 挂进 trace；网络调用本身仍不感知 ctx（受 `timeout` 约束） |
| 熔断在 cache miss 下永不触发 | 设计如此：ErrCacheMiss 计为成功 | 熔断演练须用真实故障而非 miss（`client.go`） |
| readiness 持续 UP 但操作失败 | 指示器只探 `Ping`；慢而活着的服务器照样通过 | 看 observe 指标的真实时延/错误 |

---

## 6. 设计体检表

| 指标 | 数值 |
|--------|------|
| 配置 key | 9（8 连接 + 1 经 cache 桥命名） |
| 必填 | 1（`servers` 与 `service-name` 二选一） |
| quickstart 前置外部依赖 | 1（memcached，docker） |
| "注意/坑"条数 | 4 |

设计嫌疑清单（沿自上一版，已更新）：

- ~~无健康指示器~~ —— **已解决**：每个实例现已注册导出的 `health.Indicator`
  `memcache:<name>`（`starter.go:65`、`health.go`）；配合 starter-actuator
  自动折入 `/readiness`，无需额外接线。
- discovery watch 仅管生命周期：成员变更需重启（gomemcache 结构性约束，`driver.go:61-68`）
  —— 可考虑 rebuild seam 或 client 原子替换模式（参照 dubbo 动态超时的 swap 方案）。
- 网络调用无法经 ctx 取消（gomemcache API 无 context；受 `timeout` 约束）——只有 resilience 层感知取消（`client.go`）。
  （`client.go:37-41`）。

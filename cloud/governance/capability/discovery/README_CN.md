# discovery

[English](README.md) | [中文](README_CN.md)

`discovery` 回答基础设施客户端（Redis / MySQL / MongoDB / Kafka ...）的一个问题：
*"给一个逻辑服务名，当下可连的 host:port 有哪些？"* 命名服务适配一次，所有
客户端消费同一契约。

本包是**读侧**，属于数据面：resolver 与进程内的端点快照会被 `loadbalance`
在请求路径上读取。**写侧**——发布本进程（`Instance` / `Registry` / `Server`）——
是控制面的生命周期，位于
[cloud/governance/controlplane/registration](../../../controlplane/registration/)。
每个 `starter-discovery-*` 后端同时贡献两侧适配器。

## 安装

```
go get go-spring.org/cloud
```

## 快速开始

```go
import (
    "context"
    "net"

    "go-spring.org/cloud/governance/capability/discovery"
)

// d 是 starter 注入的发现后端 bean(bean 名 = 后端名字，如由 ${spring.discovery.etcd.main} 派生的 "etcd.main")。
// 只在客户端发现生效时构造：配置了 service-name 且进程不在 mesh 模式（Runtime.UsesDiscovery）。
load, err := discovery.NewResolver(ctx, d, "orders-redis", "")
if err != nil { return err }                    // fail-fast：构造时没有端点直接报错

eps, err := load()                               // 实时快照，错误如实上抛
if err != nil { return err }
conn, err := net.Dial("tcp", eps[0].Addr)        // socket 和连接池归客户端
```

## Discovery：后端契约

```go
type Discovery interface {
    Resolve(ctx context.Context, name string) ([]Endpoint, error)
}
```

- `Resolve` 返回当前**未收窄**的快照。某个服务的第一次调用可能阻塞（播种查询，
  由 ctx 约束）；之后的调用是廉价读——新鲜度在后端内部：它用注册中心自己的通知
  机制（watch / 订阅 / 轮询）维持缓存最新。
- 收窄刻意不做成参数：后端只回答"这个名字现在有哪些地址"，而所有消费方的过滤
  语义一致，因为实现只在 discovery 包里。`NewResolver` 施加固定的 scheme，
  `FilterByMetadata` 施加动态 selector；直接读快照的调用方自己套这两步。
- 后端是 IoC 容器里的命名 bean（各注册中心 starter 由自己的 `${spring.discovery.<backend>.<name>}`
  配置块派生，bean 名为 "<backend>.<name>"，如 "etcd.main"）；client 在配置里写这个
  名字，starter 按名注入该 bean。容器就是发现目录——名字写错在装配期即报错。

没有注册中心？用内置的 static 后端：

```go
d := discovery.NewStaticDiscovery(
    discovery.Endpoint{Addr: "127.0.0.1:6379", Scheme: "tcp"},
)
```

## Endpoint：可选性

```go
type Endpoint struct {
    Addr     string            // host:port
    Scheme   string            // "tcp"/"" 明文，或 "tls"、"grpc" ...
    Weight   int               // 由 loadbalance 消费
    Disabled bool              // 运维/提供方指令：排空、维护
    Healthy  bool              // 探针结果
    Metadata map[string]string // 只读：由后端缓存持有，所有读者共享
}
```

`Disabled` 与 `Healthy` 是两个独立维度，可选性顺序有讲究：

```
优先选： !Disabled && Healthy
   一个健康的都没有? 退化到： !Disabled
   永不： Disabled —— 连兜底也不进
```

这防住了经典 bug：运维 disabled 的实例被"无健康→用全部"的兜底重新拉回流量。

## Scheme：查询的固定收窄

```go
load, _ := discovery.NewResolver(ctx, d, "orders", "grpc")
eps, _ := load(ctx)
```

第四个参数是要保留的传输 scheme：`""` 表示“所有 scheme”，`"tcp"` 与 `""` 等价
（plain TCP）。`NewResolver` 此后每次读取都用包内的 scheme 过滤器施加它。它在
resolver 的整个生命周期内固定：收窄一条连接是构造期决定，不是每次读取的参数。
它刻意不带元数据 selector——那是动态选择策略，由 loadbalance/governance 在每次
读取时施加：

```go
eps, _ := d.Resolve(ctx, "orders")
eps = discovery.FilterByMetadata(eps, sel)
```

空 scheme 表示“不收窄”，是 no-op——配置值可以无条件透传。

Selector 是作用在 `Endpoint.Metadata` 上的类型化合取式。匹配种类是封闭集合——
`exact`、`prefix`、`suffix`、`contains`——term 是纯值，所以同一条规则既可以写在
代码里，也可以从控制面解码而来。`matcher.Selector.Validate` 是解码规则的边界校验
（未知 kind、空 key、空的 prefix/suffix/contains）；在规则生效时校验一次，
失败就继续用上一条规则。这里刻意没有布尔连接符：“或”属于上一层的规则列表。空
selector 表示“不收窄”，因此控制面解码器应当拒绝显式空规则，而不是把它装成“匹配
全部”。

## Resolver：按名绑定的消费方

```go
backend := discovery.NewStaticDiscovery(/* ... 来自注册中心的实时端点 ... */)
load, err := discovery.NewResolver(ctx, backend, "orders-redis", "")
bal, err := loadbalance.New(loadbalance.Selection{Balancer: loadbalance.RoundRobin})
if err != nil { return err }
pool := loadbalance.NewPool(load, bal, loadbalance.NewTracker(loadbalance.TrackerConfig{}), loadbalance.Selection{})
ep, err := pool.Pick(ctx, loadbalance.PickInfo{})
```

- `NewResolver` 把后端 + 服务名（加上固定 scheme）一次性绑定，用一次同步 `Resolve`
  播种（fail-fast）。它没有"不生效"状态：调用方只在客户端发现生效时才构造
  （`Runtime.UsesDiscovery`）；没有 service-name 或处于 mesh 模式的 entry 走 client
  自己的直连路径。
- 每次调用 resolver 都重读后端快照并如实上抛错误——对带缓存的后端就是一次廉价
  内存读，不掩盖注册中心抖动。
- 端点选择——round-robin、权重、一致性哈希、失败摘除——全在上一层
  [`loadbalance`](../loadbalance/README_CN.md)，它把 resolver（函数本体）收作端点源。
  发现本身不携带选择策略，resolver 也不持有任何资源——新鲜度全在后端内部，
  没有什么可 Stop。

想自己管理端点集？需要快照时直接 `Resolve`，然后自己施加运行期维度：

```go
eps, _ := d.Resolve(ctx, "orders")
eps = discovery.FilterByMetadata(eps, sel)
```

静态的 scheme 维度没有公开 helper——要它就构造 resolver，让 scheme 固定一次，
而不是在调用侧重写一遍。

## 写一个后端

```go
type myBackend struct {
    eps []discovery.Endpoint // 本后端自己的缓存
}

func (b *myBackend) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) {
    // 读缓存快照：首次调用查一次注册中心播种，之后用注册中心自己的
    // watch / 订阅 / 轮询机制保持新鲜。
    // 结果是只读契约：调用方不得赋值、重排、append，也不得写 Metadata，
    // 所以这条每请求路径上不需要防御性拷贝。
    return b.eps, nil
}
```

收窄不是后端的职责：它返回该名字的全量集合，消费方在构造期传入 scheme、每次读取
用 `FilterByMetadata`。

在 starter 的模块接线里：

```go
r.Provide(func() (discovery.Discovery, error) { return &myBackend{}, nil }).Name("default")
```

要求并发安全；带 SDK 的适配器（Nacos / Consul / etcd / DNS / Kubernetes）住在各自的
starter 里，本包只依赖本仓库的 `stdlib` / `matcher`，不引入第三方库。

## 发布

发布生命周期是控制面那一半，位于
[registration](../../../controlplane/registration/)：`Server` 在应用就绪后把本进程
注册到所有已配置中心、停机时（PreStop，无损下线时序）反注册、并把权重变更广播
出去；每个后端 starter 为每个配置块贡献一个 `registration.Registry`。`Registry`
契约与生命周期保证见该包的 README。

### 配置

整个 discovery 的配置面分**三部分**，各自独立、可单独出现：

| 部分 | 回答的问题 | 配置前缀 | 谁来配 |
|------|-----------|---------|--------|
| ① 实例身份 | 我是谁 | `${spring.discovery}.*` | 仅提供方 |
| ② 注册中心 | 往哪注册、从哪发现 | `${spring.discovery.<backend>.<name>}.*` | 提供方和/或消费方 |
| ③ 引用 | 消费方用哪个中心 | 各 client starter 自己的配置段 | 仅消费方 |

**① 实例身份**——`${spring.discovery}.*`：所有中心共享同一份。`service-name` 是
注册意图信号：**有值 = 发布本进程；无值 = 纯消费端**（不向任何中心注册）。

```properties
spring.discovery.service-name=orders
spring.discovery.addr=10.0.0.5:8080      # 注册时必填
# spring.discovery.id=                   # 可选，留空自动派生
# spring.discovery.weight=100            # 0 = 摘流；负数写入时归一为 1
# spring.discovery.version=              # 应用版本，消费端可按版本灰度路由
# spring.discovery.zone=                 # 可用区，消费端可同区优先
# spring.discovery.scheme=               # 传输协议提示（tcp/tls/http/https）
# spring.discovery.metadata.zone=cn-north
```

**② 注册中心**——命名块，一块一个中心：块数不限、后端可混用（etcd + zookeeper
同进程正常）。每个块是一个 bean，名字为 `<backend>.<name>`，**一体两面**：写侧被
`registration.Server` 收集为 `registration.Registry`，读侧是消费端按名引用的
`Discovery` 后端。各后端的完整 key 见其 starter 的 README/USAGE。

```properties
spring.discovery.etcd.main.endpoints=10.0.0.1:2379
spring.discovery.etcd.dr.endpoints=10.9.0.1:2379      # 双注册：注册扇出到两个中心
spring.discovery.zookeeper.bz.servers=10.1.0.1:2181   # 混用后端
```

**③ 引用**——消费端按 bean 名指定中心，写在各 client starter 自己的配置段里；
发现侧自身零配置，后端不感知谁在引用。

```properties
spring.http-client.instances.users.service-name=users
spring.http-client.instances.users.discovery=etcd.main    # bean 名 "<backend>.<name>"
spring.http-client.default.discovery=etcd.main            # 家族默认
```

三部分的边界：

- **①与②解耦**：配了块 + service-name 就注册，哪怕没有任何消费端引用它；反过来纯
  消费端可以只配块、不设 service-name，什么都不注册。
- **设了 service-name 但没有任何块 = 启动失败**（fail-fast）。
- **k8s 只出现在②③**：它是 discovery-only 后端，只能被引用、不参与注册（平台已替
  Pod 注册），与①永远无关。
- **同进程可既是提供方又是消费方**：①+②+③ 全配即可。

`Server` 不开端口；接入 Go-Spring 的 server 生命周期。它自身不带任何注册中心后端
——配了 `service-name` 但没有任何注册中心块时启动失败（fail-fast，不静默，报
"no discovery center is configured"）。

## Manager：后端之上的目录

`cloud/governance/capability/discovery` 还提供一个小的 `Manager`：

```go
func NewManager(backends map[string]Discovery) *Manager
func (m *Manager) Get(name string) (Discovery, bool)
func (m *Manager) Names() []string
```

- 构建后不可变、无锁；持有 nil map 时所有查找一律 miss。runtime 会把 nil 目录归一化成
  空 Manager，所以客户端拿到的 `Runtime.Discovery()` 永远非 nil。（2026-10-03）
- 接线为
  `gs.Provide(func(backends map[string]Discovery) *Manager, gs.IndexArg(0, gs.TagArg("?")))`
  ——与 resilience / loadbalance / fault 三个 authority 完全同构（谁拥有谁注册）；
  无后端 bean 时 map 为 nil = 空目录。
- Manager 由 **runtime 组合根**持有：`Runtime.disc` + `Discovery()` 访问器。**discovery
  不进治理语义**——`governance.Center` 不持有它，规则文档 / `adopt` / `dispatch` /
  `Config` schema 一律不碰，Center 只处理策略。此拍板明知多一层转发仍要单入口，
  **别再翻案**。（2026-10-03）
- client 统一经 `Runtime.Discovery()` 查表：有的直接持有 `*discovery.Manager`
  （http-client / gateway / grpc），有的把配置里的 backend 名带进构造器再 `Get`
  （mongodb / elasticsearch / neo4j / memcached / go-redis / redigo / gorm），
  构造器参数名用 `discoveryLabel`，避免与治理层的 service label 混淆。

## 新鲜度，以及 resolver 为什么不分策略

- `Discovery` 接口只留 `Resolve`（快照读，首次调用播种）；`Watch` / `WatchResult` /
  channel 契约删除，新鲜度机制全部下沉到各后端内部——别提议恢复 `Watch` 接口。
  （2026-09-04）
- 后端新鲜度选型：事件推送（etcd `clientv3.Watch` / nacos `Subscribe` 回调刷缓存）、
  读穿 TTL（DNS 类，无 goroutine）、informer（k8s endpointslice，`Close` 停全部）、
  blocking-query 循环（consul）。
- `discovery.Resolver` 是函数类型 `func(ctx context.Context) ([]Endpoint, error)`；构造器
  `NewResolver(ctx, backend, name, scheme)` 绑 backend+name 一次、同步 `Resolve` 播种
  fail-fast；**零资源无 Stop**。调用方只在客户端发现生效时构造
  （`Runtime.UsesDiscovery`），mesh 与无 service-name 的 entry 不会走到这里。
- **Resolver 不做端点选择。** `Pick` 与内部 mini-RR 已删，选择全归
  `loadbalance.Pool`。语义分工：`resolver`（建连，慢时间尺度）与
  `loadbalance.Pool`（每请求，快时间尺度）两套选择器——连接池 client 用前者，
  RPC/httpx 用后者；将来 infra client 需按权重建连应在内部升层用 loadbalance，
  **不给 Resolver 加策略**。（2026-09-04）
- `Pool` 直接收 `discovery.Resolver`：`NewPool(src discovery.Resolver, bal Balancer,
  tracker *Tracker, sel Selection)`，`Pool.Pick` 传播源错误。没有 `EndpointSource` / `SourceFunc`
  （已删）；`discovery.Allows` 早已移成 loadbalance 私有 `admission`。
- 连接池消费形态：`NewPool(resolver, bal, NewTracker(TrackerConfig{}), Selection{})`，拨号闭包里 `pool.Pick(ctx, PickInfo{})`；
  gormcore 提供 `Common.NewResolver` / `NewPickPool` 共享函数。
- 「cloud 无 spring 依赖」从铁律降级为默认倾向（用户明示可突破）。

## 后端 starter 约定

- discovery 后端是 IoC 容器的**命名 bean**（bean 名 = 配置里写的名字原文，不加前缀），
  不是 `cloud/governance/capability/discovery` 里的全局 map；`RegisterDiscovery` / `GetDiscovery` 已删。
  （2026-09-08）
- `NewResolver(ctx, d Discovery, name, scheme string)` 收注入实例；调用方已决定
  进程内发现生效，因此 d 与 name 是前置条件（否则 entry 走直连路径）。
- 新后端 starter 照抄 `starter-discovery-etcd`（文件是 `discovery.go` + `registry.go`）：
  `r.Provide().Name()` + bean Destroy；名字配了但注入 nil 必须 fail-loud 列出可用名。
- 各后端非导出实现类型名（`etcdRegistry` / `zkRegistry` / `consulRegistry` /
  `nacosRegistry`）与测试假件不必镜像接口名（先例 `staticBackend implements Discovery`）。
- `Registry.UpdateWeight(ctx, inst, weight)` 签名陷阱：etcd 实现里 `inst` 只用于定位，
  其余字段连同 `inst.Weight` 被忽略——要么收成身份参数，勿以现状为准。

## registry 家族配置（终态 v2，旧单块方案已废弃）

- 命名空间是 **`spring.discovery.*`**（不是 `spring.registry.*`——后者在代码里零命中）。
  命名块：`spring.discovery.<backend>.<name>.*`，**无默认块**（与 client starter 多实例
  约定一致；默认块 + 命名块同层会因标量/子树不可判定而不可行）。（2026-09-09）
- 每块一个 backend bean，名 `<backend>.<name>`，**同 bean 同时实现 `registration.Registry`
  + `discovery.Discovery`**（一个 bean 两种 Export），连接/探活/key-prefix 或 namespace
  单源。
- `discoveryServer` 是唯一 `gs.Server`，注入
  `Registries []registration.Registry  autowire:"?"` 切片收集**跨后端全部** registrar：
  就绪注册全部 / PreStop 反注册全部 / UpdateWeight 广播；绑全局 `${spring.discovery}`
  身份（`service-name` 为注册意图信号）。后端 starter `import _` 传递引入，Go package
  init 去重保证单 Server，**勿用 OnMissingBean 协商**。
- 发现端引用写 `discovery=etcd.main`：键名是 `discovery`，值是 backend bean 名（带后端
  类型前缀，跨界永不撞名）。client 侧配置形如 `spring.go-redis.default.discovery`。
- 服务端多写默认：多块 + `service-name` = Dubbo 式全量双注册；纯消费端 = 只配块无
  `service-name`。
- 值分派（type=字符串）被否——需全局注册表 + 激活协商 + 晚绑定，违背「类型在 key 里」
  与「容器只装配」拍板。
- 注册端形态：每家构建一个共享客户端 bean（含启动探测），注册端 Server 与发现端后端都
  从它派生；发现端 bean 从中心自动派生（默认名 = 后端名），双角色应用只配一次集群、
  读写同 prefix/scope 不漂移。
- 新后端照抄四家形态（`BindEach(p, "${spring.discovery.<backend>}", ...)` + `OnProperty`
  前缀 + 双 Export + import 核心）；registrar 不需要命名（接口收集）；多集群发现 = 多块。
- k8s 在后端家族里，配置 `spring.discovery.k8s.<name>`、bean 名 `k8s.<name>`、客户端
  `discovery: k8s.<name>`；但**目录名仍是 `starter-discovery-k8s`**（并没有改名成
  `starter-registry-*`）。命名统一性优先于「registry 暗示写侧」的论证——没有 registrar
  的后端也在家族里，用文档说明它只做发现。
- **「只做发现」成员判别标志：** 不 import registry 核心、不提供 `registration.Registry`、
  不读 `${spring.discovery.service-name}` / `.addr`。该例外在三处登记别再当漏配
  （`scripts/check-observability.sh` 的 `NO_REGISTRAR="k8s"`、`starter/DESIGN{,_CN}.md`
  §3/§4），也别提议「补一个 registrar」；只做发现的后端仍要上报读侧（`obsSystem` +
  成功/失败两侧 `discovery.Synced`）。
- etcd 与 zookeeper 各自持有逐字重复的 `instanceValue` JSON struct（及 `withDimensions`），
  **不合并**：wire format 是每个后端存储的冻结兼容面，共享类型会把两个格式的演进耦合
  （为一个后端加的字段会静默变另一个后端的存量数据）。见到相似不要提议下沉；同理
  `config`（绑定）、`registration.Instance`（领域）、`instanceValue`（存储）三层「形似」
  是各司其职。

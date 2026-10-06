# discovery

[English](README.md) | [中文](README_CN.md)

`discovery` 回答基础设施客户端（Redis / MySQL / MongoDB / Kafka ...）的一个问题：
*"给一个逻辑服务名，当下可连的 host:port 有哪些？"* 命名服务适配一次，所有
客户端消费同一契约。两侧都在这里：本包驱动本进程自身的发布（`Server`），
每个 `starter-discovery-*` 后端贡献与它自己那个中心对话的 `Registry`。

## 安装

```
go get go-spring.org/cloud
```

## 快速开始

```go
import (
    "context"
    "net"

    "go-spring.org/cloud/discovery"
)

// d 是 starter 注入的发现后端 bean(bean 名=后端标签，如由 ${spring.discovery.etcd.main} 派生的 "etcd.main");
// nil 表示"未启用发现"。
load, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }                    // fail-fast：构造时没有端点直接报错
if load == nil { return err }                    // "不生效"（无后端/无名/mesh）：直接拨配置地址

eps, err := load()                               // 实时快照，错误如实上抛
if err != nil { return err }
conn, err := net.Dial("tcp", eps[0].Addr)        // socket 和连接池归客户端
```

## Discovery：后端契约

```go
type Discovery interface {
    Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)
}
```

- `Resolve` 返回当前快照。某个服务的第一次调用可能阻塞（播种查询，由 ctx
  约束）；之后的调用是廉价读——新鲜度在后端内部：它用注册中心自己的通知
  机制（watch / 订阅 / 轮询）维持缓存最新。
- 后端是 IoC 容器里的命名 bean（各注册中心 starter 由自己的 `${spring.discovery.<backend>.<name>}`
  配置块派生，bean 名为 "<backend>.<name>"，如 "etcd.main"）；client 在配置里写标签，starter 按名注入该 bean。
  容器就是发现目录——标签重复、拼错在装配期即报错。

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
    Metadata map[string]string
}
```

`Disabled` 与 `Healthy` 是两个独立维度，可选性顺序有讲究：

```
优先选： !Disabled && Healthy
   一个健康的都没有? 退化到： !Disabled
   永不： Disabled —— 连兜底也不进
```

这防住了经典 bug：运维 disabled 的实例被"无健康→用全部"的兜底重新拉回流量。

## 选项：收窄查询

```go
eps, _ := d.Resolve(ctx, "orders", discovery.WithScheme("grpc"), discovery.WithTag("v2"))
```

| 选项 | 语义 | 谁来生效 |
|---|---|---|
| `WithScheme(s)` | 限定到单一传输 scheme；空 scheme 与 `"tcp"` 等价 | 所有后端，经 `FilterByScheme` |
| `WithTag(t)` | 注册中心原生标记（Consul service tag、discovery label） | 注册中心支持 tag 的后端在查询侧过滤；其余忽略 |

两者传空都是 no-op——配置值可以无条件透传。

## Resolver：按名绑定的消费方

```go
backend := discovery.NewStaticDiscovery(/* ... 来自注册中心的实时端点 ... */)
load, err := discovery.NewResolver(ctx, backend, "orders-redis")
bal := loadbalance.NewRoundRobin()
pool := loadbalance.NewPool(load, bal)
ep, err := pool.Pick(loadbalance.PickInfo{})
```

- `NewResolver` 把后端标签 + 服务名（加上选项）一次性绑定，用一次同步 `Resolve`
  播种（fail-fast）；名字为空或 mesh 模式开启时返回 `(nil, nil)`——"发现不生效"，
  调用方直接拨配置地址。
- 每次调用 resolver 都重读后端快照并如实上抛错误——对带缓存的后端就是一次廉价
  内存读，不掩盖注册中心抖动。
- 端点选择——round-robin、权重、一致性哈希、失败摘除——全在上一层
  [`loadbalance`](../loadbalance/README_CN.md)，它把 resolver 经
  函数本体）收作端点源。发现本身不携带选择策略，resolver 也不持有
  任何资源——新鲜度全在后端内部，没有什么可 Stop。

想自己管理端点集？需要快照时直接 `Resolve`:

```go
eps, _ := d.Resolve(ctx, "orders", discovery.WithScheme("grpc"))
```

## 写一个后端

```go
type myBackend struct{ /* 命名服务客户端 */ }

func (b *myBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
    q := discovery.NewQuery(name, opts...)
    // 读缓存快照（首次调用查一次注册中心播种，之后用注册中心自己的
    // watch/订阅/轮询机制保持新鲜）；
    // 用 discovery.FilterByScheme(raw, q.Scheme) 过滤；
    // 注册中心支持 tag 则在查询里带上 q.Tag
}

// 在 starter 的模块接线里：
r.Provide(func() (discovery.Discovery, error) { return &myBackend{}, nil }).Name("default")
```

要求并发安全；带 SDK 的适配器（Nacos / Consul / etcd / DNS / Kubernetes）住在
各自的 starter 里，本包保持零依赖。

## 发布：注册核心

`Server` 拥有本进程在全部已配置中心上的发布生命周期——唯一那个"注册到所有中心、
从所有中心反注册、把权重变更广播出去"的 bean。每个后端 starter 为每个已配置的
`${spring.discovery.<backend>.<name>}` 块派生一个 `Registry`；`Server` 把它们
全部收集起来——跨后端——并统一驱动：

- 应用就绪后，注册到所有中心；
- 停机开始时（PreStop，先于任何 server 停止），从所有中心反注册——无损下线时序；
- 权重变更广播到所有中心（`UpdateWeight`）。

任一中心注册失败即启动失败：消费方在某个中心看不到本实例等于视图分裂。应用就绪前
什么都不注册，整体以注册意图信号为条件。

```go
type Registry interface {
    Register(ctx context.Context, inst Instance) error      // 就绪后调用一次
    Deregister(ctx context.Context, inst Instance) error    // 幂等
    UpdateWeight(ctx context.Context, inst Instance, weight int) error
}
```

discoveryServer bean 上的 `UpdateWeight(ctx, weight)` 向所有中心以新权重重新发布
本实例——实例不会离开发现视图；观察方（含 loadbalance pool）在下次快照看到新值。

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
`Server` 收集为 `Registry`，读侧是消费端按名引用的 `Discovery` 后端。各后端的完整
key 见其 starter 的 README/USAGE。

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

`cloud/discovery` 还提供一个小的 `Manager`:

```go
func NewManager(backends map[string]Discovery) *Manager
func (m *Manager) Get(label string) (Discovery, bool)
func (m *Manager) Labels() []string
```

- **nil-receiver 安全**（nil → 一律 miss 不 panic）、构建后不可变无锁、`Labels()`
  空时返回 nil（不是空 slice）。（2026-10-03）
- 接线为
  `gs.Provide(func(backends map[string]Discovery) *Manager, gs.IndexArg(0, gs.TagArg("?")))`
  ——与 resilience / loadbalance / fault 三个 authority 完全同构（谁拥有谁注册）；
  无后端 bean 时 map 为 nil = 空目录。
- Manager 挂进治理中心：`Center` 加 `disc *discovery.Manager` 字段 + `Discovery()`
  访问器，`NewCenter` 第五参是该 `*discovery.Manager`。**discovery 不进治理语义**
  （规则文档 / `adopt` / `dispatch` / `Config` schema 一律不碰）——Center 只持有不裁决。
  此拍板明知语义不纯仍要单入口，**别再翻案**。（2026-10-03）
- client 全部改走 `center.Discovery().Get(label)` 查表：map 注入型（http-client
  `newRoute`、gateway `newRouteTable`、grpc）持有 `*discovery.Manager`；按名注入型
  （mongodb / elasticsearch / neo4j / memcached / go-redis / redigo / gorm）收
  `discoveryLabel string`，ctor 内 `disc, _ := center.Discovery().Get(discoveryLabel)`；
  参数名一律 `discoveryLabel`（避免与治理 service `label` 局部变量重名）。
- 集合注入对多入口 client 仍是合法形态（`map[string]discovery.Discovery` +
  `TagArg("?")`，http-client 范式）；下游 client 已收口到
  `center.Discovery().Get(label)`。

## 新鲜度，以及 resolver 为什么不分策略

- `Discovery` 接口只留 `Resolve`（快照读，首次调用播种）；`Watch` / `WatchResult` /
  channel 契约删除，新鲜度机制全部下沉到各后端内部——别提议恢复 `Watch` 接口。
  （2026-09-04）
- 后端新鲜度选型：事件推送（etcd `clientv3.Watch` / nacos `Subscribe` 回调刷缓存）、
  读穿 TTL（DNS 类，无 goroutine）、informer（k8s endpointslice，`Close` 停全部）、
  blocking-query 循环（consul）。
- `discovery.Resolver` 是函数类型 `func() ([]Endpoint, error)`；构造器
  `NewResolver(ctx, backend, name, opts)` 绑 backend+name 一次、同步 `Resolve` 播种
  fail-fast、mesh/无名返回 `(nil,nil)`；**零资源无 Stop**。
- **Resolver 不做端点选择。** `Pick` 与内部 mini-RR 已删，选择全归
  `loadbalance.Pool`。语义分工：`resolver`（建连，慢时间尺度）与
  `loadbalance.Pool`（每请求，快时间尺度）两套选择器——连接池 client 用前者，
  RPC/httpx 用后者；将来 infra client 需按权重建连应在内部升层用 loadbalance，
  **不给 Resolver 加策略**。（2026-09-04）
- `Pool` 直接收 `discovery.Resolver`：`NewPool(src discovery.Resolver, bal Balancer,
  opts ...PoolOption)`，`Pool.Pick` 传播源错误。没有 `EndpointSource` / `SourceFunc`
  （已删）；`discovery.Allows` 早已移成 loadbalance 私有 `admission`。
- 连接池消费形态：`NewPool(resolver, bal)`，拨号闭包里 `pool.Pick(PickInfo{})`；
  gormcore 提供 `Common.NewResolver` / `NewPickPool` 共享函数。
- 「cloud 无 spring 依赖」从铁律降级为默认倾向（用户明示可突破）。

## 后端 starter 约定

- discovery 后端是 IoC 容器的**命名 bean**（bean 名 = 配置 label 原文，不加前缀），
  不是 `cloud/discovery` 里的全局 map；`RegisterDiscovery` / `GetDiscovery` 已删。
  （2026-09-08）
- `NewResolver(ctx, d Discovery, name string, opts...)` 收注入实例（d nil/无名/mesh →
  `(nil,nil)`）。
- 新后端 starter 照抄 `starter-discovery-etcd`（文件是 `discovery.go` + `registry.go`）：
  `r.Provide().Name()` + bean Destroy；label 配了但注入 nil 必须 fail-loud 列出可用名。
- 各后端非导出实现类型名（`etcdRegistry` / `zkRegistry` / `consulRegistry` /
  `nacosRegistry`）与测试假件不必镜像接口名（先例 `staticBackend implements Discovery`）。
- `Registry.UpdateWeight(ctx, inst, weight)` 签名陷阱：etcd 实现里 `inst` 只用于定位，
  其余字段连同 `inst.Weight` 被忽略——要么收成身份参数，勿以现状为准。

## registry 家族配置（终态 v2，旧单块方案已废弃）

- 命名空间是 **`spring.discovery.*`**（不是 `spring.registry.*`——后者在代码里零命中）。
  命名块：`spring.discovery.<backend>.<name>.*`，**无默认块**（与 client starter 多实例
  约定一致；默认块 + 命名块同层会因标量/子树不可判定而不可行）。（2026-09-09）
- 每块一个 backend bean，名 `<backend>.<name>`，**同 bean 同时实现 `discovery.Registry`
  + `discovery.Discovery`**（一个 bean 两种 Export），连接/探活/key-prefix 或 namespace
  单源。
- `discoveryServer` 是唯一 `gs.Server`，注入
  `Registries []discovery.Registry  autowire:"?"` 切片收集**跨后端全部** registrar：
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
- **「只做发现」成员判别标志：** 不 import registry 核心、不提供 `discovery.Registry`、
  不读 `${spring.discovery.service-name}` / `.addr`。该例外在三处登记别再当漏配
  （`scripts/check-observability.sh` 的 `NO_REGISTRAR="k8s"`、`starter/DESIGN{,_CN}.md`
  §3/§4），也别提议「补一个 registrar」；只做发现的后端仍要上报读侧（`obsSystem` +
  成功/失败两侧 `discovery.Synced`）。
- etcd 与 zookeeper 各自持有逐字重复的 `instanceValue` JSON struct（及 `withDimensions`），
  **不合并**：wire format 是每个后端存储的冻结兼容面，共享类型会把两个格式的演进耦合
  （为一个后端加的字段会静默变另一个后端的存量数据）。见到相似不要提议下沉；同理
  `config`（绑定）、`discovery.Instance`（领域）、`instanceValue`（存储）三层「形似」
  是各司其职。

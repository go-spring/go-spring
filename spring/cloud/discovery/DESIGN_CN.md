# discovery 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`discovery` 是零依赖、框架无关的客户端服务发现基础层。它定义每家命名服务需满足
的最小契约,以及所有基础设施客户端共用的管件:一次适配,Redis / MySQL / MongoDB /
Kafka / gRPC 通吃。

## 1. 职责与边界

- **做:**
  - 读侧 —— `Discovery`(`Resolve` + `Watch`)、`Endpoint`、`WatchResult`、可选的
    `Catalog`,以及 `Resolver`(有状态、靠 Watch 刷新的端点选择器)。
  - 包级读后端注册表(`RegisterDiscovery` / `GetDiscovery`);写侧(把本进程发布
    出去)住在 `starter-registry-*` starter 里,不在本包。
- **不做:**
  - **不做选择策略,不做流量反馈。** `Resolver.Pick` 只是最简 round-robin。策略
    (weighted / least-conn / consistent-hash / zone-aware)与失败摘除归
    `go-spring.org/spring/cloud/loadbalance`,它在 discovery **之上**。命名是共享的、
    按拓扑慢变速;选择与反馈是每消费者、每请求的。混在一起会让一个共享的注册中心
    读者被某个吵闹消费者的失败污染。
  - **不认识 mesh。** "我在不在 mesh 里"是部署问题,与名字解析正交;开关住在
    `go-spring.org/spring/cloud/mesh`。
  - **不做链路追踪。** 跨跳的 trace 传播在 `starter-otel`。
  - **不做 RPC 框架 provider 侧注册**(kitex / kratos / dubbo-go)。每个框架已有各自
    注册模型,再套一层只会变成翻译层——是负价值。
  - **不做 SDK 后端。** Nacos / Consul / etcd / DNS / Kubernetes 适配器会拖进各自的
    SDK 客户端,故住在各自的 starter 里、按名注册。唯一例外是
    [NewStaticDiscovery]:零依赖、固定快照的参考后端,留在本包作为最小实现示范,
    并供 example/test 复用。

## 2. 关键抽象

- **两方法 `Discovery`**(`Resolve` + `Watch`)让适配面最小。`Watch` 返回
  `<-chan WatchResult`;每个推送的是**完整快照**(不是增量),第一份是当前状态,
  ctx 是唯一生命周期——取消即关 channel。无 `Watcher` 句柄、无 `Stop`。不支持
  streaming 的后端内部轮询即可,契约统一。
- **`Endpoint` 三态可选性。** `Disabled`(运维/提供方指令——排空、维护、Nacos
  `enabled=false`)与 `Healthy`(探针结果)分开。候选集 = `!Disabled && Healthy`,
  无健康节点时退化到 `!Disabled`;`Disabled` 实例**永不**进任一集合。这防住了经典
  bug:运维 disabled 的实例被"无健康→用全部"的兜底重新拉回流量。
- **`Resolver` 是纯决策层。** 一次显式 `Resolve` 播种(同步、fail-fast),后台
  `Watch` 刷新,round-robin 在候选里选。它**故意不拨号**——socket、连接池、重试、
  死连接驱逐都归客户端。拨号级 failover(若需要)归客户端或 helper,不归 resolver。
  这和 `loadbalance.Pick` 一样:决策但不建连。
- **`Catalog` 可选,作为独立接口。** 有的后端枚举不了服务名(DNS、静态适配器、按名
  访问的 k8s headless Service)。把枚举塞进 `Discovery` 会逼这些后端用空列表或 panic
  撒谎。消费方按需类型断言 `d.(Catalog)`。
- **包级读注册表 + init 期 panic**,与 driver-registry 惯用法同构(如 starter-go-redis
  `RegisterDriver`)。空名 / nil / 重复注册是接线 bug,不是运行期状态。`GetDiscovery`
  返回**带候选列表的可读错误**,拼错名或漏装 starter 在构造时一目了然。

## 3. 不变量

- 后端与 resolver 必须并发安全;`Resolver` 用 `atomic.Pointer[[]Endpoint]` 存快照、
  `sync.Once` 保护 `Stop`,故 `Stop` 可与 `Pick` 并发、也能挂 bean 析构。
- `Resolver.Pick` 优先 `!Disabled && Healthy`;无健康节点则退化到 `!Disabled`;
  `Disabled` 永不可选。discovery 不能因后端不上报健康就黑洞流量。
- `Watch` channel 在 ctx 取消、或后端发出终结性 `WatchResult.Err` 时关闭;消费方停止
  range、保留最后一份快照继续服务——陈旧地址也比没有强。退避重连(若需要)是调用方
  的事,不归 `Watch`。
- 读后端注册表(`discoveries`)由自己的锁保护;本包无其它状态触及它。

## 4. 权衡与放弃的方案

- **只做 client 侧。** 本包只管命名:把名字解析到活地址。把本进程注册到注册中心归
  `starter-registry-*` starter(etcd/nacos/consul/zookeeper),RPC 框架 provider 注册
  按 §1 保持框架原生,两者都不在本包。kitex `registry.Registry`、kratos
  `registry.Registrar`、dubbo-go 配置化注册、go-zero `discov.EtcdConf` 差异足够大,
  再套一层就是翻译,故不强加。
- **`Resolver.Pick` 只做最简 round-robin,不做 weighted / 一致性哈希。** 策略归上一层;
  discovery 保持窄职责,避免与 `loadbalance`(策略 + 摘除)重叠。
- **`Watch` 用 channel,而非 pull 式 `Watcher.Next`。** `<-chan WatchResult` 让 ctx 成为
  唯一生命周期控制,首份 send 即种子(省掉 Resolve+Watch 双步舞),且是 Go 最眼熟的
  流式形态(`for r := range ch`)。错误作为 `WatchResult.Err` 在 channel 上传递。
- **mesh 开关与 trace 传播放在本包之外。** 早期草稿把 mesh 分支放在 resolver 里、把
  trace 接缝放在本包;都移走了——两者都不是 discovery 的本职,而且 mesh 分支单凭一个
  服务名根本凑不出可拨的地址。

# discovery
[English](README.md) | [中文](README_CN.md)

`discovery` 是零依赖、框架无关的**客户端**服务发现抽象。它回答基础设施客户端
(Redis / MySQL / MongoDB / Kafka ...)的一个问题:*"给一个逻辑服务名,当下可连
的 host:port 有哪些?"*——以及对称的写侧问题:*"把本进程发布到注册中心。"*

它**只管命名**。选择策略(round-robin / 加权 / 一致性哈希 / zone-aware)和流量反馈
(失败摘除)在上一层 `go-spring.org/spring/cloud/loadbalance`;mesh 开关在
`go-spring.org/spring/cloud/mesh`;链路追踪传播在 `starter-otel`。本包不碰策略、
不碰 mesh、不碰 trace,正是"一次适配、所有客户端通吃"的前提。

## 特性

- **读侧** —— `Discovery` 接口:`Resolve`(一次性快照)+ `Watch`(一个
  `<-chan WatchResult`,推完整快照;ctx 是唯一生命周期——取消即停)。
- **`Endpoint{Addr, Scheme, Weight, Disabled, Healthy, Metadata}`** —— 消费方取的值
  类型。`Disabled`(运维/提供方指令)与 `Healthy`(探针结果)分开:被 disabled 的
  实例永不被选中,连兜底也不进。
- **`Resolver`** —— `Resolve` 的有状态版:一次 `Resolve` 播种、后台 `Watch` 刷新、
  round-robin 选一个活端点。它是**纯决策层,不拨号**;socket 和连接池归客户端。
- **可选 `Catalog`** —— 独立接口(`Services`),给能枚举服务名的后端用;枚举不了
  的后端不实现即可。
- **写侧** —— `Registrar` 接口(`Register` / `Deregister`)和
  `Instance{ServiceName, ID, Addr, Scheme, Weight, Disabled, Metadata}`,把本进程
  发布到注册中心(Nacos / Consul ...)。
- **包级注册表** —— `RegisterDiscovery` / `GetDiscovery` 与 `RegisterRegistrar` /
  `GetRegistrar`,带可读的 not-found 错误。

## 用法

在一处适配公司命名服务:

```go
import "go-spring.org/spring/cloud/discovery"

type myBackend struct{ /* 命名服务客户端 */ }

func (b *myBackend) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	/* 返回当前快照 */
}
func (b *myBackend) Watch(ctx context.Context, name string) (<-chan discovery.WatchResult, error) {
	/* 每次拓扑变更推一份完整快照;ctx 取消时关闭 channel */
}

func init() { discovery.RegisterDiscovery("default", &myBackend{}) }
```

在基础设施客户端里消费:

```go
d, err := discovery.GetDiscovery("default")
if err != nil { return err }
r, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }
defer r.Stop()

ep, err := r.Pick()               // 一个当前存活的端点
if err != nil { return err }
conn, err := net.Dial("tcp", ep.Addr)   // socket 和连接池归客户端
```

把本进程注册到注册中心(通过 starter):

```go
r, _ := discovery.GetRegistrar("consul")
_ = r.Register(ctx, discovery.Instance{
	ServiceName: "orders",
	Addr:        "10.0.0.5:8080",
	Metadata:    map[string]string{"zone": "us-east-1a"},
})
```

设计细节见 [DESIGN_CN.md](DESIGN_CN.md)。

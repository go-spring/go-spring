# discovery

[English](README.md) | [中文](README_CN.md)

`discovery` 回答基础设施客户端(Redis / MySQL / MongoDB / Kafka ...)的一个问题：
*"给一个逻辑服务名，当下可连的 host:port 有哪些？"* 命名服务适配一次，所有
客户端消费同一契约。只做读侧——把本进程注册到注册中心是 `starter-registry-*`
starter 的活。

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

// d 是 starter 注入的发现后端 bean(bean 名=后端标签，如由 ${spring.registry.etcd.main} 派生的 "etcd.main");
// nil 表示"未启用发现"。
load, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }                    // fail-fast:构造时没有端点直接报错
if load == nil { return err }                    // "不生效"(无后端/无名/mesh):直接拨配置地址

eps, err := load()                               // 实时快照,错误如实上抛
if err != nil { return err }
conn, err := net.Dial("tcp", eps[0].Addr)        // socket 和连接池归客户端
```

## Discovery:后端契约

```go
type Discovery interface {
    Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)
}
```

- `Resolve` 返回当前快照。某个服务的第一次调用可能阻塞(播种查询,由 ctx
  约束);之后的调用是廉价读——新鲜度在后端内部:它用注册中心自己的通知
  机制(watch / 订阅 / 轮询)维持缓存最新。
- 后端是 IoC 容器里的命名 bean(各注册中心 starter 由自己的 `${spring.registry.<backend>.<name>}`
  配置块派生，bean 名为 "<backend>.<name>"，如 "etcd.main");client 在配置里写标签，starter 按名注入该 bean。
  容器就是发现目录——标签重复、拼错在装配期即报错。

没有注册中心？用内置的 static 后端:

```go
d := discovery.NewStaticDiscovery(
    discovery.Endpoint{Addr: "127.0.0.1:6379", Scheme: "tcp"},
)
```

## Endpoint:可选性

```go
type Endpoint struct {
    Addr     string            // host:port
    Scheme   string            // "tcp"/"" 明文,或 "tls"、"grpc" ...
    Weight   int               // 由 loadbalance 消费
    Disabled bool              // 运维/提供方指令:排空、维护
    Healthy  bool              // 探针结果
    Metadata map[string]string
}
```

`Disabled` 与 `Healthy` 是两个独立维度，可选性顺序有讲究:

```
优先选: !Disabled && Healthy
   一个健康的都没有? 退化到: !Disabled
   永不: Disabled —— 连兜底也不进
```

这防住了经典 bug:运维 disabled 的实例被"无健康→用全部"的兜底重新拉回流量。

## 选项:收窄查询

```go
eps, _ := d.Resolve(ctx, "orders", discovery.WithScheme("grpc"), discovery.WithTag("v2"))
```

| 选项 | 语义 | 谁来生效 |
|---|---|---|
| `WithScheme(s)` | 限定到单一传输 scheme;空 scheme 与 `"tcp"` 等价 | 所有后端,经 `FilterByScheme` |
| `WithTag(t)` | 注册中心原生标记(Consul service tag、registry label) | 注册中心支持 tag 的后端在查询侧过滤;其余忽略 |

两者传空都是 no-op——配置值可以无条件透传。

## Resolver:按名绑定的消费方

```go
load, err := discovery.NewResolver(ctx, "default", "orders-redis")
bal, _ := loadbalance.New(loadbalance.RoundRobin)
pool := loadbalance.NewPool(loadbalance.SourceFunc(load), bal)
ep, err := pool.Pick(loadbalance.PickInfo{})
```

- `NewResolver` 把后端标签 + 服务名(加上选项)一次性绑定，用一次同步 `Resolve`
  播种(fail-fast);名字为空或 mesh 模式开启时返回 `(nil, nil)`——"发现不生效"，
  调用方直接拨配置地址。
- 每次调用 resolver 都重读后端快照并如实上抛错误——对带缓存的后端就是一次廉价
  内存读,不掩盖注册中心抖动。
- 端点选择——round-robin、权重、一致性哈希、失败摘除——全在上一层
  [`loadbalance`](../loadbalance/README_CN.md),它把 resolver 经
  `loadbalance.SourceFunc` 收作端点源。发现本身不携带选择策略,resolver 也不持有
  任何资源——新鲜度全在后端内部,没有什么可 Stop。

想自己管理端点集？需要快照时直接 `Resolve`:

```go
eps, _ := d.Resolve(ctx, "orders", discovery.WithScheme("grpc"))
```

## 写一个后端

```go
type myBackend struct{ /* 命名服务客户端 */ }

func (b *myBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
    q := discovery.NewQuery(name, opts...)
    // 读缓存快照(首次调用查一次注册中心播种,之后用注册中心自己的
    // watch/订阅/轮询机制保持新鲜);
    // 用 discovery.FilterByScheme(raw, q.Scheme) 过滤;
    // 注册中心支持 tag 则在查询里带上 q.Tag
}

// 在 starter 的模块接线里:
r.Provide(func() (discovery.Discovery, error) { return &myBackend{}, nil }).Name("default")
```

要求并发安全;带 SDK 的适配器(Nacos / Consul / etcd / DNS / Kubernetes)住在
各自的 starter 里,本包保持零依赖。

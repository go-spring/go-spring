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

d, err := discovery.GetDiscovery("default")     // starter 注册的后端
if err != nil { return err }

r, err := discovery.NewResolver(ctx, d, "orders-redis")
if err != nil { return err }                    // fail-fast:构造时没有端点直接报错
defer r.Stop()

ep, err := r.Pick()                             // 按 round-robin 选一个可选端点
if err != nil { return err }
conn, err := net.Dial("tcp", ep.Addr)           // socket 和连接池归客户端
```

## Discovery:后端契约

```go
type Discovery interface {
    Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)
    Watch(ctx context.Context, name string, opts ...Option) (<-chan WatchResult, error)
}
```

- `Resolve` 返回当前快照——冷启动时调一次。
- `Watch` 返回快照 channel;第一份立即送达、即当前状态，之后每份都是完整
  替换(绝不是增量)。取消 ctx 即关 channel;后端的终结性错误以
  `WatchResult.Err` 送达、随后 channel 关闭——拿着最后一份快照继续服务
  (陈旧地址也比没有强)。
- 后端按标签注册(`RegisterDiscovery("default", b)`);`GetDiscovery` 按标签
  解析，错误信息会列出全部已注册名，拼错名或漏装 starter 在构造时一目了然。
  空名 / nil / 重复注册直接 panic——那是接线 bug。

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

## Resolver:现成的消费方

```go
r, err := discovery.NewResolver(ctx, d, "orders-redis")
defer r.Stop()
ep, err := r.Pick()
```

- 一次同步 `Resolve` 播种(fail-fast),后台 `Watch` 刷新——快照始终新鲜,
  你这边不用轮询。
- `Pick` 在候选集上做朴素 round-robin。权重、一致性哈希、失败摘除——都在
  上一层 [`loadbalance`](../loadbalance/README_CN.md),它把 `Resolver` 包成
  自己的端点源。
- 并发安全;`Stop` 可与 `Pick` 并发调用,也可挂在 bean 析构里。

想自己管理端点集？直接 Watch:

```go
ch, _ := d.Watch(ctx, "orders", discovery.WithScheme("grpc"))
for res := range ch {
    if res.Err != nil { break }        // 保留最后一份快照,继续服务
    replaceAllEndpoints(res.Endpoints)
}
```

## Catalog:可选的枚举能力

能列出全部服务名的后端(网关路由、控制台)实现 `Catalog`;枚举不了的(DNS、
static、按名访问的 k8s headless Service)不实现即可:

```go
if c, ok := d.(discovery.Catalog); ok {
    names, _ := c.Services(ctx)
}
```

## 写一个后端

```go
type myBackend struct{ /* 命名服务客户端 */ }

func (b *myBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
    q := discovery.NewQuery(name, opts...)
    // 查注册中心;用 discovery.FilterByScheme(raw, q.Scheme) 过滤;
    // 注册中心支持 tag 则在查询里带上 q.Tag
}

func (b *myBackend) Watch(ctx context.Context, name string, opts ...discovery.Option) (<-chan discovery.WatchResult, error) {
    // 每次拓扑变更推完整快照(第一份立即送达);
    // ctx 取消时关闭;或送达 WatchResult.Err 后关闭
}

func init() { discovery.RegisterDiscovery("default", &myBackend{}) }
```

要求并发安全;带 SDK 的适配器(Nacos / Consul / etcd / DNS / Kubernetes)住在
各自的 starter 里,本包保持零依赖。

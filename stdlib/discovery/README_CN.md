# discovery
[English](README.md) | [中文](README_CN.md)

`discovery` 是零依赖、框架无关的**客户端**服务发现抽象。回答基础设施客户端
(Redis / MySQL / MongoDB / Kafka ...)的一个问题:"给一个逻辑服务名,当下
可连的 host:port 有哪些?"。RPC 框架自身的 provider 注册故意不在范围内;配套的
`Registrar` 面向"把本进程发布到注册中心"这一与传输无关的场景。

## 特性

- `Discovery` 接口:`Resolve`(冷启动一次快照)+ `Watch`(通过 `Watcher` 推送
  变更)。
- `Endpoint{Addr, Weight, Healthy, Metadata}` —— 下游消费的唯一值类型。
- 包级后端注册表(`Register` / `Get` / `MustGet`),每家公司命名服务只需一处
  适配。
- `LiveDialer` —— 冷启动 Resolve、后台 Watch,暴露 `DialContext` / `Dial`,
  形状匹配常见客户端 dialer 钩子(Redis / go-sql-driver/mysql / pgx /
  ClickHouse / mssql)。
- `Registrar`:面向 VM / 裸机场景把本进程写入注册中心(Nacos、Consul...);
  配套 `RegisterRegistrar` 注册表。
- 服务网格开关(`SetMeshMode` / `MeshMode` / `NewClientDialer`):sidecar 存在
  时把 discovery 与 LB 降级为透传。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

在一处适配公司命名服务:

```go
import "go-spring.org/stdlib/discovery"

type myBackend struct{ /* client */ }

func (b *myBackend) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) { /* ... */ }
func (b *myBackend) Watch(ctx context.Context, name string) (discovery.Watcher, error)      { /* ... */ }

func init() { discovery.Register("default", &myBackend{}) }
```

在基础设施客户端里消费:

```go
ld, err := discovery.NewClientDialer(ctx, "default", "orders-redis")
if err != nil { return err }
defer ld.Stop()

rdb := redis.NewClient(&redis.Options{
    Addr:            "orders-redis",   // 只是标签,dialer 忽略
    Dialer:          ld.DialContext,
    ConnMaxLifetime: 30 * time.Second, // 让老连接过期,新连接落到新地址
})
```

把本进程注册到 Consul / Nacos(通过 starter):

```go
r, _ := discovery.MustGetRegistrar("consul")
_ = r.Register(ctx, discovery.Registration{
    ServiceName: "orders",
    Addr:        "10.0.0.5:8080",
    Metadata:    map[string]string{"zone": "us-east-1a"},
})
```

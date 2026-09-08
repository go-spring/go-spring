# health

[English](README.md) | [中文](README_CN.md)

组件健康检查契约。数据库连接池、缓存客户端、消息队列连接这类组件构造一个
`Indicator` 并导出为 bean,收集方(如 `starter-actuator`)自动装配全部实现,
用于 readiness / startup / liveness 探针。

## 安装

```
go get go-spring.org/cloud
```

## 用法

### 贡献一个组件的健康检查

给一个 Redis 客户端贡献健康检查:

```go
import (
    "context"

    "github.com/redis/go-redis/v9"
    "go-spring.org/gs"
    "go-spring.org/cloud/actuator/health"
)

func newRedisHealth(name string, client redis.UniversalClient) *health.Indicator {
    return &health.Indicator{
        Name:  "redis:" + name,
        Probe: func(ctx context.Context) error { return client.Ping(ctx).Err() },
    }
}

func init() {
    gs.Provide(newRedisHealth, gs.ValueArg("cache"), gs.TagArg("cache"))
}
```

几点说明:

- `Name` 是该组件在健康报告里的 key(如 `redis:cache`、`mysql:orders`),
  应用内必须唯一。多实例客户端在名字里带上实例名,每个实例各贡献一个
  indicator,报告里就能分别看到每个实例的状态。
- `Probe` 返回 nil 表示健康,返回 error 表示故障及其原因。必须尊重
  `ctx` 的超时/取消,把 ctx 传给底层调用(如上面的 `client.Ping(ctx)`),
  否则一个卡死的依赖会拖住整个探针请求。
- 这里只负责"上报",不负责"暴露":starter-actuator 等收集方自动装配
  全部 `Indicator` bean 并聚合,决定暴露成什么端点、多久探测一次。

### 只参与部分探针

比如"启动时必须连上、启动后不再影响流量"的下游(预热数据源),只参与
startup 探针:

```go
&health.Indicator{
    Name:   "redis:" + name,
    Probe:  probe,
    Groups: []health.Group{health.GroupStartup},
}
```

`Groups` 声明该 indicator 参与哪些探针分组,可任意组合,如
readiness + startup、liveness + readiness。

### 可选依赖

可选依赖(如纯加速的缓存)标记为非关键:照常上报、报告里能看到故障,
但故障时聚合结果是 DEGRADED 而非 DOWN,不会摘除 Pod:

```go
&health.Indicator{Name: "redis:" + name, Probe: probe, Optional: true}
```

对应地,必需依赖故障时聚合结果为 DOWN,readiness 探针失败、Pod 被摘出
Service 端点但不重启——这正是把依赖检查放进 readiness 而非 liveness
的意义。

## 探针分组

分组与 Kubernetes 容器探针对应:liveness / readiness / startup。
未声明 Groups 的 indicator 由收集方按默认路由到 readiness + startup,不进
liveness,这样下游抖动不会触发 Pod 重启。liveness 必须在 Groups 里显式声明,
且只应用于自身状态检查,不要用于下游资源。

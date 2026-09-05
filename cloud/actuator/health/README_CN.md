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

只参与 startup 探针:

```go
&health.Indicator{
    Name:   "redis:" + name,
    Probe:  probe,
    Groups: []health.Group{health.GroupStartup},
}
```

可选依赖标记为非关键:照常上报,但故障时不会摘除 Pod:

```go
&health.Indicator{Name: "redis:" + name, Probe: probe, Optional: true}
```

## 探针分组

分组与 Kubernetes 容器探针对应:liveness / readiness / startup。
未声明 Groups 的 indicator 由收集方按默认路由到 readiness + startup,不进
liveness,这样下游抖动不会触发 Pod 重启。liveness 必须在 Groups 里显式声明,
且只应用于自身状态检查,不要用于下游资源。

# health
[English](README.md) | [中文](README_CN.md)

`health` 是零依赖、框架无关的健康检查抽象。会上报自身健康的组件(数据库连接池、
缓存客户端、消息队列连接...)实现 `Indicator` 接口并以 bean 导出;收集方
(如 `starter-actuator`)自动装配全部实现,组合成 readiness / startup / liveness
探针。

## 特性

- `Indicator` 接口:`HealthName`、`CheckHealth`、`HealthGroups`、`IsCritical`
  --身份、检查、探针路由、严重性。
- `Status` 状态:`StatusUp` / `StatusDown`。
- Kubernetes 探针分组:`GroupLiveness`、`GroupReadiness`、`GroupStartup`。
- `NewIndicator` 工厂,配 `WithGroups`(声明探针分组)与 `NonCritical`(上报但不
  拉低聚合)选项。不带选项时 indicator 为关键、不声明分组,收集方套用默认路由
  (readiness + startup,绝不含 liveness)。

## 安装

```
go get go-spring.org/spring
```

## 用法

不依赖收集方就能暴露组件健康:

```go
import (
    "context"

    "github.com/redis/go-redis/v9"
    "go-spring.org/gs"
    "go-spring.org/spring/cloud/actuator/health"
)

func newRedisHealth(name string, client *redis.Client) health.Indicator {
    return health.NewIndicator("redis:"+name, func(ctx context.Context) error {
        return client.Ping(ctx).Err()
    })
}

func init() {
    gs.Provide(newRedisHealth, gs.ValueArg("cache"), gs.TagArg("cache")).
        Export(gs.As[health.Indicator]())
}
```

只贡献 startup 探针(启动期强依赖):

```go
health.NewIndicator("redis:"+name, probe, health.WithGroups(health.GroupStartup))
```

把可选缓存标记为非关键--照常上报,但绝不把 Pod 摘出流量:

```go
health.NewIndicator("redis:"+name, probe, health.NonCritical())
```

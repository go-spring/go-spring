# health
[English](README.md) | [中文](README_CN.md)

`health` 是零依赖、框架无关的健康检查抽象。会上报自身健康的组件(数据库
连接池、缓存客户端、消息队列连接...)实现 `Indicator` 接口并以 bean 导出;
收集方(如 `starter-actuator`)自动装配全部实现,组合成 readiness / startup /
liveness 探针。

## 特性

- 单一必需接口 `Indicator { HealthName() string; CheckHealth(ctx) error }`。
- `Status` 状态:`StatusUp` / `StatusDown`。
- Kubernetes 探针分组:`GroupLiveness`、`GroupReadiness`、`GroupStartup`。
- 可选 `Grouped` 接口:只想参与部分探针时实现它;未实现时默认走
  readiness + startup(安全默认)。
- 可选 `Critical` 接口:返回 `IsCritical() false` 的指标仍会逐组件上报,但其 `DOWN`
  不会拉低聚合探针,因此"降级但可容忍"的依赖不会把 Pod 摘出流量。默认(未实现)为关键。
- `GroupsOf` / `InGroup` / `IsCritical` 辅助函数,让收集方按探针过滤并加权。

## 安装

```
go get go-spring.org/stdlib
```

## 用法

不依赖收集方就能暴露组件健康:

```go
import (
    "context"

    "go-spring.org/gs"
    "go-spring.org/spring/actuator/health"
)

type redisHealth struct {
    name   string
    client *redis.Client
}

func (r *redisHealth) HealthName() string { return "redis:" + r.name }

func (r *redisHealth) CheckHealth(ctx context.Context) error {
    return r.client.Ping(ctx).Err()
}

func newRedisHealth(name string, c *redis.Client) health.Indicator {
    return &redisHealth{name: name, client: c}
}

func init() {
    gs.Provide(newRedisHealth, gs.ValueArg("cache"), gs.TagArg("cache")).
        Export(gs.As[health.Indicator]())
}
```

只贡献 startup 探针(比如启动期强依赖):

```go
func (r *redisHealth) HealthGroups() []health.Group { return []health.Group{health.GroupStartup} }
```

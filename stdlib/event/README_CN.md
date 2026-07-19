# event
[English](README.md) | [中文](README_CN.md)

`event` 是进程内的发布/订阅事件总线,是 Spring `ApplicationEventPublisher` /
`@EventListener` 的 Go 惯用等价物。模块之间通过发布类型化事件通信,而不是相互
直接调用,让生产者与消费者保持解耦。

## 特性

- 零第三方依赖,属于 stdlib 基础层。
- 任意具体 struct 就是事件——无需 marker 接口或注解扫描。
- 类型安全的泛型订阅(`Subscribe[T]` / `SubscribeAsync[T]`);唯一的反射是一
  个类型键,把发布值路由到订阅该动态类型的处理器。
- 同步处理器按确定顺序在调用者协程中运行(`WithOrder`);错误用 `errors.Join`
  聚合,单个失败的订阅者不会静默吞掉其他。
- 异步处理器在独立的 buffered worker 协程中运行(`WithBuffer`、
  `WithErrorHandler`);慢处理器不会阻塞发布者。
- 优雅 `Close`:异步 worker 会先排空已缓冲事件再退出。
- nil / 空透传:无订阅者时 Publish 为 no-op;nil bus 上订阅返回 no-op cancel。
- 可选的容器集成 `Listener` 接口——bean 通过 Export 为 `event.Listener` 被收
  集,注册器在装配后调用一次 `Register(bus)`,与 `health.Indicator` 的 Export
  收集范式对齐。

## 快速开始

Import 路径: `go-spring.org/stdlib/event`。

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/stdlib/event"
)

type ConfigChanged struct{ Key, Value string }

func main() {
    bus := event.New()
    defer bus.Close()

    cancel := event.Subscribe(bus, func(ctx context.Context, e ConfigChanged) error {
        fmt.Printf("reload %s=%s\n", e.Key, e.Value)
        return nil
    })
    defer cancel()

    _ = bus.Publish(context.Background(), ConfigChanged{Key: "log.level", Value: "debug"})
}
```

容器托管的监听器实现 `event.Listener` 并 Export 该接口;注册器收集这些 bean
并在装配后调用 `Register(bus)`。每个 Export 出去的 listener bean 必须显式命
名,以避免多 bean 共用一个 Export 时的 `__default__` 冲突。

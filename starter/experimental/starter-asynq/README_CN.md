# starter-asynq

[English](README.md) | [中文](README_CN.md)

`starter-asynq` 为 Go-Spring 提供 [Asynq](https://github.com/hibiken/asynq)
支持：基于 Redis 的任务队列，含生产者 `Client`（入队）与可选启用的 worker
`Server`（出队 + 执行），每个 `spring.asynq.instances.<name>` 实例各一套。

## 安装

```bash
go get go-spring.org/starter-asynq
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-asynq"
```

### 2. 配置

```properties
spring.asynq.instances.a.addr=127.0.0.1:6379
spring.asynq.instances.a.concurrency=4
spring.asynq.instances.a.server.enabled=true
```

### 3. 注入

```go
type Service struct {
    Client *StarterAsynq.Client `autowire:"a"`
    Server *StarterAsynq.Server `autowire:"a:server"`
}
```

### 4. 使用

```go
// worker 启动前先注册 handler。
s.Server.RegisterHandler("example:greet", func(ctx context.Context, t *asynq.Task) error {
    return nil
})

// 生产者入队。
info, err := s.Client.Enqueue(ctx, asynq.NewTask("example:greet", payload))
```

## 核心特性

- **双角色、单实例** — `Client`（总是装配，入队带守卫）与 `Server`（仅
  `server.enabled=true` 时装配；长期运行的 worker 是显式 opt-in）。二者共
  享同一套 Redis 连接配置。
- **入队守卫** — `Client.Enqueue` 走治理执行器（限流/熔断）；未导入
  `starter-governance-file` 时退化为 `Client.EnqueueContext`。
- **优雅停机** — 销毁时 `Server` 按 `shutdown-timeout` 排空在飞任务。
- **健康指示器** — `asynq:<name>` 探针经新建 inspector ping Redis。

## 高级特性

**多队列** — 按队列声明优先级权重：

```properties
spring.asynq.instances.a.queues.critical=6
spring.asynq.instances.a.queues.default=3
```

**多实例** — 更多 `spring.asynq.instances.<name>` 条目，各自独立的生产者/worker。

## 可观测

starter **只声明**每次受守卫入队的身份（`observe.go`）：span 名 `enqueue`、
`messaging.system`/`messaging.operation` 标签，以及任务类型作 `messaging.destination.name`
的 span/日志 detail。信号本身由 resilience 层发射——它是 executor 链条上唯一看得见整次调用
（含重试）的点——所以除 call 级 `messaging.client.operation.duration` 直方图外，每次调用还多一条
attempt 级的 `messaging.client.attempt.duration` 直方图，外加 `messaging.client.active_requests`
gauge、`resilience.client.calls` 计数器，以及 tag 为 `_app_asynq_access` 的访问日志。未安装
`starter-otel` 提供的 provider 时全部为空操作。

## 设计说明

* **worker 是 `gs.Server`，不是 `gs.Runner`。** Go-Spring 的 `gs.Runner` 是启动期、禁止阻塞的接口；
  长期运行的消费者应归 `gs.Server`，因此 worker 不会阻塞启动、并能优雅停机。它用 asynq 的
  `Start` + 等 ctx，而非 `asynq.Server.Run`——后者自装的信号处理器会与 Go-Spring 的关机流程竞争。
* **handler 可在 worker 运行前的任意时刻注册。** `RegisterHandler` 可以在构造函数之后调用（例如从
  你自己的 `Init` 里）：mux 首次使用时惰性构建，并与运行中的 server 共享。
* **payload 是你自己的 `[]byte`。** 序列化、重试/队列策略（asynq `Options` 原样透传）与定时任务都是
  asynq 自身的事——通过 asynq 的 API 配置，而非 starter 配置。
* **panic 由 asynq 恢复，而非 starter。** asynq 的 processor guard 会恢复 handler panic；
  starter 刻意不二次包裹 handler。

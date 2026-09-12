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
  `starter-governance` 时退化为 `Client.EnqueueContext`。
- **优雅停机** — 销毁时 `Server` 按 `shutdown-timeout` 排空在飞任务。
- **健康指示器** — `asynq:<name>` 探针经新建 inspector ping Redis。

## 高级特性

**多队列** — 按队列声明优先级权重：

```properties
spring.asynq.instances.a.queues.critical=6
spring.asynq.instances.a.queues.default=3
```

**多实例** — 更多 `spring.asynq.instances.<name>` 条目，各自独立的生产者/worker。

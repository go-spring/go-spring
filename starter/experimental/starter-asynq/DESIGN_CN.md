# starter-asynq 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

[Asynq](https://github.com/hibiken/asynq)（Redis 任务队列）的 Client 原型
starter。一个实例产出生产者 Client（入队）与可选启用的 worker Server
（出队 + 执行）。

## 1. 职责与边界

- **负责**：`spring.asynq.<name>` 每实例装配、Redis 连接（双角色共享）、
  带守卫的入队路径、worker 生命周期（gs.Server）、handler mux、健康指示器。
- **不负责**：任务序列化（payload 是应用自己的 `[]byte`）、重试/队列策略
  （asynq Options 透传）、定时任务（asynq 自身的 cron，不在本 starter 范围）。

## 2. 关键抽象与 Seam

- **双角色、单 Config** — `Client`（总是装配）与 `Server`（仅
  `server.enabled=true`）。worker 是 opt-in：长期消费者是运维的显式决策，
  契合 no-autoconfig-exclude 立场（见 spring/DESIGN.md §4）。
- **`Client.Enqueue` 守卫** — 同步入队触 Redis、是过载敏感路径；经中立的
  `resilience.ExecutorFor` / `fault.InjectorFor` seam 与 starter 自带的生产
  观测（observe.go），治理关闭时退化为 `Client.EnqueueContext`。
- **`Server` 实现 `gs.Server` 而非 `gs.Runner`** — 这是承重 seam。gs.Runner
  是启动期、禁止阻塞的接口（迁移、缓存预热）；gs.Server 才是长期运行、
  优雅停止的服务接口。误用 gs.Runner 会阻塞启动并破坏信号处理。
  `Server.Run` 用 asynq 的 `Start` + 等 ctx，刻意不用 `asynq.Server.Run`——
  后者的 `waitForSignals` 自己装信号处理器，与 gs 的优雅关机竞争。
- **handler mux 惰性构建** — `RegisterHandler` 可能先于 `Init` 执行（应用
  从自己的 Init 里注册），因此 mux 首次使用时才创建，与 Init/Run 共享。

## 3. 约束

- `Queues` 需带 `:=` 默认（缺 `spring.asynq.<n>.queues` 键时必须绑定为 nil，
  不能使模块装配失败）。
- asynq 自身恢复 handler panic（其 processor guard）；本 starter 刻意不做
  二次包裹——边界写入文档而非双重执行。

## 4. 权衡 / 已否决的方案

- **asynq.Server.Run vs Start+ctx**：Run 内置的信号处理与 gs 冲突；否决，
  改用 Start + 等 ctx。
- **gs.Runner vs gs.Server**：Runner 阻塞启动；Server 才是正确的长期运行
  seam。这是本批最硬的 bug，记录于此以免未来 starter 重蹈。

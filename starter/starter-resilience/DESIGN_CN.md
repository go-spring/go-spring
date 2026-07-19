# starter-resilience 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-resilience` 属于 **global / infrastructure** 形态(见
[starter/DESIGN.md](../DESIGN.md) §2.4),把
[alibaba/sentinel-golang][sentinel] 注册为 `stdlib/resilience` 的推荐 driver。
不注册 bean、不开端口;任一适配器只要空导入本 starter,就能选 `driver=sentinel`。

[sentinel]: https://github.com/alibaba/sentinel-golang

## 1. 职责与边界

- **在范围内:**import 时调用 `sentinel.InitDefault()` 并
  `resilience.RegisterDriver("sentinel", ...)`;把中立
  `resilience.Policy` 按 resource 翻译成 sentinel 规则。
- **不在范围内:**决定韧性*施加在哪里*——那是适配器的活。`stdlib/resilience`
  提供三种 seam:HTTP 客户端 `NewRoundTripper`、连接拨号 `NewDialer`、
  HTTP 入站 `NewHandler`;本 starter 从不在其中选择。

## 2. 关键决策

- **没有"单一通用 per-request seam"。**各 client 库钩子不同(oauth2 →
  `http.RoundTripper`,go-redis → `redis.Hook`,gorm → plugin callback,
  MQ → call-site helper)。`stdlib/resilience` 保留中立
  `Executor.Execute(ctx, resource, fn)`,让每个适配器桥接到自家形态。
  本 starter 提供*引擎*,不提供*缝隙*。
- **`Policy` → sentinel 规则按 resource 懒加载。**sentinel 按 resource 名
  索引,规则在首次 `Entry` 时载入;并发首触由 `sync.Mutex` + `loaded`
  map 守护。
- **`MaxRetries` 与 `Timeout` 在 sentinel `Entry` 之外应用。**sentinel
  本身不建模这两者,executor 在其外层包装。`ctx.Err()` 会提前中断重试
  循环,避免被取消的请求耗尽预算。
- **Block 原因映射为中立 sentinel。**`BlockTypeCircuitBreaking →
  ErrCircuitOpen`、`BlockTypeIsolation → ErrBulkheadFull`、缺省
  `→ ErrRateLimited`。调用方仅依赖 `stdlib/resilience`;sentinel 是
  starter 侧细节。

## 3. 约束

- **import 期 init。**`sentinel.InitDefault()` 失败即 panic,让环境异常
  在启动时暴露,而不是首次调用时暴露。
- **不跑 `go mod tidy`。**内部依赖(spring、stdlib)靠 `go.work` 解析;
  tidy 会去 proxy 404。
- **sentinel 版本锁死 v1.0.4**——后续小版本调整了 flow / circuitbreaker
  规则字段;上游回归应在此处发现,而非每个适配器都受牵连。

## 4. 零依赖兜底

`stdlib/resilience` 内建 `default` driver(令牌桶 + 连续失败熔断 + 重试 +
超时,零三方依赖),让框架开箱即用、测试无需拉 sentinel。本 starter 的
价值体现在需要 sentinel 自适应流控与可调熔断的生产链路。

## 5. 取舍 / 弃选方案

- **让 `stdlib/resilience` 依赖 sentinel——弃选。**四层规则要求基础层零
  依赖;本 starter 是一种具体实现,而非抽象。
- **给所有库来一个统一的 dialer / RoundTripper seam——弃选。**`LiveDialer`
  是唯一真通用的 seam,但只覆盖建连;per-request 钩子只能停在各库自选之处。
- **MQ 用 `otelsarama` 之类上游 wrapper——弃选。**这类 wrapper 已废弃 /
  锁定于特定客户端版本;call-site span helper 老化更慢(见姊妹条 MQ 可
  观测决策)。

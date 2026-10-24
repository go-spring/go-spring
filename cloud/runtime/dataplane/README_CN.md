# dataplane

[English](README.md) | [中文](README_CN.md)

数据面分为两棵独立的树——[outbound](outbound/) 与 [inbound](inbound/)——共用方向中立的
[kernel](kernel/)。本页记录横跨两棵树的一条边界：**可观测的划分**。

## 两个观测面

可观测不是一个层，而是两个面，所有者、生命周期和输入都不同：

```
                  请求面                              器官状态面
              (每次调用/请求)                      (每个器官实例)

  调用方 ──▶ observe ──▶ 治理器官 ──▶ 传输
                │              │
                │              └── 自己发布自己的状态（拉取）
                └── 记录这一次调用的结果
```

| | 请求面 | 器官状态面 |
| --- | --- | --- |
| 问题 | **这一次调用**发生了什么：状态、耗时、错误类型、值不值得记录？ | **这个器官**现在持有什么状态？ |
| 输入 | 调用的结果与 `Recorder` | 器官自己的字段 |
| 生命周期 | 一次调用 | 器官实例（`Close` 时反注册） |
| 发布方式 | 每次调用（span、指标、日志） | 每次采集，拉取（observable 仪器） |
| 所在包 | `outbound/observe`、`inbound/observe` | `outbound/resilience`、`inbound/admission`、sentinel starter、`kernel/loadbalance`、`kernel/discovery` |

## 规则

> **信号归属于它生命周期的所有者。** 随一次调用生灭的，归请求面；描述进程级器官状态的
> （熔断状态、预算用量、槽位占用、端点挂起），由器官在自己的包里发布。

两条推论都是硬约束：

1. **请求面从不读取治理状态。** 它只记录调用返回了什么。被拒调用体现为
   `resilience.error_type=circuit_open`（及其同类），这是请求级事实，不是读熔断器。
2. **内核不 import 任何 observability SDK。** 内核只暴露只读访问器
   （`Breaker.Stats`、`Budget.Usage`、`Bulkhead.Stats`）；持有它的器官自己注册
   observable 回调，并在 `Close` 时反注册。不存在跨层状态 listener，
   `chain.Executor` 上也没有状态查询方法。

## 谁发布什么

请求面（既有）：

| 指标 | 含义 |
| --- | --- |
| `resilience.client.duration`、`.calls` | 出站耗时与结果；保护拒绝时带 `resilience.error_type` |
| `*.operation.duration`、`*.attempt.duration` | 按 operation 的整体/单次尝试耗时 |
| `resilience.server.duration`、`.calls` | 入站耗时与结果 |
| `*.request.duration`、`*.active_requests` | 入站单路由耗时与在途请求数 |

器官状态面：

| 指标 | 所有者 | 属性 |
| --- | --- | --- |
| `resilience.client.breaker.state`（1-hot）、`.transitions_total{to}` | 出站 resilience | `service`、`unit` |
| `resilience.client.ratelimit.usage_ratio` | 出站 resilience | `service`、`unit` |
| `resilience.server.ratelimit.usage_ratio` | 入站 admission | `system`、`service` |
| `resilience.server.bulkhead.usage_ratio`、`.queued` | 入站 admission | `system`、`service` |
| `loadbalance.endpoint.suspended`、`.suspension.total` | loadbalance | — |
| `discovery.cache.age_seconds`、`discovery.sync_total` | discovery | 取决于 observer |
| `discovery.instance.registered` | registration | 取决于 observer |
| sentinel：`resilience.client.breaker.state`、`.transitions_total{to}` | sentinel starter | `service` |

说明：

- 出站状态面没有 `system` 属性：一个 executor 由同一 label 的所有 client backend 共享，
  它不知道 system。system 是请求面的维度。
- 熔断的开始/结束时间就是 `state` gauge 在时序库中的边沿；"已打开超过 N 秒"用
  `state{state="open"} == 1` 加 `for:` 告警，不再单独设 age 指标。
- `transitions_total` 是每个 breaker 实例的累计值，策略换代重建后从 0 重新计数；
  请使用 `increase()`/`rate()`。

## 明确不覆盖

- **ClusterLimiter 的远端用量。** 配置了 store 时扣减的是 store，它没有用量查询接口，
  因此不发布本地用量；拒绝仍以每次调用的 `rate_limited` 体现。
- **注入故障的归因。** 请求面把注入故障记录成它模仿的真实错误，因此注入与真实故障在指标上
  不可区分。这是既有的已知缺口。
- **压测流量打标。** `traffic` 只负责传播标记；指标是否消费它由应用决定。

## 执行方式

本边界没有自动化测试，是设计契约：新增指标时必须说明它属于哪个面、服务于哪个决策或告警，
并按本页评审。

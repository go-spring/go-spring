# 放火：目标、范围与平台参考

## 这个包是什么

`fault` 是 go-spring 治理栈里的**进程内、调用级**故障注入器。它位于框架本来就在治理的
调用路径上，因此一次 drill 可以精确到一次服务调用、一个接口或一类流量；注入的失败会流经与
真实失败相同的重试、熔断、超时与负载均衡逻辑。

它与基础设施混沌平台（数据包级、节点级、云平台级、代理级）**互补**：那些平台作用在应用调用
路径看不到的层，而本包作用在应用与治理所在的层。

## 目标

**按需、可控、可归因地**制造下游异常，让治理策略与调用方容错路径在故障下可被观察，而不必
等待一次真实事故。

- **按需**：故障在演练要求时出现。
- **可控**：剂量、时长、影响面都有护栏。
- **可归因**：`RunID` 与规则身份进入 span、metrics、日志，演练流量可以与真实流量区分。

## 范围

### 注入什么：下游行为

放火制造的是"对端做什么"。

| 层 | 效果 |
|---|---|
| 传输 | `refused`、`reset`、`io-timeout`（读写截止，不是 context 截止） |
| 协议 | 带状态码的错误应答；入站 drill 还可以声明响应 body |
| 延迟 | 固定 / 抖动 / 慢响应 |
| 数据 | 入站可声明响应 body；出站不替换载荷 |

### 剂量与节奏

- `rate`（概率）或 `every`（确定性每 N 次调用）；
- `start-after`：从规则首次匹配起延迟整条规则；
- `on`/`off`：间歇窗口，用于"坏一阵再恢复"；
- `max-affected` / `max-duration`：爆炸半径与自愈护栏。

这些组合既能表达持续故障，也能表达有界故障——"坏 30 秒然后恢复"本身就是一等公民。

所有剂量字段都按 ATTEMPT 计数：出站调用重试 N 次就是 N 个真实下游请求，各自独立判定——
与 Envoy/Istio 的 per-request 模型一致。调用级失败率是这些 attempt 的组合。

### 如何选中调用

方向、服务/接口、metadata（含 traffic class）、endpoint 或实例。规则是有序列表，第一条
匹配者生效；每条规则必须点名 service，没有 catch-all。

### 两个方向

- **出站**：客户端接缝在 resilience 的重试循环内部注入，重试、熔断与负载均衡会真实反应。
- **入站**：服务端接缝在 handler 之前决策，由 adapter 写出声明的响应。

两个方向拥有各自独立的配置与护栏计数。

### 归因

每个 drill 携带 `RunID`；注入事实连同命中的规则与声明的效果，进入 span、metrics、日志。

## 边界

本包刻意停留在应用调用路径内：

- 不制造负载与并发，也不做资源压力；
- 不做基础设施状态混沌（Pod/进程/实例、DNS、时钟、主备切换）；
- 不做数据包级效果（丢包、损坏、分区、带宽整形）；
- 不操纵治理裁决（强制打开熔断、摘除 endpoint）；
- 不做编排——调度、停止条件与后置断言属于调用它的平台。

## 为什么它嵌入治理栈是合适的

- **治理层会真实反应**。放火落在重试循环内部，重试计数、熔断状态与 endpoint 健康度会像
  面对真实下游失败一样变化。
- **覆盖所有客户端，不只是 HTTP**。缓存、SQL、消息、RPC 共享同一个接缝。
- **说的是治理词汇**。传输失败、协议错误应答、I/O 超时被区分；入站 drill 声明的是响应
  状态码，而不是失败分类。
- **调用级精度**。可以只打一个接口、一类流量、一个租户——比路由或代理规则更细。
- **配置热更新**。规则推送后在原地切换，无需重启。
- **天然可归因**。`RunID` 与规则身份让演练信号在 trace、metrics、日志中与真实故障区分。

## 平台参考

比较前提：只对比**进程内注入器能实现**的效果，也只引用能力集能由应用调用路径表达的平台
（这里取 Envoy、Istio、Toxiproxy 作为 L4/L7 参照）。数据包级、资源级、状态级混沌位于调用
路径之下，不在本文对比范围内。

| 能力 | 主流参照（L4/L7） | 本包 |
|---|---|---|
| 按调用比例注入 | Envoy/Istio `percentage` | 有（`rate`） |
| 故障持续 / 窗口 | 注入持续到配置被移除 | 有（`start-after`、`every`、`on`/`off`、`max-duration`） |
| 按调用/路由/头匹配 | Envoy cluster/header；Istio header/path | 有（service/action/metadata） |
| 并发上限 | Envoy `max_active_faults` | `max-affected` |
| 延迟 + 抖动 | Envoy/Istio `delay`；Toxiproxy latency+jitter | 有 |
| 带状态码的错误应答 | Envoy/Istio `abort` | 入站：`status` + `body`；出站：status 挂在注入错误上 |
| 连接重置 / I/O 超时 | Toxiproxy `reset_peer`/`timeout` | 有 |

L4/L7 平台表达的是数据包与状态码；本包在此基础上补齐治理词汇，并能触达代理看不到的应用
客户端。

## 参考资料

- Envoy HTTP fault filter: https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/fault_filter
- Istio HTTPFaultInjection: https://istio.io/latest/docs/reference/config/networking/virtual-service/#HTTPFaultInjection
- Toxiproxy: https://github.com/Shopify/toxiproxy

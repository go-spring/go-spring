# loadbalance 设计
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`loadbalance` 在 discovery 快照之上添策略层与离群摘除。它是 stdlib(零依赖)
包,只 import `go-spring.org/spring/discovery`;RPC 框架适配(gRPC balancer /
kitex loadbalancer 等)在各自 starter 里把此内核翻译成框架的 picker。

## 1. 职责与边界

- **做:** 定义 `Balancer`,内置五种策略共用一张注册表,提供离群摘除
  `Tracker`,并用 `Pool` 把 discovery health、mesh 模式、tracker 摘除粘在一起。
- **不做:**
  - 不做发现。候选集合每次 `Pick` 都由外部喂入(通过 `EndpointSource`);本包
    不 Resolve / Watch。
  - 不写 RPC 框架适配代码。gRPC / kitex 适配在各 starter 里复用此处的策略与
    `Tracker`。
  - 不依赖 `resilience`。熔断语义在 `Tracker` 里刻意重写 —— 见 §4。

## 2. 关键抽象与缝隙

- **`Balancer.Pick(eps, info)` 每次都收候选集合。** balancer 只持有选择态
  (rr 游标 / hash 环 / SWRR current-weight / least-conn 在途),discovery 与
  摘除由调用方持有。故策略可组合(`zone_aware` 内嵌另一个 `Balancer`),拓扑
  抖动时同一 balancer 可持续使用。
- **注册 `Factory`(不是 `Balancer` 实例)。** balancer 有可变的 per-target
  状态,每 target 必须独立实例;注册工厂强制这一点。
- **`Result.Done` 闭环请求生命周期。** `least_conn` 在 `Done` 中递减在途;
  `Pool` 会包一层 `Done`,让 `Tracker` 也能看到 rr / hash / weighted 这种自
  身无 `Done` 的策略结果。调用方必须对非 nil `Done` 调用一次且仅一次。
- **`Tracker` 有意做成可查询。** 暴露 `Eligible(eps)` 与 `Ejected(addr)`,让
  `Pool` 在路由**之前**就把坏实例剔除。`resilience.Executor` 只在调用时
  reject 且不可查询 —— 是熔断该有的形状,但不适合 LB 层前置过滤。
- **`Pool` 合并两种健康信号:** `discovery.Endpoint.Healthy` 与
  `Tracker.Eligible` 顺序应用;两级都在集合被过空时回退到输入。最终
  `Pool.Pick` 的 `Done` 被包了一层,自动喂 Tracker。

## 3. 不变量

- Balancer 与 `Tracker` 必须并发安全。
- `healthy(eps)`、`Tracker.Eligible` 都不允许在 `eps` 非空但被过滤到空时返回
  空集 —— 探一次退化实例总好过全流量黑洞。
- `Threshold <= 0` 的 `Tracker` 是透明透传(`Eligible` 返回输入,`Record` no-op),
  接线始终零开销直到配了摘除。
- 网格开关在 `Pool.Pick` 处一次处理,不进各 balancer,所有策略统一降级。
- 策略名(`RoundRobin` / `LeastConn` / `ConsistentHash` / `Weighted` /
  `ZoneAware`)会出现在服务配置和 gRPC LB config 里,故稳定不改。

## 4. 权衡与放弃的方案

- **熔断语义与 `resilience` 重复,不复用。** resilience Executor 按 resource
  键、只在调用点 reject、不可查询;`Tracker` 按 endpoint 地址键且必须可查询
  才能让 `Pool` 前置过滤。复用会形状错配;共享同一份 `DoneInfo.Err` 信号即可
  保持一致,不必产生代码依赖(中心定义、边缘桥接)。
- **候选集合每次注入,不在 balancer 里缓存。** discovery 已经持有快照,再
  缓存一次是重复状态,拓扑变化时更容易出错。
- **`consistent_hash` 用与顺序无关的指纹**(每 addr hash XOR + 长度)判断是否
  重建环,故 discovery 快照重排不动环 —— 因为快照本身不保证顺序稳定。
- **`weighted` 每次 Pick 清理 state map 里失效的地址**,避免拓扑抖动累积
  权重残留。

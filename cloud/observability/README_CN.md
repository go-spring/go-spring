# observability

[English](README.md) | [中文](README_CN.md)

`cloud/observability` 是**跨平面的遥测词汇**,既不是一个平面,也不是能力 SDK。
它装着两个平面与所有能力都可以使用、但没有任何一方拥有的部件:

| 包 | 回答什么问题 |
| --- | --- |
| [attrs](attrs/) | 挂在 `context.Context` 上的每请求属性(框架代你启动的 span 也能读到),以及全仓唯一一份 duration 直方图分桶定义。 |
| [conf](conf/) | `conf.Refresh`:应用级属性刷新的共享插桩漏斗(指标 + 那条舰队级日志)。 |

两者都不 import `chain` / `guard` / `breaker` / `resilience`,所以 cache、lock、
scheduling、配置 starter、控制面 registration 使用它们时,不会把治理栈拖进
依赖图。

本目录刻意**不放在** `governance/dataplane/` 下:受治理调用插桩(它才是数据面,
而且是最外层)位于 [outbound/observe](../governance/dataplane/outbound/observe/) 与 [inbound/observe](../governance/dataplane/inbound/observe/),
出站包装 `chain.Executor`、入站驱动 admission 的 Admit/Done;两侧的 observe 子包分别在 `outbound/observe` 与 `inbound/observe`。

依赖规则:顶层中立词汇——`propagate`、`mesh`、`observability/attrs`、
`observability/conf`——可被两个平面和任何能力依赖;平面之间只允许
控制面 → 数据面,且永不反向依赖能力 SDK。

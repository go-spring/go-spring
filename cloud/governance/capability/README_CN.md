# capability

[English](README.md) | [中文](README_CN.md)

`capability` 是治理运行时的**平面中立体**：既不属于控制面、也不属于数据面，但两个平面都消费的机制与概念。控制面的文档嵌入它们的类型，数据面的方向树用它们执行。

## 判定规则

> 当一个包回答的是状态、词汇或机制问题——地址、端点、失败状态、预算、流量类别、服务命名、匹配——并且既不知道调用的方向、也不知道文档的来源时，它属于 `capability`。

一旦代码开始说 `Executor`、`Operation`、`Recorder`、`Request`、`Admit`、`Done`，它就知道自己的方向，归入 [../dataplane/outbound](../dataplane/outbound/) 或 [../dataplane/inbound](../dataplane/inbound/)；一旦它负责获取、校验或分发文档，归入 [../controlplane/](../controlplane/)。

这也是一条依赖不变量：

> **`capability` 不 import `controlplane/`，也不 import `dataplane/`；两个平面都可以 import `capability`。**

## 内容

| 包 | 回答什么 |
| --- | --- |
| [circuitbreaker](circuitbreaker/) | 这条路由/端点的熔断状态是什么？ |
| [bulkhead](bulkhead/) | 同时允许多少调用者占用并发槽？ |
| [ratelimit](ratelimit/) | 还剩多少预算，怎么计数？ |
| [fault](fault/) | 哪条规则管辖这次调用，它做了什么决定？ |
| [traffic](traffic/) | 这次调用是真实流量还是压测流量？ |
| [label](label/) | 受治理的服务怎么命名（`prefix:name`）？ |
| [discovery](discovery/) | 这个逻辑名此刻有哪些存活的 `host:port`？ |
| [loadbalance](loadbalance/) | 给定存活集合，这一发请求给谁？ |
| [matcher](matcher/) | 这个字符串 map 命中哪些规则？ |

`discovery` 与 `loadbalance` 主要被出站代码消费，但它们的 API 不携带任何方向词汇——它们只负责解析一个名字、选出一个端点。`fault` 与 `loadbalance` 还持有由控制面根驱动的状态。它们按概念归在这里：方向不是它们的，文档来源也不是它们的。

## 说明

- 本目录是命名空间：自身不持有 Go 包。
- 这些包内部的 OTel instrumentation scope 字符串保留历史路径（例如 `.../dataplane/discovery`），它们是面向后端的契约，不是 import 路径。
- 若某个包后来出现治理运行时之外的消费者（cache、lock、scheduler、config），按消费面规则上浮到 `cloud/`，而不是留在这里。

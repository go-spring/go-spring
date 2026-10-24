# kernel

[English](README.md) | [中文](README_CN.md)

`kernel` 是**数据面里方向中立的那一半**：两个调用方向都会使用的概念与机制，但不属于任何一方。

## 规则

> 当一个包**不知道自己在服务哪个方向**时，它属于 `kernel`。它只回答地址、端点、失败状态、预算、流量类别、服务命名这些问题——从不回答“这一次出站尝试”或“这一个入站请求”。

一旦代码开始使用 `Executor`、`Operation`、`Recorder`、`Request`、`Admit`、`Done`
这些词，它就知道方向了，应放在 [outbound](../outbound/) 或 [inbound](../inbound/) 下。

这同时是一条依赖不变式：

> **`kernel` 永远不 import `inbound/` 或 `outbound/`；方向树可以 import `kernel`。**

## 内容

| 包 | 回答的问题 |
| --- | --- |
| [circuitbreaker](circuitbreaker/) | 这个路由/端点的熔断状态是什么？ |
| [bulkhead](bulkhead/) | 同时允许多少调用方占住并发槽？ |
| [ratelimit](ratelimit/) | 预算还剩多少，怎么计数？ |
| [fault](fault/) | 哪条规则管辖这次调用，它决定了什么？ |
| [traffic](traffic/) | 这是真实流量还是压测流量？ |
| [label](label/) | 受治理的服务怎么命名（`prefix:name`）？ |
| [discovery](discovery/) | 这个名字此刻有哪些存活的 `host:port`？ |
| [loadbalance](loadbalance/) | 给定存活集合，这一发请求给谁？ |

`discovery` 与 `loadbalance` 目前只被出站代码消费，但它们的 API 不携带方向词汇——一个解析名字，一个选择端点。按概念归入这里，而不是按当前调用方。

## 说明

- 本目录是命名空间，自身没有 Go 包。
- 这些包里的 OTel instrumentation scope 字符串保留历史路径（例如 `.../dataplane/discovery`）。它们是后端契约，不是 import path。
- 如果某个包将来出现请求路径之外的消费者（cache、lock、scheduler、config），按消费面规则上浮到 `cloud/`，而不是继续留在这里。

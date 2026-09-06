# starter-elasticsearch 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-elasticsearch` 属于 Client 形态（`starter/DESIGN.md` §2.2），
提供 `elastic/go-elasticsearch/v8` 客户端。三个非显然决定值得写下来：
可选 Driver-bean 缝隙、空实现 `destroy`、discovery 只在启动期生效。

## 1. 职责与边界

- 用 `gs.Group` 把 `${spring.elasticsearch}` 每条绑到
  `*elasticsearch.Client` bean。不做默认单实例。
- 构造时跑一次性 `Info` 健康检查，让坏地址/坏证书/坏凭据在启动暴露而不是
  首次使用时才炸。
- 若配置了 `service-name`，从 `cloud/discovery` 解析节点地址；否则使用
  静态 `Addresses`（或 `CloudID`）。

## 2. 关键抽象与缝隙

- **可选 Driver-bean 缝隙。** starter 不直接建 ES 客户端；client 装配由
  `Driver`（接口）负责，公司/伞包 starter 可把自己的 `Driver` 作为可选
  容器 bean 提供。没有该 bean 时 starter 回退到内置 `DefaultDriver`。
  bean 是进程级的，自定义 driver 委托 `DefaultDriver` 以保留按实例行为；
  让 APM/OTel 版 transport 接入而不动 starter 公共 API。
- **`destroy = nil`（v8 client 无 `Close`）。** v8 client 的 transport 是
  `net/http` 空闲连接复用——没有东西可关。`destroyClient` 返 `nil` 只是
  让 `gs.Group` 的 destroy 槽保持与其他 client 一致
  （`project_es_starter_smoke`）。
- **Discovery 只在启动生效。** `service-name` 在启动被解析一次，得到固定
  `Addresses` 列表。v8 client 自己在该列表上做 round-robin，但不会再解析。
  这是刻意的：ES 节点变更以天为单位，不是秒；client 自己会 sniff 集群
  状态。
- **`HealthCheck` 一定传 context。** transport 的 OTel 埋点在 nil 父 context
  上会 panic，所以 `client.Info` 显式带 `WithContext(context.Background())`。

## 3. 约束

- **`Addresses` 或 `CloudID` 或 `ServiceName`。** 三种供给模式仅用一种；
  三者都空时，health check 无处可去，启动失败。
- **`DiscoveryScheme` 决定端点 scheme。** `cloud/discovery` 返的端点仅带
  `host:port`；client 需要 `scheme://host:port`，构造地址时补上
  `discovery-scheme`（`http` / `https`）。
- **冒烟拉 ES 8.13 镜像（已缓存）。** 首启 readiness 最多 120s
  （`project_es_starter_smoke`）；测试相应等待。

## 4. 权衡 / 已否决方案

- **运行时重新 sniff / 解析——否决。** v8 client 已在 seed 列表上做连接
  池化与 dead-node 回退；再套重解析循环会打架。
- **starter 里包 `otelelasticsearch` transport——否决。** transport 包装由
  driver 的 `CreateClient` 完成（内置 `DefaultDriver` 装一个 dynamic
  transport，其 observe+resilience 行为由 `Init` 换入）；基础 transport
  保持纯 net/http，让不 import otel 的应用零负担。

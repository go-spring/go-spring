# starter-nats 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-nats` 属于 Client 形态（`starter/DESIGN.md` §2.2），有一个小变体：
被注入的 bean 是 `*nats.Conn` 的包装类型而不是原始 `*nats.Conn`，因为
NATS 同一连接同时承载 core + JetStream 两套 API。

## 1. 职责与边界

- 用 `gs.Group` 把 `spring.nats.<name>` 每条绑到 `*Conn` bean；
  不做默认单实例（本仓库 client 类 starter 一律仅多实例，见
  `project_starter_capability_backlog`）。
- `Conn` 内嵌 `*nats.Conn`，调用方仍能直接在 bean 上用
  `Publish` / `Subscribe` / `Request`；`JetStream` 只有在
  `jetstream.enabled=true` 时非 nil，且从同一连接派生。
- 把连接事件（async 错误、disconnect、reconnect、close）桥接进 go-spring
  `log`，与业务日志一起落盘。
- 可选地挂 resilience executor（限流 + 熔断）作为可选的
  `PublishGuarded` / `RequestGuarded`；原生 `Publish` / `Request` 不动——
  NATS 无可拒绝式 middleware 缝隙，所以护栏建在调用点。

## 2. 关键抽象与缝隙

- **bean = 包装，非原始 `*nats.Conn`。** 包装同时承载可选 JetStream 上下文
  与 resilience executor，让调用方不用同时 autowire 两个 bean 再自行判断
  关系。
- **`Healthy()` 反映实时状态。** 包装返回 `Conn.IsConnected()`，让
  actuator `health.Indicator`（或 K8s readiness）看到自动重连客户端的
  实时状态，而不是启动期的旧成功值。
- **`destroy = Drain`，不是 `Close`。** `Drain` 让 in-flight 订阅完成再关
  连接，符合框架优雅关停契约。
- **resource key 按实例而非按 subject。** resilience executor 的
  `resource` 字符串就是连接 bean 名，limiter / breaker 状态按连接维度
  聚合，而非按 subject。

## 3. 约束

- **JetStream 必须复用同一连接。** `jetstream.enabled=true` 时通过
  `jetstream.New(nc)` 建 JetStream；失败则原始 `nc` 被关且启动失败。
  不存在第二条连接。
- **TLS 开关是叠加的。** `tls.enabled=true` 触发 `nats.Secure`；
  `insecure-skip-verify`、`ca-file`、`cert-file/key-file` 叠加用于 mTLS 或
  CA 覆盖。
- **鉴权是互相正交的。** 用户名/密码、token、creds file、NKey seed 都可
  独立设，各自映射到不同的 `nats.Option`。
- **`MaxReconnects=-1` 表示无限重连。** client 端重连即可靠性机制，
  没有外部 supervisor。

## 4. 权衡 / 已否决方案

- **直接暴露 `*nats.Conn` 加独立 `JetStream` bean——否决。** 会强迫调用方
  同时 autowire 两个 bean 并自行选择；包装隐藏了这个分裂。
- **在 `Publish` 上做 middleware 型护栏——否决。** NATS 无可拒绝式
  middleware 缝隙，包裹 `Publish` 会静默改变语义；可选的 `PublishGuarded`
  是诚实的接口面。

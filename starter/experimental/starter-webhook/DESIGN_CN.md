# starter-webhook 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

一个薄客户端原型 starter（`starter/DESIGN.md` §2.2），以 starter-mail 为
模板：按次无状态、`gs.Group` 无 destroy 钩子、trace 助手依赖 OTel 全局。
与 mail 唯一有意的分歧是韧性 seam —— 本仓库所有客户端 starter 都带。

## 1. 职责与边界

- **负责**：`${spring.webhook}` 下的每实例通知器组、通道载荷构建与加签、
  POST 路径、逐次韧性与 observe span、非 2xx/厂商错误的错误面。
- **不负责**：模板渲染（调用方自己渲染 `Text`，与 starter-mail 不带模板
  引擎同立场）、治理执行器之外的重试策略、邮件/短信投递、接收端轮换
  （轮换走配置）。

## 2. 关键抽象与 Seam

- **`Notifier`（starter.go）** — 一个端点 + 一个通道 + 一个签名密钥；
  `Send` 构建载荷、把 POST 包进 `webhook.send` producer span，并路由穿过
  `fault.WrapExecutor(resilience.ExecutorFor("webhook:<name>:<channel>"),
  fault.InjectorFor())` 加 `resilience.WrapExecutor` —— 与所有客户端
  starter 相同的中性 seam 栈，与 starter-governance 零耦合。
- **`buildPayload`（payload.go）** — 纯函数 通道 →（body, 额外 query）。
  钉钉加签向 URL 追加 `timestamp`/`sign`；飞书签名进 body；两者都是对
  `<毫秒>\n<secret>` 的 HMAC-SHA256。纯函数让格式可以脱离网络做单测。
- **Trace 助手（trace.go）** — `StartSendSpan` / `EndSpan` 镜像
  starter-mail 的模式；span 带 `webhook.channel` 且只带目标主机（带 token
  的完整 URL 不进遥测）。

## 3. 约束

- 不做启动探针，这与 mail 的拨号探针不同：唯一通用的 webhook 探针就是
  真实 POST，而启动期向生产聊天群发一条垃圾通知，比首次发送才报错是更糟
  的失败模式。构造期仍做廉价校验 —— 未知 `channel` 立即失败，URL 错误
  在首次发送时带端点信息浮出。
- 不注册 `health.Indicator`：无状态客户端，与 mail 同立场。
- 接受的厂商怪癖：某些配置下钉钉/飞书会用 200 应答错误体；非 2xx 一定是
  错误，而"200 带错误体"只在接收方选择非 200 状态码时浮出。需要 body 级
  错误解析的通道（罕见）应以 `buildPayload` 形状自行扩展。

## 4. 权衡 / 已否决的备选

- **每通道载荷构建器 vs. 抽象层**：五种格式各约 10 行 JSON 组装；按通道
  建插件家族的组织成本会超过被组织的代码。`buildPayload` 保持纯 switch，
  加一个通道就是加一个函数。
- **Send 上带韧性 vs. 不带（与 mail 对齐）**：通知常打到有频控的聊天
  端点；guard 只花一次接口调用，无治理时退化为直通 —— 与 mail（SMTP 路
  径面向连接、失败本身就很响）的不对称是合理的。
- **纯 stdlib vs. 通知 SDK**：不存在同时覆盖这五种格式的像样 Go SDK；
  手写 JSON 让模块零依赖、可审计。

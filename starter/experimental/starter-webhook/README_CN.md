# starter-webhook

[English](README.md) | [中文](README_CN.md)

`starter-webhook` 为 Go-Spring 提供零第三方依赖的通知支持：多实例 webhook
通知器，按接收方自己的载荷格式 POST —— generic JSON、钉钉、飞书、企业微信
或 Slack —— 支持可选的 HMAC 加签、逐次发送的韧性（限流/熔断/故障注入）
与 OTel span。需要邮件通道时配合 `starter-mail` 使用。

## 安装

```bash
go get go-spring.org/starter-webhook
```

## 快速开始

### 1. 导入

```go
import _ "go-spring.org/starter-webhook"
```

### 2. 配置

```properties
# 通用接收端（自建端点）
spring.webhook.instances.ops.url=https://alerts.example.com/hook
spring.webhook.instances.ops.channel=generic

# 钉钉群机器人（加签密钥可选）
# spring.webhook.instances.ding.url=https://oapi.dingtalk.com/robot/send?access_token=xxx
# spring.webhook.instances.ding.channel=dingtalk
# spring.webhook.instances.ding.secret=SEC...

# 飞书 / 企业微信 / Slack 机器人
# spring.webhook.instances.feishu.url=https://open.feishu.cn/open-apis/bot/v2/hook/xxx
# spring.webhook.instances.feishu.channel=feishu
# spring.webhook.instances.feishu.secret=...
# spring.webhook.instances.wecom.url=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx
# spring.webhook.instances.wecom.channel=wecom
# spring.webhook.instances.slack.url=https://hooks.slack.com/services/XXX/YYY/ZZZ
# spring.webhook.instances.slack.channel=slack
```

### 3. 注入

```go
type Service struct {
    Ops  *StarterWebhook.Notifier `autowire:"ops"`
    Ding *StarterWebhook.Notifier `autowire:"ding"`
}
```

### 4. 使用

```go
err := s.Ops.Send(ctx, &StarterWebhook.Notification{
    Title: "deploy",
    Text:  "api 服务已发布",
})
```

`Title` 是标题（多数接收方渲染为加粗/markdown），`Text` 是正文。非 2xx
应答（以及厂商错误体）都会以错误形式返回。

## 核心特性

- **五种通道、一套 API** — `generic`、`dingtalk`、`feishu`、`wecom`、
  `slack`；换接收方是改配置，不是改代码。
- **加签** — `secret` 自动启用钉钉加签（timestamp+sign 查询对）或飞书
  签名。
- **韧性** — 每次 Send 走治理执行器（`webhook:<name>:<channel>`），端点
  抖动会被熔断而不是堆请求。未导入 `starter-governance` 时是透明直通。
- **可观测** — 经 observe kit 的逐次访问日志，外加 OTel span
  （`webhook.send`），依赖 `starter-otel` 安装的全局 provider。
- **天然无状态** — 不持有连接、无健康指示器、无 destroy 钩子；为什么没有
  启动探针见 DESIGN。

## 高级特性

**多通知器** — 任意多个端点、混合通道，按 bean 名选择（见上面配置块）。

**扇出助手** — 把同一条通知发往你持有的每个通道；通知器是彼此独立的
bean：

```go
for _, n := range []*StarterWebhook.Notifier{s.Ops, s.Ding} {
    _ = n.Send(ctx, note)
}
```

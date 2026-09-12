# starter-webhook

[English](README.md) | [中文](README_CN.md)

`starter-webhook` provides notification support for Go-Spring with zero
third-party dependencies: multi-instance webhook notifiers that POST a
notification in the receiver's own payload format — generic JSON,
DingTalk (钉钉), Feishu (飞书), WeCom (企业微信) or Slack — with optional
HMAC signing, per-send resilience (rate limit / circuit breaking / fault
injection) and OTel spans. Pair it with `starter-mail` for email delivery.

## Installation

```bash
go get go-spring.org/starter-webhook
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-webhook"
```

### 2. Configure

```properties
# Generic receiver (self-built endpoints)
spring.webhook.instances.ops.url=https://alerts.example.com/hook
spring.webhook.instances.ops.channel=generic

# DingTalk group robot (加签 secret optional)
# spring.webhook.instances.ding.url=https://oapi.dingtalk.com/robot/send?access_token=xxx
# spring.webhook.instances.ding.channel=dingtalk
# spring.webhook.instances.ding.secret=SEC...

# Feishu / WeCom / Slack bots
# spring.webhook.instances.feishu.url=https://open.feishu.cn/open-apis/bot/v2/hook/xxx
# spring.webhook.instances.feishu.channel=feishu
# spring.webhook.instances.feishu.secret=...
# spring.webhook.instances.wecom.url=https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx
# spring.webhook.instances.wecom.channel=wecom
# spring.webhook.instances.slack.url=https://hooks.slack.com/services/XXX/YYY/ZZZ
# spring.webhook.instances.slack.channel=slack
```

### 3. Inject

```go
type Service struct {
    Ops  *StarterWebhook.Notifier `autowire:"ops"`
    Ding *StarterWebhook.Notifier `autowire:"ding"`
}
```

### 4. Use

```go
err := s.Ops.Send(ctx, &StarterWebhook.Notification{
    Title: "deploy",
    Text:  "service api released",
})
```

`Title` is the headline (rendered bold/markdown by most receivers), `Text`
is the body. Non-2xx answers (and vendor error bodies) surface as errors.

## Core Features

- **Five channels, one API** — `generic`, `dingtalk`, `feishu`, `wecom`,
  `slack`; switching receiver is a config change, not a code change.
- **Signing** — `secret` enables DingTalk 加签 (timestamp+sign query pair)
  or the Feishu signature automatically.
- **Resilience** — every Send routes through the governance executor
  (`webhook:<name>:<channel>`), so a flapping endpoint gets circuit-broken
  instead of piling up. With `starter-governance` absent this is a
  transparent pass-through.
- **Observability** — per-send access log through the observe kit, plus OTel
  spans (`webhook.send`) riding the globals installed by `starter-otel`.
- **Stateless by design** — no connections held, no health indicator, no
  destroy hook; see DESIGN for why there is no startup probe.

## Advanced Features

**Multiple notifiers** — any number of endpoints, mixed channels, selected
by bean name (see the config block above).

**Fan-out helper** — send the same notification through every channel you
hold; the notifiers are independent beans:

```go
for _, n := range []*StarterWebhook.Notifier{s.Ops, s.Ding} {
    _ = n.Send(ctx, note)
}
```

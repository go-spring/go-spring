# starter-webhook 使用说明 — 参考手册

深度使用文档。概览见 [README.md](README.md)。所有行为声明均对照源码
（`starter.go`、`config.go`、`payload.go`、`trace.go`、`webhook_test.go`）与自包含的
[example/](example/)（自带本地 HTTP receiver；`go run .` 冒烟验证，无需 docker）核实。
**本 starter 就是组件本身**——generic/DingTalk/Feishu/WeCom/Slack 的 payload 格式与签名
在仓内实现；接收端配置见各平台机器人文档
（[DingTalk](https://open.dingtalk.com/document/robots/custom-robot-access)、
[Feishu](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot)、
[WeCom](https://developer.work.weixin.qq.com/document/path/91770)、
[Slack](https://api.slack.com/messaging/webhooks)）。

**激活条件**：每个 `spring.webhook.<name>` 子树按名字各创建一个 `*Notifier` bean。
不配置则不装配。刻意**没有启动探测**——唯一通用探测是真发一次 POST，而
"sending a junk notification at boot is worse than failing on first use"
（源码注释，starter.go newNotifier）。

---

## 1. 完整工程示例

一个向 DingTalk 机器人与自建 receiver 投递、带治理限流与可观测的告警服务。文件树
（对应冒烟验证过的 [example/](example/)）：

```
demo/
├── go.mod
├── main.go
├── alert.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-webhook    latest
    go-spring.org/starter-otel       latest   # 可选：真实 span 导出
    go-spring.org/starter-governance latest   # 可选：限流 / 熔断 / fault
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-webhook"
)

func main() { gs.Run() }
```

**alert.go** —— 应用的全部通知面：

```go
package alert

import (
    "context"

    "go-spring.org/spring/gs"

    StarterWebhook "go-spring.org/starter-webhook"
)

// Service 注入命名 notifier。新增通道是纯配置变更加一个字段——不改 starter。
type Service struct {
    Alert  *StarterWebhook.Notifier `autowire:"alert"`  // dingtalk + 加签
    Report *StarterWebhook.Notifier `autowire:"report"` // generic receiver
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// Fire 投递一条告警。Send 负责构建渠道 payload、签名、给 POST 套 producer span，
// 并经 resilience executor 路由（治理启用时为限流 / 熔断 / fault 注入）。
func (s *Service) Fire(ctx context.Context, title, text string) error {
    return s.Alert.Send(ctx, &StarterWebhook.Notifier.Notification{Title: title, Text: text})
}
```

**conf/app.properties** —— 完整注释配置（在 example 配置上扩展）：

```properties
# --- notifier "alert"：带 加签 secret 的 DingTalk 群机器人 ---------------------
spring.webhook.alert.url=https://oapi.dingtalk.com/robot/send?access_token=xxx
spring.webhook.alert.channel=dingtalk
spring.webhook.alert.secret=SEC...
spring.webhook.alert.timeout=5s

# --- notifier "report"：向自建 receiver 的纯 JSON POST -------------------------
spring.webhook.report.url=http://127.0.0.1:18080/hook
spring.webhook.report.channel=generic
# spring.webhook.report.observability.level=brief   # 按实例，ctor 绑定

# --- 可观测（starter-otel）----------------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true

# --- governance（可选；资源标签 webhook:alert:dingtalk）------------------------
govern.source.file.path=conf/govern.yaml
```

其他渠道（见 [example/conf/app.properties](example/conf/app.properties) 注释）：
`feishu`（`https://open.feishu.cn/open-apis/bot/v2/hook/xxx` + 签名 `secret`）、
`wecom`（`https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`，secret 被忽略）、
`slack`（`https://hooks.slack.com/services/XXX/YYY/ZZZ`，secret 被忽略）。

**验证**（与 example 同构，example 自带断言收到的 payload）：

```bash
cd example && go run .      # 打印 "Webhook delivered: map[...]" 后自退出
# 对真实机器人：直接看群消息；或看下文的 span/metrics
curl -s -X POST http://127.0.0.1:18080/hook ...
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-webhook
  └─ init(): gs.Group("${spring.webhook}", newNotifier, nil)  [薄壳、mail 风格：
        按调用无状态，无 destroy hook]

gs.Run()
  ├─ 配置绑定：spring.webhook.<name>.* → Config（value tag；url 经 expr 必填）
  ├─ newNotifier：
  │    ├─ 用空 Notification 干跑一次 buildPayload——校验 channel 取值、
  │    │   （dingtalk/feishu）签名可用性，全程无网络请求
  │    ├─ exec := fault.WrapExecutor(resilience.ExecutorFor("webhook:<name>:<channel>"))
  │    ├─ exec := resilience.WrapExecutor(exec, "webhook", c.Observability)
  │    └─ &http.Client{Timeout: c.Timeout} —— 每 notifier 一个 client，不池化
  ├─ bean 就绪：*Notifier 注入所有 `autowire:"<name>"` 处
  ├─ Run / 服务：无后台 goroutine、无探测（理由见顶部激活说明）
  └─ SIGTERM：无 destroy hook（每次 Send 是无状态 HTTP 请求）
```

resilience 资源标签为 `webhook:<name>:<channel>`——按实例**且**按渠道，因此同 URL
不同名的两个 notifier 拥有独立的熔断/限流状态（对比 starter-s3，其标签只有 endpoint）。

### 2.2 一次投递的逐层走读

`Notifier.Send(ctx, notification)`（starter.go:92）：

1. **Payload 构建** —— `buildPayload(channel, n, secret, time.Now())`（payload.go:39）：
   - `generic`：`{"title":..., "text":...}` 的 JSON POST。
   - `dingtalk`：markdown 消息（`msgtype=markdown`，正文 = title + "\n\n" + text）；配置了
     secret 时，加签对——`timestamp`（毫秒）+ HMAC-SHA256(ts+"\n"+secret)、base64、
     URL 转义——作为**额外 query 参数**返回，由 `withQuery` 合并进 URL
     （时间戳取 `time.Now()`，因此每次发送都是新签名）。
   - `feishu`：text 消息；配置 secret 时同一 HMAC 计算折叠进 **body**
     （`timestamp` + `sign` 字段）。
   - `wecom` / `slack`：markdown / text 消息，secret 被忽略。
   - 未知 channel → 报错 `webhook: unknown channel ... (want generic|dingtalk|feishu|wecom|slack)`。
2. **Span** —— `startSend` 开 producer span `webhook.send`（属性
   `messaging.system=webhook`、`webhook.channel`、`webhook.destination.host`——只有
   scheme://host，签名 URL 不进遥测）。
3. **Executor** —— POST 闭包经资源标签下的治理 executor：引入 starter-governance 后
   限流 / 熔断 / retry（若经治理配置）/ fault 注入生效，否则透明直通。
   `resilience.WrapExecutor` 按级别发 outcome span + 调用计数 + 时长直方图 +
   访问日志。手工构造的零值 Notifier（测试场景）没有 executor，直接 POST——
   Send 两种情况都可用。
4. **POST** —— `n.post`：`Content-Type: application/json`，client 受 `timeout` 约束；
   任何非 2xx 状态都是错误，并带 body 前 512 字节。
5. `EndSpan(span, err)` 记录失败并关闭 span。

重试行为：**无内建 retry**。仅当通过 starter-governance 为
`webhook:<name>:<channel>` 资源配置了 retry 策略才会重试；无治理时失败的 POST 立即
把错误返回给调用方。⚠ DingTalk/Feishu 有些失败以 HTTP 200 + 错误 body 返回——
`post` 只检查状态码，这类响应当作成功（见 §6 嫌疑）。

### 2.3 启动干跑能拦住什么（拦不住什么）

`newNotifier` 在装配前用空 Notification 构建一次 payload。这能校验 channel 枚举与
secret 参与签名的可用性——全程无网络。它拦不住错误 URL、过期 access token、错误
secret（签名会被计算，但接收方不校验）。这些都在首次 Send 时暴露。

---

## 3. 逐 key 行为参考

前缀 `spring.webhook.<name>.*` —— 所有 key 均为 ctor 绑定的 `Config`（config.go），
即**带实例前缀**，包括 observability 子结构（与 starter-s3 不同，这里没有
wrapper-field tag）。

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `url` | string | — | **必填**（`expr:"$ != ''"`）。POST 目标；dingtalk 通常带 `access_token` query。 | 缺失/为空 → 绑定期校验报错。格式错 → 首次 Send 报 `webhook: invalid url`。 |
| `channel` | string | generic | `generic`\|`dingtalk`\|`feishu`\|`wecom`\|`slack`；决定 payload 构建器、签名方案，以及 resilience 标签第三段。 | 未知值 → 启动期经干跑 buildPayload 报错（报错文案见 §2.2）。 |
| `secret` | string | — | DingTalk 加签（SEC...）或 Feishu 签名密钥；generic/wecom/slack **忽略**。 | ⚠ 配在错误渠道被静默忽略——未签名消息在 Send 时被接收方拒绝。 |
| `timeout` | duration | 5s | 约束一次 POST（`http.Client.Timeout`）。 | 过小 → 慢平台出现客户端超时；executor（按治理策略）可能重试放大负载。 |
| `observability` | struct | brief | 每次发送的访问日志/metric/span 级别（`off`\|`brief`\|`detailed`），实例前缀下的子 key `observability.level` / `.maxArgBytes`（默认 512）/ `.skipOps`；喂给 `resilience.WrapExecutor`。 | `off` 去掉逐发送日志；`detailed` 按 maxArgBytes 记录 payload 字节。 |

---

## 4. 验证与故障演练

### 4.1 投递演练（自包含，与 example/check.sh 同路径）

```bash
cd example && go run .    # receiver 捕获 POST；example 断言
                           # title=="deploy" && text=="example finished"
```

对真实机器人：`Send(ctx, &Notification{Title:"smoke", Text:"hello"})` 后看群消息。

### 4.2 payload/签名演练（dingtalk）

把 notifier 指向本地 receiver 并检查请求：

```bash
# receiver 日志：query 带 timestamp=...&sign=...（HMAC of ts+"\n"+secret）
# 同一秒内两次发送也产生两个不同签名（time.Now() 取新值）。
```

离线复算验证：`base64(HMAC_SHA256(secret, ts+"\n"+secret))`——与 `dingtalkSign`
（payload.go:77）一致。

### 4.3 失败演练（无需重启）

用一个坏 URL（如端口 1）启动：每次 Send 返回 `webhook: post failed` 带连接错误，
producer span（status Error、error 事件）与 executor 的按 outcome 计数都有记录。
starter 自身没有可热切换的东西；引入 starter-governance 后，改规则文件即可给
`webhook:alert:dingtalk` 在线武装限流——超限的发送会以 limit-reject outcome 快速失败，
不再到达平台。

### 4.4 可观测演练

引入 starter-otel 后发送一次并读：span `webhook.send`（kind PRODUCER，属性见 §2.2
第 2 步）、时长直方图、访问日志行（级别来自 `observability.level`；`detailed` 按
`maxArgBytes` 带 body 字节）。验证脱敏性质：span 属性是
`webhook.destination.host=scheme://host`，绝不带签名 URL。

### 4.5 渠道矩阵演练

example 的 generic receiver 也能当 slack/wecom/feishu 的 payload 检查器：切换
`channel` 后 Send，对照 §2.2 的构建器断言收到的 JSON 形状（feishu/wecom 的
`msg_type`/`content`，slack 的 `text`）。未知 channel 取值必须在启动期失败——
那正是干跑在工作。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动中止 "unknown channel" | `channel` 拼写错误 | generic/dingtalk/feishu/wecom/slack 之一。 |
| 启动中止：url 绑定报错 | 缺 `spring.webhook.<name>.url` | 补上——expr tag 必填。 |
| 首次 Send 报 "invalid url" | URL 格式错（空格、scheme 错等） | 设计上无启动网络探测；修 URL。 |
| DingTalk 拒收：sign 不匹配 | secret 错误/轮换，或时钟偏移 >1 小时（签名内时间戳） | 重新复制加签 secret；校准主机时钟。 |
| DingTalk 返回 200 但群里没消息 | 厂商以 HTTP 200 返回错误 body（errcode）——`post` 不检查 | 查平台机器人日志；见 §6 嫌疑（仅按状态码判成功）。 |
| 配了 `secret` 但消息未签名 | 渠道是 generic/wecom/slack（secret 被忽略） | 只有 dingtalk/feishu 签名。 |
| 发送突然以 limit-reject 失败 | 治理对 `webhook:<name>:<channel>` 武装了限流 | 调大限额或撤规则（热切换）。 |
| Feishu 拒收：timestamp invalid | 签名时间戳过旧（主机时钟）或 secret 不匹配 | 处置同 DingTalk sign 行。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 5（另 3 个实例前缀 observability.* 子 key） |
| 其中必填 | 1（`url`） |
| quickstart 前置外部依赖 | 0（example 自带 receiver） |
| "注意/坑" 条数 | 4 |

设计嫌疑清单（第一条沿用上一版）：
- 渠道 payload 构建器在 starter 内且无扩展点——新增渠道必须改 starter；可考虑
  channel 注册表。
- 新增：`post` 把任何 2xx 当成功，但 DingTalk/Feishu 会以 HTTP 200 返回厂商错误
  body（`errcode`）——starter.go:116-117 的 doc 注释声称会捕获，实现没有。要么按
  已知渠道解析 body，要么修正注释。
- 新增：无内建 retry，尽管 webhook 是典型的 at-least-once 场景——retry 只存在于
  治理策略中，容易被漏配。
- 新增：`plainText`（payload.go:129）发送路径未使用——死辅助函数，建议删除。

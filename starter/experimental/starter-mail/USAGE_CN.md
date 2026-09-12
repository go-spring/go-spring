# starter-mail 使用说明 — 参考手册

深度使用文档。概览见 [README.md](README.md)。所有行为声明均对照源码
（`starter.go`、`config.go`、`trace.go`）与可运行的 [example/](example/) /
[example-otel/](example-otel/) 核实。**SMTP 与消息语义（header、MIME、附件、认证机制）
见 [go-mail 官方文档](https://github.com/wneessen/go-mail) 与
[RFC 5321](https://www.rfc-editor.org/rfc/rfc5321)** —— 本文只写 go-spring 的增量：
配置、装配、启动 fail-fast、trace 辅助。

**激活条件**：每个 `spring.mail.instances.<name>` 子树按名字各创建一个 `*Mailer` bean。
不配置 `spring.mail.instances.*` 则不装配、不做启动拨号，starter 完全惰性。没有 `enabled` key，
也没有默认单例。

---

## 1. 完整工程示例

一个通过命名 mailer 发送 HTML+附件邮件、并经 starter-otel 上报 trace 的通知服务。
文件树（对应 [example-otel/](example-otel/)，已用 docker-compose 的 MailHog + Jaeger
冒烟验证）：

```
demo/
├── go.mod
├── main.go
├── notify.go
└── conf/
    └── app.properties
```

**go.mod**（关键依赖）：

```
require (
    github.com/wneessen/go-mail     latest   # 传递依赖，由 starter 引入
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-mail      latest
    go-spring.org/starter-otel      latest   # 可选：真实 span 导出
)
```

**main.go**：

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**notify.go** —— 应用的全部邮件面：

```go
package notify

import (
    "context"

    "go-spring.org/spring/gs"

    StarterMail "go-spring.org/starter-mail"
)

// Service 注入一个命名 mailer。第二个 mailer（如营销通道）只是配置变更
// 加一个 autowire 字段——不需要改 starter 代码。
type Service struct {
    Notify *StarterMail.Mailer `autowire:"notify"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// SendReport 自行渲染正文（starter 刻意不带模板引擎），发送一封
// multipart/alternative 邮件并带附件。
func (s *Service) SendReport(ctx context.Context) error {
    ctx, span := StarterMail.StartSendSpan(ctx, "daily-report")
    defer StarterMail.EndSpan(span, nil) // Send 失败时传入 err；见下
    return s.Notify.Send(ctx, &StarterMail.Message{
        To:      []string{"alice@example.com"},
        Subject: "daily report",
        Text:    "plain fallback",
        HTML:    "<h1>report</h1>",
        Attachments: []StarterMail.Attachment{
            {Filename: "report.csv", Data: reportBytes()},
        },
    })
}
```

**conf/app.properties** —— 完整注释配置：

```properties
# --- mailer "notify"（每个 spring.mail.instances.<name> 子树一个实例）-------------------
spring.mail.instances.notify.host=smtp.example.com
spring.mail.instances.notify.port=587
spring.mail.instances.notify.username=apikey
spring.mail.instances.notify.password=${SMTP_PASSWORD}
spring.mail.instances.notify.auth-type=auto
spring.mail.instances.notify.from=noreply@example.com
spring.mail.instances.notify.timeout=10s
spring.mail.instances.notify.tls.mode=starttls
# spring.mail.instances.notify.tls.insecure-skip-verify=false   # 仅测试

# --- 可观测（starter-otel），已在 example-otel 验证 ---------------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
```

本地验证时把 mailer 段换成 [example/conf/app.properties](example/conf/app.properties) 的
MailHog 行（`host=127.0.0.1 port=1025 tls.mode=none from=noreply@example.com`），并启动
`docker compose -f example/docker-compose.yml up -d`（SMTP :1025，Web UI :8025）。

**验证**（与 example 自身的断言同构）：

```bash
go run .                                          # 启动；观察 "mailer created host=..." 日志
curl -s :8025/api/v2/messages | jq .total         # MailHog：>=1 封已投递
curl -s :8025/api/v2/messages | jq '.messages[0].Content.Headers'   # 收件人、主题、附件
curl -s 'http://127.0.0.1:16686/api/traces?service=demo' | jq '.data[0].spans[].operationName'  # "mail.send"（example-otel 拓扑）
```

---

## 2. 装配与时序

### 2.1 Bean 生命周期

```
import starter-mail
  └─ init(): gs.Group("${spring.mail}", newMailer, nil)     [每个 spring.mail.instances.<name> 一个实例]

gs.Run()
  ├─ 配置绑定：spring.mail.instances.<name>.* → Config（value tag；host 由 errutil.RequireField 强制）
  ├─ newMailer：解析 auth（仅 username 非空时）→ TLS mode → mail.NewClient
  ├─ fail-fast 探测：client.DialWithContext（受 timeout 约束）后 Close
  │     └─ host/port/auth/TLS 配错 ⇒ 启动报错，拒绝拉起（源码注释：
  │        "so a misconfiguration surfaces at boot rather than on the first send"）
  ├─ bean 就绪：*Mailer 注入所有 `autowire:"<name>"` 处
  ├─ Run / 服务：无后台 goroutine、无连接池
  └─ SIGTERM：无 destroy hook（每次 Send 自行拨号并关闭；无可释放资源）
```

设计理由引自源码注释：group 注册处写明了为何没有 destroy 回调——"the underlying
client opens a fresh connection per Send and closes it when done, so there is nothing to
release at shutdown"（starter.go init / Mailer 文档）。

### 2.2 一次发送的逐层走读

`Mailer.Send(ctx, msgs...)`（starter.go:169）：

1. **构建阶段** —— 对每条消息执行 `m.build(msg)`：解析 From（`msg.From` → 否则用
   mailer 配置的 `from`；两者皆空报 "no From address"）；经 go-mail 解析器校验 To
   （必填）、Cc、Bcc；组装正文——Text+HTML ⇒ `multipart/alternative`（收件端客户端
   自选最丰富部分），仅 HTML ⇒ HTML 正文，否则纯 Text；附件从内存字节挂载
   （`AttachReader`）。
2. **投递阶段** —— 单次 `DialAndSendWithContext` 开一条 SMTP 连接发完全部消息后关闭。
   部分失败语义归 go-mail；starter 只把错误包一层 `errutil.Explain(err, "mail: send failed")`。
3. **Tracing（选配，调用侧）** —— 若应用调用了 `StartSendSpan`（trace.go:41），span
   `mail.send`（SpanKind=client，属性 `messaging.system=smtp`、`messaging.operation=send`、
   `mail.purpose=<purpose>`）在 `EndSpan` 里记录错误并结束。不引入 starter-otel 时
   OTel 全局 TracerProvider 是 no-op——线上零字节，也不告警。

注意 trace span **不在** `Send` 内部：starter 暴露两个辅助函数，由调用方夹住调用
（已记入 §6 设计嫌疑）。

---

## 3. 逐 key 行为参考

前缀 `spring.mail.instances.<name>.*` —— 绑定进 ctor 的 `Config`（config.go），因此这些 key
**带实例前缀**。（本 starter 无 wrapper 字段 value tag，不存在顶层 key。）

| Key | 类型 | 默认值 | 行为 / 联动 | 配错后果 |
|-----|------|--------|-------------|----------|
| `host` | string | — | **必填**（newMailer 里 `errutil.RequireField`）。无 localhost 兜底。 | 缺失 → 启动即失败，报必填字段错误。 |
| `port` | int | 587 | 传给 `mail.WithPort`。 | ⚠ 465 不配 `tls.mode=tls` → 对隐式 TLS 服务器说明文 → 启动拨号失败；587 配 `tls.mode=tls` 同理失败。 |
| `username` | string | — | 非空则开启 SMTP 认证并使用 `password`；空 ⇒ `WithSMTPAuth(SMTPAuthNoAuth)`（匿名）。 | 只配 password 不配 username 会被静默忽略。 |
| `password` | string | — | 仅 username 非空时生效。 | 密码错误 → 启动拨号失败（fail-fast）。 |
| `auth-type` | string | auto | `auto`\|`plain`\|`login`\|`cram-md5`（parseAuthType，大小写不敏感；接受 `crammd5` 别名）。仅 username 非空时读取。 | 未知值 → 启动报错并列出合法值。 |
| `from` | string | — | 默认发件人；逐消息的 `Message.From` 可覆盖。 | 空 from + 消息不带 From → Send 时报错（非启动期）。 |
| `timeout` | duration | 10s | 同时约束启动探测与每次发送的拨号（`WithTimeout` + 探测 context）。 | 过小 → 慢中继上出现伪启动/发送超时。 |
| `tls.mode` | string | starttls | `starttls`（空值同义，强制升级）\| `tls`/`ssl`（隐式 TLS）\| `none`（明文，仅测试）。未知 → 启动报错。 | 生产用 `none` 会明文送凭据；mode 与 port 不匹配 → 启动期拨号失败。 |
| `tls.insecure-skip-verify` | bool | false | 为 true 时安装 `InsecureSkipVerify` 的 TLS 配置（ServerName=host）。 | 仅测试可用；生产等于接受伪造证书——静默 MITM 暴露。 |

`value:"${tls}"` 子结构绑定意味着 `tls.*` key 也在同一实例前缀下
（`spring.mail.instances.<name>.tls.mode`），不在顶层。

---

## 4. 验证与故障演练

### 4.1 投递演练（MailHog，与 example/check.sh 同路径）

```bash
cd example && docker compose up -d && go run .      # 打印 "mail sent and delivered OK"
curl -s :8025/api/v2/messages | jq '.total'                          # 1
curl -s :8025/api/v2/messages | jq '.messages[0].To'                 # alice, bob, carol
```

example 在发送 To×2 + Cc×1、带一个附件的一封邮件后断言 `total >= 1`
（example.go runTest）。

### 4.2 fail-fast 演练

把 `spring.mail.instances.notify.port=9999`（无监听）后启动：启动中止并报
`mail: startup dial to ...:9999 failed`。这是刻意姿态——探测（newMailer，
starter.go:137-144）的存在就是让坏配置到不了首次发送。

### 4.3 TLS 姿态演练

将 `host/port` 指向真实提交服务器并故意配错 `tls.mode`（如对只收 STARTTLS 的 587 端口
配 `none`）：启动在探测处失败；改回 `starttls` 即可启动。探测执行与 Send 相同的协商，
因此 TLS mode 错误免费被拦截。

### 4.4 Trace 演练（example-otel）

```bash
cd example-otel && docker compose up -d && go run .   # example 自带断言 Jaeger 中有 trace
curl -s 'http://127.0.0.1:16686/api/traces?service=mail-otel-example&limit=1' | grep '"data":\['
```

可读 span 字段：operation `mail.send`、kind CLIENT、属性 `messaging.system=smtp`、
`mail.purpose`（你传入的字符串）、失败时有 error 事件 + status Error。

### 4.5 From 回退演练

配置里去掉 `from`，应用发送的 `Message` 也不带 `From` → 每次 Send 返回
`mail: no From address (set message.From or spring.mail.instances...from)`。任一侧补回即恢复。

---

## 5. 排障表

| 症状 | 可能原因 | 处置 |
|------|----------|------|
| 启动中止："startup dial ... failed" | host/port 错误、服务未起、或 TLS mode 与端口不匹配 | 修好三元组（port ↔ tls.mode ↔ 服务器能力）；报错里带 host:port。 |
| 启动中止："unknown tls mode / auth-type" | 枚举值拼写错误 | 用 starttls\|tls\|none 与 auto\|plain\|login\|cram-md5（大小写不敏感）。 |
| 启动中止：required field host | 缺 `spring.mail.instances.<name>.host` | 补上；没有 localhost 默认值。 |
| Send 报 "no From address" | 配置 `from` 与 `Message.From` 皆空 | 任设其一（消息侧优先）。 |
| Send 报 "message has no recipients" | `To` 为空 | To 必填；只有 Cc/Bcc 不够。 |
| 启动正常、之后 "send failed" | 中继重启 / 凭据过期 / timeout 过小 | 调大 `timeout`；探测只证明启动期健康——见 §6 嫌疑。 |
| 附件丢失 | Data 传 nil 或只给了文件名 | Attachment 是名字+字节；mailer 从不读磁盘。 |
| 完全没有 span | 未引入 starter-otel，或未调 StartSendSpan | 两者都要——span 夹持在设计上归调用方。 |

## 6. 设计体检表

| 指标 | 数值 |
|------|------|
| 配置 key 总数 | 10（含 2 个 `tls.*`） |
| 其中必填 | 1（`host`） |
| quickstart 前置外部依赖 | 1（SMTP 服务器 / MailHog） |
| "注意/坑" 条数 | 4 |

设计嫌疑清单（前两条沿用上一版）：
- Trace 需要手工 `StartSendSpan`/`EndSpan` 而非内置于 `Send`——调用侧样板代码；
  可考虑在引入 starter-otel 时直接包住 Send。
- 有启动拨号探测却无健康检查——探测结果不暴露到运行时；启动期健康与稳态健康被混同。
- fail-fast 探测使每次启动对每个实例做一次 SMTP 登录，对配额受限的中继
  （如 verified-sender API）有成本；可接受但实例数放大时要记得。
- `port` ↔ `tls.mode` 耦合只靠探测隐式校验——启动错误若能点名这组配对，
  可省一轮排障。

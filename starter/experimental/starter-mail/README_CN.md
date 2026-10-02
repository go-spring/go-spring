# starter-mail

[English](README.md) | [中文](README_CN.md)

`starter-mail` 基于 github.com/wneessen/go-mail 提供 SMTP 发信封装,让 Go-Spring
应用便捷地发送事务性邮件(通知、告警、验证码等)。该库为纯 Go 实现,交叉编译无负担。

它只负责发信(不做 IMAP/POP3 收信),也不内置模板引擎:HTML 正文由业务自行渲染后
传入最终字符串,模板方案完全由你掌控。

## 安装

```bash
go get go-spring.org/starter-mail
```

## 快速开始

### 1. 导入 `starter-mail` 包

参考 [main.go](example/main.go) 文件。

```go
import _ "go-spring.org/starter-mail"
```

### 2. 配置 Mailer

在项目的[配置文件](example/conf/app.properties)中,于 `spring.mail.instances.<name>` 下定义
一个或多个具名 mailer。`host` 为必填项,不做 localhost 兜底——服务器缺失或不可达会
在启动时快速失败。

```properties
spring.mail.instances.notify.host=smtp.example.com
spring.mail.instances.notify.port=587
spring.mail.instances.notify.username=apikey
spring.mail.instances.notify.password=${SMTP_PASSWORD}
spring.mail.instances.notify.from=noreply@example.com
spring.mail.instances.notify.tls.mode=starttls
```

### 3. 注入 Mailer

参考 [main.go](example/main.go) 文件。每个具名实例都会以该名字注册为一个
`*Mailer` bean。没有默认单例——按名字选择实例;新增一个 mailer 仅需改配置。

```go
import StarterMail "go-spring.org/starter-mail"

type Service struct {
    Notify *StarterMail.Mailer `autowire:"notify"`
}
```

### 4. 发送邮件

参考 [main.go](example/main.go) 文件。构造 `Message` 后调用 `Send`。可提供
纯文本正文、HTML 正文或二者兼具(multipart/alternative),并支持多收件人与附件。

```go
err := s.Notify.Send(ctx, &StarterMail.Message{
    To:      []string{"alice@example.com", "bob@example.com"},
    Cc:      []string{"carol@example.com"},
    Subject: "Welcome",
    Text:    "纯文本兜底正文。",
    HTML:    "<h1>Hello</h1><p>一封 <b>HTML</b> 邮件。</p>",
    Attachments: []StarterMail.Attachment{
        {Filename: "report.txt", Data: reportBytes},
    },
})
```

每次 `Send` 打开一条连接、发送全部消息后关闭,调用之间不保持长连接(因此也无需
destroy 钩子)。

## 核心特性

[示例](example/main.go)会对接一个真实的 MailHog 服务器自校验:发送一封带 HTML
正文、纯文本备选、附件及多收件人(To/Cc)的邮件,再通过 MailHog 的 HTTP API 验证
投递结果。

* **多收件人**:`To`、`Cc`、`Bcc` 均为地址列表;至少需要一个 `To`。
* **HTML + 附件**:设置 `HTML`(可选配 `Text` 作为兜底),通过 `Attachments` 传入
  文件字节即可附加附件。
* **多 mailer**:`spring.mail` 下的每个条目都成为一个独立配置的 `*Mailer` bean;
  按名字注入即可通过不同服务器或不同发件人发信。
* **可选启动探测**:host 缺失或 auth/TLS 模式非法仍在启动时快速失败;服务器不可达
  仅在 `ping=true` 时由探测拦下,否则等到首次发信才暴露。

## 认证与 TLS

设置 `username`(及 `password`)即启用认证。`auth-type` 选择机制:`auto`(协商服务器
支持的最强机制,默认)、`plain`、`login` 或 `cram-md5`。未设 `username` 时匿名连接。

`tls.mode` 选择传输安全:

* `starttls`(默认):先以明文连接再通过 STARTTLS 升级;服务器不支持则连接失败。
  通常用 587 端口。
* `tls`:从首字节起隐式 TLS(SMTPS),通常用 465 端口。
* `none`:不加密;仅用于本地测试服务器。

## 配置项

`spring.mail.instances.<name>` 下的每个 mailer 读取以下属性:

| 属性 | 默认值 | 说明 |
| --- | --- | --- |
| `host` | (必填) | SMTP 服务器主机名。 |
| `port` | `587` | SMTP 服务器端口。 |
| `username` | `` | SMTP 认证用户名;为空表示匿名。 |
| `password` | `` | SMTP 认证密码。 |
| `auth-type` | `auto` | 认证机制:`auto`/`plain`/`login`/`cram-md5`。 |
| `from` | `` | `Message` 未设 `From` 时使用的默认发件人。 |
| `timeout` | `10s` | 限定启动探测与每次发信的拨号时长。 |
| `ping` | `false` | 启动时拨一次服务器,提前拦截坏配置。 |
| `tls.mode` | `starttls` | 传输安全:`starttls`/`tls`/`none`。 |
| `tls.insecure-skip-verify` | `false` | 关闭证书校验(仅测试用)。 |

## 设计说明

* **`Send` 是非幂等的。** 重发会投递第二封邮件,因此无论策略配置了多少次重试,
  执行器都只跑一次 `Send`——重试永不重复投递。
* **观测只声明、不发射。** `Send` 以 `email.client` 前缀声明本次操作,并以收件人
  **数量**作为 `Detail`(绝不是地址或主题——那些是个人数据);resilience 发射点产出
  span、时长直方图与访问日志,失败的发送在 Warn 级别写出 detail。
* **`tls.mode` 是 mode 枚举,不是 `enabled` + 证书文件。** SMTP 有三种线路行为
  (STARTTLS、隐式 TLS、明文),一个布尔表达不了。
* **正文形状由 `Text` / `HTML` 决定。** 两者都设 → `multipart/alternative`;
  只设一个 → 该正文;都为空 → 空的纯文本正文。
* **`insecure-skip-verify` 仍钉 `ServerName`。** 它取 `host`,因此在重新开启校验后
  通配符证书依然能匹配名字。
* **不做连接池。** SMTP 服务通常按连接限速,故每次 `Send` 现拨现关——池是被有意
  省去的。

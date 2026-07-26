# starter-mail 设计

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-mail` 属于 Client 形态（`starter/DESIGN.md` §2.2），基于
`github.com/wneessen/go-mail` 提供 SMTP mailer。starter 很小但有三个
非显然决定值得写下来：不接 `destroy`、TLS 走 mode 非 enabled+证书、
启动时预拨号。

## 1. 职责与边界

- 用 `gs.Group` 把 `${spring.mail}` 每条绑到 `*Mailer` bean。不做默认单实例
  （`project_client_starter_multiinstance`），按名注入（如
  `autowire:"notify"`）。
- Send 每次现开连接、投递、关连接。无长连接，故不接 `destroy`。
- 不带模板引擎。调用方自行渲染 HTML/文本，把成品字符串传进来；模板不是
  邮件库应管的事。

## 2. 关键抽象与缝隙

- **TLS 是 mode 枚举，不是 flag+cert。** `tls.mode` 三选一：`starttls`
  （默认） / `tls`（465 隐式 TLS） / `none`。这与其他 starter 的
  `tls.enabled=true` 形状不同，因为 SMTP 有三种线路行为而不是两种
  （`project_starter_mail`）。
- **启动预拨号 fail-fast。** `newMailer` 拨一次、关一次，让 host/port/auth/TLS
  错在启动就暴露，而不是等首封邮件。对 mailer 特别重要——首封往往是
  运维告警。
- **不接 `destroy`。** `DialAndSendWithContext` 每次 Send 现拨现关。注册
  destroy hook 会去关一个不持任何资源的 client。
- **Auth 可选。** `Username` 空时用 `SMTPAuthNoAuth`（受信内网 open relay）。
  否则 `auth-type` 字符串映射到 `plain / login / cram-md5 / auto`。
- **Message 形状有意窄。** From（消息级覆盖或 mailer 级默认）、To/Cc/Bcc、
  subject、plain+html+附件。不做日历/富类型——那属于调用方。

## 3. 约束

- **`Host` 必填。** 无 localhost 默认；缺 host 启动被拒。
- **`Text` 与 `HTML` 决定 body 形状。** 都有 → multipart/alternative；
  只有一个 → 该 body；都空 → plain-text 空 body。
- **至少一个 `To`。** 仅 `Cc`/`Bcc` 不满足——符合 SMTP 实际。
- **`InsecureSkipVerify` 仍钉 `ServerName`。** 从 `Host` 取值，让通配符
  证书在重新启用校验时仍能过名字匹配。

## 4. 权衡 / 已否决方案

- **连接池——否决。** SMTP 服务常按连接限速；池化会鼓励长连接违反
  server 侧连接上限并让凭据轮换复杂化。per-send 拨号跟 SMTP 实际对齐。
- **内建模板引擎——否决。** 模板（`html/template`、`text/template`、
  sprig、mjml）是调用点决策；硬塞一个进 starter 会锁死用户选择。
- **`tls.enabled=true` + `tls.cert-file` 形状——否决。** SMTP 有三种
  线路行为（25/587 上的 STARTTLS、465 上的隐式 TLS、明文）；一个布尔
  区分不了 STARTTLS 与隐式 TLS。

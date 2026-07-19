# starter-mail

[English](README.md) | [中文](README_CN.md)

`starter-mail` provides an SMTP mailer wrapper based on
github.com/wneessen/go-mail, making it easy to send transactional email
(notifications, alerts, verification codes) from Go-Spring applications. The
library is pure Go, so cross-compilation stays clean.

It sends only — there is no IMAP/POP3 receiving — and ships no template engine:
the caller renders the HTML body itself and passes the final string in, so you
keep full control over templating.

## Installation

```bash
go get go-spring.org/starter-mail
```

## Quick Start

### 1. Import the `starter-mail` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-mail"
```

### 2. Configure the Mailers

Define one or more named mailers under `spring.mail.<name>` in your project's
[configuration file](example/conf/app.properties). The host is required and there
is no localhost fallback — a missing or unreachable server fails at startup.

```properties
spring.mail.notify.host=smtp.example.com
spring.mail.notify.port=587
spring.mail.notify.username=apikey
spring.mail.notify.password=${SMTP_PASSWORD}
spring.mail.notify.from=noreply@example.com
spring.mail.notify.tls.mode=starttls
```

### 3. Inject the Mailer

Refer to the [example.go](example/example.go) file. Each named instance is
registered as a `*Mailer` bean under that name. There is no default singleton —
select an instance by name; adding a second mailer is a pure-config change.

```go
import StarterMail "go-spring.org/starter-mail"

type Service struct {
    Notify *StarterMail.Mailer `autowire:"notify"`
}
```

### 4. Send an Email

Refer to the [example.go](example/example.go) file. Build a `Message` and call
`Send`. A plain-text body, an HTML body, or both (multipart/alternative) may be
supplied, along with multiple recipients and attachments.

```go
err := s.Notify.Send(ctx, &StarterMail.Message{
    To:      []string{"alice@example.com", "bob@example.com"},
    Cc:      []string{"carol@example.com"},
    Subject: "Welcome",
    Text:    "Plain-text fallback body.",
    HTML:    "<h1>Hello</h1><p>An <b>HTML</b> mail.</p>",
    Attachments: []StarterMail.Attachment{
        {Filename: "report.txt", Data: reportBytes},
    },
})
```

Each `Send` opens one connection, delivers all supplied messages, and closes it,
so no long-lived socket is held between calls (and no destroy hook is needed).

## Core Features

The [example](example/example.go) self-asserts sending against a live MailHog
server: one message with an HTML body, a plain-text alternative, an attachment,
and multiple recipients (To/Cc), then verifies delivery through MailHog's HTTP
API.

* **Multiple recipients**: `To`, `Cc`, and `Bcc` are address lists; at least one
  `To` is required.
* **HTML + attachments**: set `HTML` (optionally with `Text` as a fallback) and
  attach files by passing their bytes in `Attachments`.
* **Multiple mailers**: every entry under `spring.mail` becomes an independently
  configured `*Mailer` bean; inject them by name to send through different
  servers or from different senders.
* **Fail-fast**: a missing host, an unknown auth/TLS mode, or an unreachable
  server surfaces at startup via a bounded connection probe, not on the first
  send.

## Authentication and TLS

Authentication is enabled by setting `username` (and `password`). `auth-type`
selects the mechanism: `auto` (negotiate the strongest the server offers, the
default), `plain`, `login`, or `cram-md5`. With no `username` the mailer connects
anonymously.

`tls.mode` selects transport security:

* `starttls` (default): connect in plaintext then upgrade via STARTTLS; the
  connection fails if the server does not offer it. Typically port 587.
* `tls`: implicit TLS from the first byte (SMTPS), typically port 465.
* `none`: no encryption; for local test servers only.

## Configuration

Each mailer under `spring.mail.<name>` reads the following properties:

| Property | Default | Description |
| --- | --- | --- |
| `host` | (required) | SMTP server hostname. |
| `port` | `587` | SMTP server port. |
| `username` | `` | SMTP auth username; empty means anonymous. |
| `password` | `` | SMTP auth password. |
| `auth-type` | `auto` | Auth mechanism: `auto`/`plain`/`login`/`cram-md5`. |
| `from` | `` | Default sender used when a `Message` sets no `From`. |
| `timeout` | `10s` | Bounds the startup probe and each send's dial. |
| `tls.mode` | `starttls` | Transport security: `starttls`/`tls`/`none`. |
| `tls.insecure-skip-verify` | `false` | Disable certificate verification (testing only). |

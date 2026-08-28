# starter-mail Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `trace.go`) and the runnable
[example/](example/) / [example-otel/](example-otel/). **SMTP and message semantics (headers,
MIME, attachments, auth mechanisms) are [go-mail's documentation](https://github.com/wneessen/go-mail)
and [RFC 5321](https://www.rfc-editor.org/rfc/rfc5321)** — everything below is go-spring's
increment: configuration, wiring, fail-fast startup, tracing helpers.

**Activation**: every `spring.mail.<name>` subtree creates exactly one `*Mailer` bean named
`<name>`. No `spring.mail.*` keys → no beans, no startup dial, starter is inert. There is no
`enabled` key and no default singleton.

---

## 1. Complete worked project

A notification service that sends HTML+attachment mail through a named mailer, with tracing via
starter-otel. File tree (mirrors [example-otel/](example-otel/), which is smoke-verified against
MailHog + Jaeger via docker-compose):

```
demo/
├── go.mod
├── main.go
├── notify.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    github.com/wneessen/go-mail     latest   # transitive, pulled by the starter
    go-spring.org/spring            v1.3.x
    go-spring.org/starter-mail      latest
    go-spring.org/starter-otel      latest   # optional: real span export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**notify.go** — the application's entire mail surface:

```go
package notify

import (
    "context"

    "go-spring.org/spring/gs"

    StarterMail "go-spring.org/starter-mail"
)

// Service wires one named mailer. A second mailer (e.g. a marketing host) is a
// pure config change plus another autowire field — no starter code edit.
type Service struct {
    Notify *StarterMail.Mailer `autowire:"notify"`
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// SendReport renders the body itself (the starter ships no template engine,
// by design) and sends one multipart/alternative mail with an attachment.
func (s *Service) SendReport(ctx context.Context) error {
    ctx, span := StarterMail.StartSendSpan(ctx, "daily-report")
    defer StarterMail.EndSpan(span, nil) // set err if Send fails; see below
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

**conf/app.properties** — the complete, commented surface:

```properties
# --- mailer "notify" (one instance per spring.mail.<name> subtree) ------------
spring.mail.notify.host=smtp.example.com
spring.mail.notify.port=587
spring.mail.notify.username=apikey
spring.mail.notify.password=${SMTP_PASSWORD}
spring.mail.notify.auth-type=auto
spring.mail.notify.from=noreply@example.com
spring.mail.notify.timeout=10s
spring.mail.notify.tls.mode=starttls
# spring.mail.notify.tls.insecure-skip-verify=false   # test only

# --- observability (starter-otel), verified in example-otel/ ------------------
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.trace.sampler-ratio=1.0
```

For local verification swap the mailer block for the MailHog lines from
[example/conf/app.properties](example/conf/app.properties) (`host=127.0.0.1 port=1025
tls.mode=none from=noreply@example.com`) and start
`docker compose -f example/docker-compose.yml up -d` (SMTP :1025, Web UI :8025).

**Verify** (isomorphic to the example's own assertions):

```bash
go run .                                          # boots; watch "mailer created host=..." log
curl -s :8025/api/v2/messages | jq .total         # MailHog: >=1 delivered message
curl -s :8025/api/v2/messages | jq '.messages[0].Content.Headers'   # recipients, subject, attachment
curl -s 'http://127.0.0.1:16686/api/traces?service=demo' | jq '.data[0].spans[].operationName'  # "mail.send" (example-otel setup)
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-mail
  └─ init(): gs.Group("${spring.mail}", newMailer, nil)     [one instance per spring.mail.<name>]

gs.Run()
  ├─ config bind: spring.mail.<name>.* → Config (value tags; host required via errutil.RequireField)
  ├─ newMailer: parse auth (only if username set) → TLS mode → mail.NewClient
  ├─ fail-fast probe: client.DialWithContext (bounded by timeout) then Close
  │     └─ bad host/port/auth/TLS ⇒ startup ERROR, app refuses to boot (source comment:
  │        "so a misconfiguration surfaces at boot rather than on the first send")
  ├─ bean ready: *Mailer injected wherever `autowire:"<name>"` appears
  ├─ Run / serve: nothing background — no goroutines, no pooled sockets
  └─ SIGTERM: no destroy hook (each Send dials and closes per call; nothing to release)
```

Design rationale from source comments: the group registration documents *why* there is no
destroy callback — "the underlying client opens a fresh connection per Send and closes it when
done, so there is nothing to release at shutdown" (starter.go init / Mailer doc).

### 2.2 One send, layer by layer

`Mailer.Send(ctx, msgs...)` (starter.go:169):

1. **Build phase** — `m.build(msg)` per message: resolve From (`msg.From` → else the mailer's
   configured `from`; both empty → error "no From address"); validate To (required), Cc, Bcc
   addresses via go-mail's parsers; assemble the body — Text+HTML ⇒ `multipart/alternative`
   (recipient's client picks the richest part), HTML-only ⇒ HTML body, else plain Text;
   attach files from in-memory bytes (`AttachReader`).
2. **Delivery phase** — a single `DialAndSendWithContext` opens one SMTP connection, delivers
   all messages, and closes it. Partial failure semantics are go-mail's; the starter wraps any
   error with `errutil.Explain(err, "mail: send failed")`.
3. **Tracing (opt-in, caller-side)** — if the app called `StartSendSpan` (trace.go:41), the
   span `mail.send` (SpanKind client, attributes `messaging.system=smtp`,
   `messaging.operation=send`, `mail.purpose=<purpose>`) records the error and ends via
   `EndSpan`. Without starter-otel the OTel global TracerProvider is a no-op — nothing is sent
   on the wire, nothing warns.

Note the trace span is **not** inside `Send`: the starter exposes the two helpers and the
caller brackets the call (recorded as a design suspect in §6).

---

## 3. Per-key behavior reference

Prefix `spring.mail.<name>.*` — bound into the ctor `Config` (config.go), so keys ARE
instance-prefixed. (The starter has no wrapper-field value tags; nothing here is top-level.)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `host` | string | — | **Required** (`errutil.RequireField` in newMailer). No localhost fallback. | Missing → startup fails fast with a required-field error. |
| `port` | int | 587 | Passed to `mail.WithPort`. | ⚠ 465 without `tls.mode=tls` → implicit-TLS server spoken plaintext → dial fails at startup. 587 with `tls.mode=tls` likewise fails. |
| `username` | string | — | Non-empty turns SMTP auth on with `password`; empty ⇒ `WithSMTPAuth(SMTPAuthNoAuth)` (anonymous). | Password without username is silently ignored. |
| `password` | string | — | Used only when username set. | Wrong password → startup dial fails (fail-fast). |
| `auth-type` | string | auto | `auto`\|`plain`\|`login`\|`cram-md5` (parseAuthType, lower-cased; `crammd5` alias accepted). Only consulted when username non-empty. | Unknown value → startup error listing valid values. |
| `from` | string | — | Default sender; per-message `Message.From` overrides. | Empty from + message without From → Send-time error, not startup. |
| `timeout` | duration | 10s | Bounds BOTH the startup probe and each send's dial (`WithTimeout` + probe context). | Too small → spurious startup/send timeouts on slow relays. |
| `tls.mode` | string | starttls | `starttls` (empty = same, mandatory upgrade) \| `tls`/`ssl` (implicit TLS) \| `none` (plaintext, test only). Unknown → startup error. | `none` in production sends credentials in the clear; wrong mode vs port → dial failure at boot. |
| `tls.insecure-skip-verify` | bool | false | When true installs a TLS config with `InsecureSkipVerify` (ServerName=host). | Test-only; in production it accepts forged server certificates — silent MITM exposure. |

The `value:"${tls}"` sub-struct binding means `tls.*` keys live under the same instance
prefix (`spring.mail.<name>.tls.mode`), not at top level.

---

## 4. Verification & fault drills

### 4.1 Delivery drill (MailHog, same path as example/check.sh)

```bash
cd example && docker compose up -d && go run .      # prints "mail sent and delivered OK"
curl -s :8025/api/v2/messages | jq '.total'                          # 1
curl -s :8025/api/v2/messages | jq '.messages[0].To'                 # alice, bob, carol
```

The example asserts `total >= 1` after sending one message with To×2 + Cc×1 and one attachment
(example.go runTest).

### 4.2 Fail-fast drill

Set `spring.mail.notify.port=9999` (nothing listening) and boot: startup aborts with
`mail: startup dial to ...:9999 failed`. This is the intended posture — the probe
(newMailer, starter.go:137-144) exists precisely so bad config never reaches first-send.

### 4.3 TLS posture drill

Point `host/port` at a real submission server with `tls.mode` mismatched (e.g. `none` on 587
against a STARTTLS-only relay): boot fails on the probe. Flip to `starttls` → boots. The probe
catches TLS-mode mistakes for free because it performs the same negotiation a Send would.

### 4.4 Trace drill (example-otel)

```bash
cd example-otel && docker compose up -d && go run .   # asserts traces in Jaeger itself
curl -s 'http://127.0.0.1:16686/api/traces?service=mail-otel-example&limit=1' | grep '"data":\['
```

Span fields to read: operation `mail.send`, kind CLIENT, attributes `messaging.system=smtp`,
`mail.purpose` (the string you passed), error event + status Error on failure.

### 4.5 From-fallback drill

Remove `from` from config, keep the app sending `Message` without `From` → every Send returns
`mail: no From address (set message.From or spring.mail...from)`. Add either side back to heal.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup aborts: "startup dial ... failed" | host/port wrong, server down, or TLS mode vs port mismatch | Fix the triple (port ↔ tls.mode ↔ server offering); the probe message names host:port. |
| Startup aborts: "unknown tls mode / auth-type" | typo in enum value | Use starttls\|tls\|none and auto\|plain\|login\|cram-md5 (case-insensitive). |
| Startup aborts: required field host | `spring.mail.<name>.host` missing | Set it; there is no localhost default. |
| Send fails "no From address" | neither config `from` nor `Message.From` | Set one (or both — message wins). |
| Send fails "message has no recipients" | `To` empty | To is mandatory; Cc/Bcc alone are not enough. |
| Send works at boot, fails later "send failed" | relay restarted / credentials expired / timeout too small | Raise `timeout`; the probe only proves boot-time health — see suspect §6. |
| Attachments missing | passed nil Data or wrong filename only | Attachment is name+bytes; the mailer never reads disk. |
| No spans anywhere | starter-otel not imported, or StartSendSpan not called | Both are needed — the span bracket is caller-side by design. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 10 (incl. 2 `tls.*`) |
| Required | 1 (`host`) |
| Quickstart external deps | 1 (SMTP server / MailHog) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger; first two carried over from the previous edition):
- Trace requires manual `StartSendSpan`/`EndSpan` instead of living inside `Send` — caller-side
  boilerplate; consider wrapping Send itself when starter-otel is present.
- No health indicator despite a startup dial probe existing — the probe result is not exposed
  at runtime; boot-time health and steady-state health are conflated.
- Fail-fast probe costs one SMTP login per boot per instance against quota-limited relays
  (e.g. verified-sender APIs); acceptable but worth remembering at instance-count scale.
- `port` ↔ `tls.mode` coupling is validated only implicitly by the probe — a friendlier
  startup error naming the pairing would save a troubleshooting round.

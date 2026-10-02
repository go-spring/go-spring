# starter-mail Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `observe.go`) and the runnable
[example/](example/) / [example-otel/](example-otel/). **SMTP and message semantics (headers,
MIME, attachments, auth mechanisms) are [go-mail's documentation](https://github.com/wneessen/go-mail)
and [RFC 5321](https://www.rfc-editor.org/rfc/rfc5321)** — everything below is go-spring's
increment: configuration, wiring, an opt-in startup probe, operation declaration.

**Activation**: every `spring.mail.instances.<name>` subtree creates exactly one `*Mailer` bean named
`<name>`. No `spring.mail.instances.*` keys → no beans, no startup dial, starter is inert. There is no
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
# --- mailer "notify" (one instance per spring.mail.instances.<name> subtree) ------------
spring.mail.instances.notify.host=smtp.example.com
spring.mail.instances.notify.port=587
spring.mail.instances.notify.username=apikey
spring.mail.instances.notify.password=${SMTP_PASSWORD}
spring.mail.instances.notify.auth-type=auto
spring.mail.instances.notify.from=noreply@example.com
spring.mail.instances.notify.timeout=10s
spring.mail.instances.notify.tls.mode=starttls
# spring.mail.instances.notify.tls.insecure-skip-verify=false   # test only

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
  └─ init(): gs.Module(OnProperty("spring.mail.instances"), ...)  [one instance per spring.mail.instances.<name>]

gs.Run()
  ├─ config bind: spring.mail.instances.<name>.* → Config (value tags; host required via errutil.RequireField)
  ├─ newMailer: parse auth (only if username set) → TLS mode → mail.NewClient
  ├─ probe (when ping=true): client.DialWithContext (bounded by timeout) then Close
  │     └─ bad host/port/auth/TLS ⇒ startup ERROR, app refuses to boot (source comment:
  │        "so a misconfiguration surfaces at boot rather than on the first send")
  ├─ governance applied in the ctor: exec = cloud.ClientParams{...}.ExecutorFor("mail", label)
  ├─ bean ready: *Mailer injected wherever `autowire:"<name>"` appears
  ├─ Run / serve: nothing background — no goroutines, no pooled sockets
  └─ SIGTERM: destroy hook closes the resilience executor (the SMTP client owns no live resource)
```

A `gs.Module` (not `gs.Group`) is used so the constructor can take the governance beans
alongside its config. The SMTP client opens a fresh connection per Send and closes it when
done, so there is no pooled socket to release — the only resource the `Destroy` hook releases
is the resilience executor.

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
3. **Declaration** — `Send` declares the batch's semantic identity
   (`observability.Operation`: span name `mail.send`, metric prefix `email.client`, the
   bounded labels `email.system=smtp` / `email.operation=send`, the batch size as `Detail`,
   and `NonIdempotent: true`) on the context via `observability.WithOperation`, then routes
   the delivery through the mailer's resilience executor (`resilience.Run`). A send
   **declares**; it emits nothing itself.
   - The **detail is the recipient count, never the recipients or the subject**: both are
     personal data and a failed send writes its detail at Warn unconditionally, so the
     addresses would be recorded exactly when the send is going wrong. They stay in the
     caller's own records.
   - The **non-idempotent marker** is what stops a configured retry: a resend is a second
     email, not a second attempt. See §2.5.
4. **Emission (the resilience layer)** — the observe wrapper on the executor chain is the
   single **emitter**: it opens the call span `mail.send` (covering every attempt), records
   the call-level `email.client.operation.duration`, the per-attempt
   `email.client.attempt.duration` histogram (the downstream's own latency, per try), the
   in-flight `email.client.active_requests` gauge, and the `resilience.client.calls` counter,
   and writes one access-log line per send under the `mail`/`access` tag carrying the declared
   `email.*` fields plus `status` and `duration_ms` (and `error` on failure). The emitter
   levels the log: failure → Warn, success **with** detail → Debug, success without detail →
   Info. A send always carries its recipient count, so a successful send logs at Debug. With
   starter-otel absent the OTel globals are no-ops — nothing is sent on the wire, nothing
   warns.

The span, the metrics and the access log all live **inside** the executor that `Send` routes
through, so a send is measured whether or not the caller ever held a span — there is no
caller-side bracket to remember.

### 2.3 Retry semantics: a send is never retried

`Send` runs through the mailer's resilience executor, so a governance rule for its service
label (`mail:<name or host>`) *can* carry a retry policy — and that policy is **ignored**: the
operation is declared `NonIdempotent`, and the executor runs it once regardless of
`max-retries`. A retry would deliver a second copy of the mail, which no downstream stage can
take back.

The suppression is not silent: the first time a call would have been retried, the executor
logs one warning per service (`resilience: retries configured for service "mail:..." are
suppressed — its operations are non-idempotent ...`). Timeouts, rate limiting, the circuit
breaker and the bulkhead all still apply — only the retry stage is dropped.

---

## 3. Per-key behavior reference

Prefix `spring.mail.instances.<name>.*` — bound into the ctor `Config` (config.go), so keys ARE
instance-prefixed. (The starter has no wrapper-field value tags; nothing here is top-level.)

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `host` | string | — | **Required** (`errutil.RequireField` in newMailer). No localhost fallback. | Missing → startup fails fast with a required-field error. |
| `port` | int | 587 | Passed to `mail.WithPort`. | ⚠ 465 without `tls.mode=tls` → implicit-TLS server spoken plaintext → dial fails at startup. 587 with `tls.mode=tls` likewise fails. |
| `username` | string | — | Non-empty turns SMTP auth on with `password`; empty ⇒ `WithSMTPAuth(SMTPAuthNoAuth)` (anonymous). | Password without username is silently ignored. |
| `password` | string | — | Used only when username set. | Wrong password → startup dial fails (when ping=true). |
| `auth-type` | string | auto | `auto`\|`plain`\|`login`\|`cram-md5` (parseAuthType, lower-cased; `crammd5` alias accepted). Only consulted when username non-empty. | Unknown value → startup error listing valid values. |
| `from` | string | — | Default sender; per-message `Message.From` overrides. | Empty from + message without From → Send-time error, not startup. |
| `timeout` | duration | 10s | Bounds both the startup probe (when `ping=true`) and each send's dial (`WithTimeout` + probe context). | Too small → spurious startup/send timeouts on slow relays. |
| `ping` | bool | false | Opt-in startup probe: `DialWithContext` then `Close` so bad host/port/auth/TLS aborts boot [starter.go:171-180]. | true → a bad triple aborts boot; false → surfaces on the first Send. |
| `tls.mode` | string | starttls | `starttls` (empty = same, mandatory upgrade) \| `tls`/`ssl` (implicit TLS) \| `none` (plaintext, test only). Unknown → startup error. | `none` in production sends credentials in the clear; wrong mode vs port → dial failure at boot. |
| `tls.insecure-skip-verify` | bool | false | When true installs a TLS config with `InsecureSkipVerify` (ServerName=host). | Test-only; in production it accepts forged server certificates — silent MITM exposure. |

The `value:"${tls}"` sub-struct binding means `tls.*` keys live under the same instance
prefix (`spring.mail.instances.<name>.tls.mode`), not at top level.

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

### 4.2 Ping drill

Set `spring.mail.instances.notify.port=9999` (nothing listening), add
`spring.mail.instances.notify.ping=true`, and boot: startup aborts with
`mail: startup dial to ...:9999 failed`. This is the opt-in posture — the probe
(newMailer, starter.go:171-180) exists so bad config can be caught before first-send when the
operator asks for it; without `ping=true` the same config boots and fails on the first Send.

### 4.3 TLS posture drill

Point `host/port` at a real submission server with `tls.mode` mismatched (e.g. `none` on 587
against a STARTTLS-only relay) and set `ping=true`: boot fails on the probe. Flip to `starttls`
→ boots. The probe catches TLS-mode mistakes for free because it performs the same negotiation
a Send would.

### 4.4 Trace drill (example-otel)

```bash
cd example-otel && docker compose up -d && go run .   # asserts traces in Jaeger itself
curl -s 'http://127.0.0.1:16686/api/traces?service=mail-otel-example&limit=1' | grep '"data":\['
```

Span fields to read: operation `mail.send`, attributes `email.system=smtp`,
`email.operation=send`, the declared detail `email.recipients.count`, plus
`status`, and an error event + status Error on failure. The span is opened by the resilience
emitter (the single emitter on the executor chain), not by the starter.

### 4.5 From-fallback drill

Remove `from` from config, keep the app sending `Message` without `From` → every Send returns
`mail: no From address (set message.From or spring.mail.instances...from)`. Add either side back to heal.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup aborts: "startup dial ... failed" | host/port wrong, server down, or TLS mode vs port mismatch | Fix the triple (port ↔ tls.mode ↔ server offering); the probe message names host:port. |
| Startup aborts: "unknown tls mode / auth-type" | typo in enum value | Use starttls\|tls\|none and auto\|plain\|login\|cram-md5 (case-insensitive). |
| Startup aborts: required field host | `spring.mail.instances.<name>.host` missing | Set it; there is no localhost default. |
| Send fails "no From address" | neither config `from` nor `Message.From` | Set one (or both — message wins). |
| Send fails "message has no recipients" | `To` empty | To is mandatory; Cc/Bcc alone are not enough. |
| Send works at boot, fails later "send failed" | relay restarted / credentials expired / timeout too small | Raise `timeout`; the probe only proves boot-time health — see suspect §6. |
| Attachments missing | passed nil Data or wrong filename only | Attachment is name+bytes; the mailer never reads disk. |
| No spans anywhere | starter-otel not imported | Import starter-otel; the resilience emitter opens the send's span automatically — no caller-side bracket. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 11 (incl. 2 `tls.*`) |
| Required | 1 (`host`) |
| Quickstart external deps | 1 (SMTP server / MailHog) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger):
- No health indicator despite an opt-in startup dial probe existing — the probe result is not
  exposed at runtime; boot-time health and steady-state health are conflated.
- The opt-in ping probe costs one SMTP login per boot per instance against quota-limited relays
  (e.g. verified-sender APIs); acceptable but worth remembering at instance-count scale.
- `port` ↔ `tls.mode` coupling is validated only implicitly by the probe — a friendlier
  startup error naming the pairing would save a troubleshooting round.

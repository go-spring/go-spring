# starter-mail Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-mail` is a Client-archetype starter (`starter/DESIGN.md` §2.2)
that provisions SMTP mailers backed by `github.com/wneessen/go-mail`.
It is a small starter but has three non-obvious decisions worth pinning:
observability declares (it does not emit), TLS mode ≠ enabled+cert-files, and
an opt-in startup-time dial.

## 1. Responsibilities & Boundaries

- Binds each `${spring.mail}` entry to a `*Mailer` bean via `gs.Module`
  (a module, not a group, so the ctor can take the governance beans beside
  its config).
  No single-instance default (`project_client_starter_multiinstance`);
  select one by name (e.g. `autowire:"notify"`).
- Send opens a fresh connection, delivers, closes. There is no
  long-lived socket, so `destroy` releases only the resilience executor.
- Ships no template engine. Callers render HTML/text themselves and pass
  the finished strings; a mail library is not the right place for
  templating.

## 2. Key Abstractions & Seams

- **Observability declares, it does not emit.** `Send` puts the send's
  semantic identity (`observability.Operation`: span name, metric prefix
  `email.client`, bounded `email.*` labels, the batch size as `Detail`, and the
  non-idempotent marker) on the context and routes through the resilience
  executor. The executor's observe wrapper is the single emitter — span,
  duration metrics (call-level `.operation.duration` and attempt-level
  `.attempt.duration`) and the access log. Detail reaches the span and the log
  but never a metric label, and it carries the recipient **count** rather than
  the addresses or the subject: those are personal data, and a failed send logs
  its detail at Warn.
- **TLS is a mode enum, not a flag+certs.** `tls.mode` chooses among
  `starttls` (default) / `tls` (implicit TLS on port 465) / `none`. This
  differs from every other starter's `tls.enabled=true` shape because
  SMTP has three distinct wire behaviours, not two (`project_starter_mail`).
- **The startup dial is an opt-in probe.** With `ping=true` `newMailer` dials
  once and closes so a bad host/port/auth/TLS shows up at boot instead of on the
  first send; off by default so a not-yet-up relay does not block startup.
  For a mailer this matters — the first `Send` may be an ops alert.
- **No pooled resource, a bounded destroy.** `DialAndSendWithContext` dials
  per Send and closes when done, so `destroy` has nothing to close on the SMTP
  client — it releases only the resilience executor the mailer owns.
- **Auth is optional.** With `Username` empty the mailer uses
  `SMTPAuthNoAuth` (open relay in a trusted network). Otherwise the
  `auth-type` string maps to `plain / login / cram-md5 / auto`.
- **Message shape is deliberately narrow.** From (per-message override
  or per-mailer default), To/Cc/Bcc, subject, plain+html+attachments.
  No calendar/rich types — those belong to the caller.

## 3. Constraints

- **`Host` is required.** No localhost default; missing host is rejected
  at boot.
- **`Text` and `HTML` presence chooses the body shape.** Both set →
  multipart/alternative; only one set → that body; both empty →
  plain-text empty body.
- **At least one `To`.** `Cc`/`Bcc` alone do not satisfy — matches SMTP
  reality.
- **`InsecureSkipVerify` still pins `ServerName`.** Set from `Host` so
  a wildcard cert can still validate name-matching when re-enabled.

## 4. Trade-offs / Alternatives Rejected

- **Connection pooling — rejected.** SMTP servers routinely rate-limit
  per connection; a pool encourages long-lived connections that violate
  server-side connection limits and complicate credential rotation.
  Per-send dial keeps behaviour aligned with SMTP realities.
- **Template engine bundled in — rejected.** Templating (`html/template`,
  `text/template`, sprig, mjml) is a call-site choice; forcing one into
  the starter locks callers out of their preferred one.
- **`tls.enabled=true` + `tls.cert-file` shape — rejected.** SMTP has
  three wire behaviours (STARTTLS on port 25/587, implicit TLS on 465,
  plaintext); a boolean cannot pick between STARTTLS and implicit TLS.

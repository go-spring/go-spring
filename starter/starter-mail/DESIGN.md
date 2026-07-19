# starter-mail Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-mail` is a Client-archetype starter (`starter/DESIGN.md` §2.2)
that provisions SMTP mailers backed by `github.com/wneessen/go-mail`.
It is a small starter but has three non-obvious decisions worth pinning:
no `destroy`, TLS mode ≠ enabled+cert-files, and startup-time dial.

## 1. Responsibilities & Boundaries

- Binds each `${spring.mail}` entry to a `*Mailer` bean via `gs.Group`.
  No single-instance default (`project_client_starter_multiinstance`);
  select one by name (e.g. `autowire:"notify"`).
- Send opens a fresh connection, delivers, closes. There is no
  long-lived socket, hence no `destroy` callback.
- Ships no template engine. Callers render HTML/text themselves and pass
  the finished strings; a mail library is not the right place for
  templating.

## 2. Key Abstractions & Seams

- **TLS is a mode enum, not a flag+certs.** `tls.mode` chooses among
  `starttls` (default) / `tls` (implicit TLS on port 465) / `none`. This
  differs from every other starter's `tls.enabled=true` shape because
  SMTP has three distinct wire behaviours, not two (`project_starter_mail`).
- **Startup dial is fail-fast.** `newMailer` dials once and closes so a
  bad host/port/auth/TLS shows up at boot instead of on the first send.
  For a mailer this matters — the first `Send` may be an ops alert.
- **No `destroy`.** `DialAndSendWithContext` dials per Send and closes
  when done. Registering a destroy hook would try to close a client
  that owns no live resource.
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

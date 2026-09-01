# starter-webhook Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A thin client-archetype starter (`starter/DESIGN.md` §2.2) modeled on
starter-mail: stateless per call, `gs.Group` with no destroy hook, trace
helpers riding the OTel globals. The one deliberate divergence from mail is
the resilience seam, which every client starter in this repo carries.

## 1. Responsibilities & Boundaries

- **Owns**: the per-instance notifier group under `${spring.webhook}`,
  channel payload building + signing, the POST path, per-send resilience and
  observe spans, and the non-2xx / vendor-error surface.
- **Does not own**: templating (callers render `Text` themselves, same
  stance as starter-mail ships no template engine), retry policies beyond
  what the governance executor provides, email/SMS delivery, and receiver
  rotation (rotate by config).

## 2. Key Abstractions & Seams

- **`Notifier` (starter.go)** — one endpoint + one channel + one signing
  secret; `Send` builds the payload, wraps the POST in a `webhook.send`
  producer span, and routes it through `fault.WrapExecutor(
  resilience.ExecutorFor("webhook:<name>:<channel>"), fault.InjectorFor())`
  wrapped by `resilience.WrapExecutor` — the same neutral-seam stack every
  client starter uses, zero coupling to starter-governance.
- **`buildPayload` (payload.go)** — pure function channel → (body, extra
  query). DingTalk's 加签 appends `timestamp`/`sign` to the URL; Feishu's
  signature rides the body; both are HMAC-SHA256 over `<millis>\n<secret>`.
  Pure functions make the formats unit-testable without any network.
- **Trace helpers (trace.go)** — `StartSendSpan` / `EndSpan` mirror
  starter-mail's pattern; spans carry `webhook.channel` and only the
  destination host (full URLs with tokens stay out of telemetry).

## 3. Constraints

- No startup probe, unlike mail's dial probe: the only universal webhook
  probe is a real POST, and a boot-time junk notification to a production
  chat channel is a worse failure mode than a first-send error. A cheap
  validation still happens at construction — an unknown `channel` fails
  fast, and URL errors surface on first send with the endpoint in the
  message.
- No `health.Indicator`: stateless client, same stance as mail.
- Vendor quirks accepted: DingTalk/Feishu may answer 200 with an error body
  in some setups; a non-2xx is always an error, and a 200-with-error-body
  surfaces only when the receiver chooses a non-200 status. Channels that
  need body-level error parsing (rare) belong in a custom extension of
  `buildPayload`'s shape.

## 4. Trade-offs / Alternatives Rejected

- **Per-channel payload builders vs. an abstraction layer**: the five
  formats are each ~10 lines of JSON shaping; an interface-per-channel
  plugin family would out-weigh the code it organizes. `buildPayload` stays
  a pure switch so adding a channel is one function.
- **Resilience on Send vs. none (mail parity)**: notifications often target
  rate-limited chat endpoints; the guard costs one interface call and
  degrades to pass-through without governance, so the asymmetry with mail
  (whose SMTP path is connection-oriented and fails loudly already) is
  justified.
- **stdlib-only vs. a notification SDK**: no meaningful Go SDK exists for
  the five formats combined; hand-rolled JSON keeps the module dependency-
  free and auditable.

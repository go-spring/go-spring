# starter-webhook Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `payload.go`, `trace.go`,
`webhook_test.go`) and the self-contained [example/](example/) (spins its own local HTTP
receiver; smoke-verified with `go run .`, no docker). **This starter IS the component** —
payload formats and signing for generic/DingTalk/Feishu/WeCom/Slack are implemented here;
receiver-side setup is each platform's bot docs
([DingTalk](https://open.dingtalk.com/document/robots/custom-robot-access),
[Feishu](https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot),
[WeCom](https://developer.work.weixin.qq.com/document/path/91770),
[Slack](https://api.slack.com/messaging/webhooks)).

**Activation**: every `spring.webhook.<name>` subtree creates one `*Notifier` bean named
`<name>`. No keys → no beans. There is deliberately **no startup probe** — the only universal
probe would be a real POST, and "sending a junk notification at boot is worse than failing on
first use" (source comment, starter.go newNotifier).

---

## 1. Complete worked project

An alerting service delivering to a DingTalk robot and a generic receiver, with governance
rate-limiting and observability. File tree (mirrors the smoke-verified [example/](example/)):

```
demo/
├── go.mod
├── main.go
├── alert.go
└── conf/
    └── app.properties
```

**go.mod** (module deps that matter):

```
require (
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-webhook    latest
    go-spring.org/starter-otel       latest   # optional: real span export
    go-spring.org/starter-governance latest   # optional: rate limit / breaker / fault
)
```

**main.go**:

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

**alert.go** — the application's entire notification surface:

```go
package alert

import (
    "context"

    "go-spring.org/spring/gs"

    StarterWebhook "go-spring.org/starter-webhook"
)

// Service injects named notifiers. Adding a channel is a pure config change
// plus another field — no starter code edit.
type Service struct {
    Alert  *StarterWebhook.Notifier `autowire:"alert"`  // dingtalk + 加签
    Report *StarterWebhook.Notifier `autowire:"report"` // generic receiver
}

func init() {
    gs.Provide(&Service{}).Export(gs.As[gs.Rooter]())
}

// Fire delivers one alert. Send builds the channel payload, signs it, wraps
// the POST in a producer span and routes it through the resilience executor
// (rate limit / circuit breaking / fault injection when governance is armed).
func (s *Service) Fire(ctx context.Context, title, text string) error {
    return s.Alert.Send(ctx, &StarterWebhook.Notifier.Notification{Title: title, Text: text})
}
```

**conf/app.properties** — the complete, commented surface (example config extended):

```properties
# --- notifier "alert": DingTalk group robot with 加签 secret -----------------
spring.webhook.alert.url=https://oapi.dingtalk.com/robot/send?access_token=xxx
spring.webhook.alert.channel=dingtalk
spring.webhook.alert.secret=SEC...
spring.webhook.alert.timeout=5s

# --- notifier "report": plain JSON POST to a self-built receiver -------------
spring.webhook.report.url=http://127.0.0.1:18080/hook
spring.webhook.report.channel=generic
# spring.webhook.report.observability.level=brief   # per-instance, ctor-bound

# --- observability (starter-otel) ---------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true

# --- governance (optional; resource label webhook:alert:dingtalk) ------------
govern.source.file.path=conf/govern.yaml
```

Other channels (from [example/conf/app.properties](example/conf/app.properties) comments):
`feishu` (`https://open.feishu.cn/open-apis/bot/v2/hook/xxx` + signing `secret`),
`wecom` (`https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx`, secret ignored),
`slack` (`https://hooks.slack.com/services/XXX/YYY/ZZZ`, secret ignored).

**Verify** (isomorphic to the example, which asserts the received payload):

```bash
cd example && go run .      # prints "Webhook delivered: map[...]" and self-exits
# against a real bot, simply check the group message; the receiver drill:
curl -s -X POST http://127.0.0.1:18080/hook ...   # or watch the span/metrics below
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-webhook
  └─ init(): gs.Group("${spring.webhook}", newNotifier, nil)  [thin, mail-style:
        stateless per call, no destroy hook]

gs.Run()
  ├─ config bind: spring.webhook.<name>.* → Config (value tags; url required via expr)
  ├─ newNotifier:
  │    ├─ dry-run buildPayload with an empty Notification — validates channel value and
  │    │   (for dingtalk/feishu) that signing works, WITHOUT any network call
  │    ├─ exec := fault.WrapExecutor(resilience.ExecutorFor("webhook:<name>:<channel>"))
  │    ├─ exec := resilobserve.WrapExecutor(exec, "webhook", c.Observability)
  │    └─ &http.Client{Timeout: c.Timeout} — per-notifier client, no pooling
  ├─ bean ready: *Notifier injected wherever `autowire:"<name>"` appears
  ├─ Run / serve: no background goroutines, no probe (rationale in §Activation)
  └─ SIGTERM: no destroy hook (each Send is a stateless HTTP request)
```

The resilience resource label is `webhook:<name>:<channel>` — per instance **and** channel,
so two notifiers on the same URL but different names get independent breaker/limiter state
(contrast starter-s3, whose label is endpoint-only).

### 2.2 One delivery, layer by layer

`Notifier.Send(ctx, notification)` (starter.go:92):

1. **Payload build** — `buildPayload(channel, n, secret, time.Now())` (payload.go:39):
   - `generic`: `{"title":..., "text":...}` JSON POST.
   - `dingtalk`: markdown message (`msgtype=markdown`, body = title + "\n\n" + text); when a
     secret is set, the 加签 pair — `timestamp` (ms) + HMAC-SHA256(ts+"\n"+secret), base64,
     URL-escaped — is returned as **extra query parameters**, merged onto the URL by
     `withQuery` (a fresh signature per send, since the timestamp is `time.Now()`).
   - `feishu`: text message; when a secret is set the same HMAC computation is folded into
     the **body** (`timestamp` + `sign` fields).
   - `wecom` / `slack`: markdown / text messages, secret ignored.
   - Unknown channel → error `webhook: unknown channel ... (want generic|dingtalk|feishu|wecom|slack)`.
2. **Span** — `startSend` opens the producer span `webhook.send` (attributes
   `messaging.system=webhook`, `webhook.channel`, `webhook.destination.host` —
   scheme://host only, keeping signed URLs out of telemetry).
3. **Executor** — the POST closure runs through the governance executor under the resource
   label: rate limit / circuit breaking / retry (if configured via governance) / fault
   injection when starter-governance is armed; transparent pass-through otherwise.
   `resilobserve.WrapExecutor` emits the outcome span + call counter + duration histogram +
   access log per level. A hand-built zero-value Notifier (tests) has no executor and posts
   directly — Send stays usable either way.
4. **POST** — `n.post`: `Content-Type: application/json`, client bounded by `timeout`;
   any non-2xx status is an error carrying the first 512 bytes of the body.
5. `EndSpan(span, err)` records the failure and closes the span.

Retry behavior: **no built-in retry**. Retries happen only if a retry policy for the
`webhook:<name>:<channel>` resource is armed through starter-governance; without governance
a failed POST returns the error to the caller immediately. ⚠ DingTalk/Feishu report some
failures as HTTP 200 with an error body — `post` checks only the status code, so those are
treated as success (see suspect §6).

### 2.3 What the startup dry-run catches (and what it doesn't)

`newNotifier` builds one payload for an empty Notification before wiring. That validates the
channel enum and the secret's usability in signing — without network. It cannot catch a wrong
URL, an expired access token, or a wrong secret (signature is computed, not verified by the
receiver). Those surface on first Send.

---

## 3. Per-key behavior reference

Prefix `spring.webhook.<name>.*` — all keys are ctor-bound into `Config` (config.go), i.e.
instance-prefixed, including the observability sub-struct (unlike starter-s3, there is no
wrapper-field tag here).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `url` | string | — | **Required** (`expr:"$ != ''"`). POST target; for dingtalk usually carries the `access_token` query. | Missing/empty → bind-time validation error. Malformed → first Send fails `webhook: invalid url`. |
| `channel` | string | generic | `generic`\|`dingtalk`\|`feishu`\|`wecom`\|`slack`; selects payload builder + signing scheme + the resilience label's third segment. | Unknown value → startup error from the dry-run buildPayload (exact message in §2.2). |
| `secret` | string | — | DingTalk 加签 (SEC...) or Feishu signing secret; **ignored** by generic/wecom/slack. | ⚠ Secret on the wrong channel is silently unused — unsigned messages get rejected by the receiver at Send time. |
| `timeout` | duration | 5s | Bounds one POST (`http.Client.Timeout`). | Too small → slow chat platforms produce client timeouts; the executor may retry (per governance policy) amplifying load. |
| `observability` | struct | brief | Per-send access log/metrics/span level (`off`\|`brief`\|`detailed`), sub-keys `observability.level` / `.maxArgBytes` (default 512) / `.skipOps` under the instance prefix; feeds `resilobserve.WrapExecutor`. | `off` removes the per-send log; `detailed` logs payload bytes up to maxArgBytes. |

---

## 4. Verification & fault drills

### 4.1 Delivery drill (self-contained, same path as example/check.sh)

```bash
cd example && go run .    # receiver captures the POST; example asserts
                           # title=="deploy" && text=="example finished"
```

For a real bot: `Send(ctx, &Notification{Title:"smoke", Text:"hello"})` and check the group.

### 4.2 Payload/signature drill (dingtalk)

Point the notifier at a local receiver and inspect the request:

```bash
# receiver log: query carries timestamp=...&sign=... (HMAC of ts+"\n"+secret)
# two sends one second apart produce two different signatures (fresh time.Now()).
```

Recompute offline to verify: `base64(HMAC_SHA256(secret, ts+"\n"+secret))` — matches
`dingtalkSign` (payload.go:77).

### 4.3 Failure drill (no restart)

Start with a bad URL (e.g. port 1): every Send returns `webhook: post failed` with the
connection error, wrapped in the producer span (status Error, error event) and counted in
the executor's call counter by outcome. There is nothing to "drill hot" in the starter
itself; with starter-governance, flipping the rule file arms a rate limit on
`webhook:alert:dingtalk` live — over-limit sends then fail fast with a limit-reject outcome
in the observability stream instead of reaching the platform.

### 4.4 Observability drill

With starter-otel imported, send once and read: span `webhook.send` (kind PRODUCER,
attributes as in §2.2 step 2), duration histogram, access log line (level from
`observability.level`; `detailed` includes body bytes up to `maxArgBytes`). Verify the
redaction property: the span attribute is `webhook.destination.host=scheme://host`, never
the signed URL.

### 4.5 Channel-matrix drill

The example's generic receiver can also stand in for slack/wecom/feishu payloads: flip
`channel` and Send; assert the received JSON shape against the builders in §2.2
(`msg_type`/`content` for feishu/wecom, `text` for slack). Unknown channel value must abort
at startup — that is the dry-run working.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Startup aborts "unknown channel" | typo in `channel` | One of generic/dingtalk/feishu/wecom/slack. |
| Startup aborts bind error on url | `spring.webhook.<name>.url` missing | Set it — required via expr tag. |
| First Send fails "invalid url" | malformed URL (e.g. spaces, bad scheme) | No startup network probe exists by design; fix the URL. |
| DingTalk rejects: sign mismatch | wrong/rotated secret, or clock skew >1h (timestamp in the signature) | Re-copy the 加签 secret; verify host clock. |
| DingTalk 200 but no message delivered | vendor error body (errcode) returned with HTTP 200 — not inspected by `post` | Check the platform bot logs; see suspect §6 (status-only success check). |
| `secret` set but messages unsigned | channel is generic/wecom/slack (secret ignored) | Only dingtalk/feishu sign. |
| Sends suddenly fail with limit-reject | governance rate limit on `webhook:<name>:<channel>` armed | Raise the limit or withdraw the rule (hot-reload). |
| Feishu rejects: timestamp invalid | signing timestamp too old (host clock) or secret mismatch | Same fix as the DingTalk sign row. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 5 (+3 observability.* sub-keys, instance-prefixed) |
| Required | 1 (`url`) |
| Quickstart external deps | 0 (example runs its own receiver) |
| "Watch out" entries | 4 |

Design suspects (for the audit ledger; first one carried over from the previous edition):
- Channel payload builders live in the starter with no extension point for a new channel —
  adding one requires editing the starter; consider a channel registry.
- NEW: `post` treats any 2xx as success, but DingTalk/Feishu return vendor error bodies
  (`errcode`) with HTTP 200 — the doc comment in starter.go:116-117 claims these are caught,
  the implementation is not. Either parse the body for known channels or fix the comment.
- NEW: no built-in retry despite webhooks being a classic at-least-once case — retry exists
  only via governance policy, which is easy to leave unconfigured.
- NEW: `plainText` (payload.go:129) is unused by the send path — dead helper, candidate for
  deletion.

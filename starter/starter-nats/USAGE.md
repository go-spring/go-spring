# starter-nats Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`,
`driver.go`, `messaging.go`) and the runnable [example/](example/) / [example-otel/](example-otel/).
**NATS semantics (core NATS, JetStream, queue groups, subject wildcards, drain) are
[nats.go's own documentation](https://docs.nats.io/)** — everything below is go-spring's increment.

**Activation**: every `spring.nats.instances.<name>` entry registers one `*Conn` bean named `<name>`
[starter.go:35-54]. No entries → starter inactive. Multi-instance only; no default singleton.

---

## 1. Complete worked project

Producer AND consumer via the messaging.Driver, with actuator + otel + governance composed.
File tree:

```
demo/
├── go.mod
├── main.go
├── messaging.go
└── conf/
    ├── app.properties
    └── governance.yaml
```

**go.mod** (deps that matter):

```
require (
    github.com/nats-io/nats.go    v1.38.0
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-nats    latest
    go-spring.org/starter-actuator latest   // optional: probes + /metrics mount
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
    go-spring.org/starter-governance-file latest // optional: runtime resilience/fault
)
```

**main.go**:

```go
package main

import (
    "demo/messaging"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-nats"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**messaging.go** — driver-based producer and consumer on one connection:

```go
package messaging

import (
    "context"
    "time"

    "go-spring.org/cloud/messaging"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
)

func init() {
    // The starter already registers one messaging.Driver bean per NATS
    // connection, under the connection's name — inject it by name, no manual
    // registration needed. (Building one by hand with StarterNats.NewDriver
    // would collide with the auto-registered bean.)
    gs.Provide(newConsumer).Export(gs.As[gs.Rooter]())
}

type Consumer struct {
    Sub messaging.Subscriber `autowire:"?"`
}

func newConsumer(drv messaging.Driver) *Consumer {
    sub, err := drv.NewSubscriber(context.Background(), "orders.created", "workers")
    if err != nil {
        panic(err)
    }
    return &Consumer{Sub: sub}
}

func (c *Consumer) Init(ctx context.Context) error {
    return c.Sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
        log.Infof(ctx, log.TagAppDef, "order received: %s trace-id-from-header=%v",
            string(m.Payload), m.Headers["traceparent"])
        return nil // error return → Recover error path
    })
}
```

Publish from anywhere (HTTP handler, cron, outbox drainer):

```go
pub, _ := driver.NewPublisher(ctx, "orders.created")
defer pub.Close()
err := pub.Publish(ctx, &messaging.Message{
    Payload: []byte(`{"id":42}`),
    Headers: map[string]string{"tenant": "acme"},
})
```

**conf/app.properties** — the complete commented surface:

```properties
# --- nats (multi-instance: each spring.nats.instances.<name> = one *Conn bean) ----------
spring.nats.instances.main.url=nats://127.0.0.1:4222
spring.nats.instances.main.name=orders-service        # feeds governance label + server-side conn name
spring.nats.instances.main.jetstream.enabled=true     # exposes Conn.JetStream (same connection)
# spring.nats.instances.main.max-reconnects=-1        # -1 = unlimited (default 60)
# spring.nats.instances.main.reconnect-wait=2s
# spring.nats.instances.main.connect-timeout=5s
# auth (mutually orthogonal styles, pick one):
# spring.nats.instances.main.username=... / password=...
# spring.nats.instances.main.token=...
# spring.nats.instances.main.creds-file=/etc/nats/app.creds
# spring.nats.instances.main.nkey-file=/etc/nats/app.nk
# TLS: spring.nats.instances.main.tls.enabled=true + ca-file/cert-file/key-file/...

# --- actuator (probes + metrics mount) ----------------------------------------
spring.actuator.addr=:9370

# --- observability (starter-otel) ---------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics via actuator only

# --- governance (runtime resilience) -------------------------------------------
spring.governance.source.file.path=conf/governance.yaml
```

**conf/governance.yaml** (drills in §4 use this):

```yaml
spring:
  governance:
    enabled: true
    resilience:
      nats:                 # matches the ResourceLabel scheme, see §4.3
        rate-limit: 100
        breaker:
          error-threshold: 5
```

**Broker + verify**:

```bash
docker run -d --name nats -p 127.0.0.1:4222:4222 nats:2.10 -js
go run .                                   # driver subscribe, then publish
curl -i :9370/healthz                      # actuator liveness
curl -s :9370/metrics | grep -i nats
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-nats
  └─ gs.Module(gs.OnProperty("spring.nats"))              [starter.go:35]
gs.Run()
  ├─ config bind: each spring.nats.instances.<name> → Config (value tags; url expr-validated ≠ "")
  ├─ per name: Provide(newConn, IndexArg(1,ValueArg(name)), IndexArg(2,ValueArg(c)))
  │             .Name(name).Destroy((*Conn).Destroy).Caller(1)  [starter.go:45-54]
  ├─ newConn: Driver bean, selected per entry by ${spring.nats.instances.<name>.driver}
  │     (empty = `spring.nats.default.driver` then by type; set = by bean name — naming a missing bean fails startup);
  │     falls back to bundled DefaultDriver when none is
  │           present) → CreateClient(ctx, Config, cloud.ClientParams{Resilience: mgr, Fault: inj})
  │           (nats.Connect — FAIL-FAST probe; a broker that is down aborts boot)
  ├─ CreateClient: NewConn(nc, url, params) — fixes identity AND applies governance in one step,
  │           so the Conn is complete when the Driver returns it (the `nats:<url>` service label;
  │           params.ExecutorFor → fault.WrapClientExecutor(mgr.ClientExecutorFor("nats", service), service, inj),
  │           or resilience.Unmanaged when the bundle is zero)
  │           [driver.go:145; client.go:86-93]
  ├─ jetstream.enabled → jetstream.New(nc); failure closes nc and fails boot
  │           [driver.go:149-157]
  ├─ newConn: bundles mgr/inj (= the injected *resilience.Manager / *fault.Injector beans)
  │           into the cloud.ClientParams it hands the Driver; nothing patches the Conn afterwards
  │           [driver.go:181-182]
  ├─ startup connectivity check on the bare client (IsConnected); failure destroys the bean
  │           [driver.go:196-200]
  ├─ Run / readiness
  └─ SIGTERM: (*Conn).Destroy → exec.Close() (error returned after Drain) then conn.Drain()
              — in-flight subscriptions finish, then the socket closes [client.go:105-113]
```

Fail-fast: `nats.Connect` performs the initial dial synchronously; wrong URL/auth/broker-down
exits boot with `failed to connect nats: <url>` [driver.go:132-136]. After boot, disconnects
are logged (`nats disconnected` Warn) and the client auto-reconnects [driver.go:88-97] —
`HealthCheck(ctx, conn)` reflects the live `IsConnected()` state [health.go].

### 2.2 One publish, layer by layer

`pub.Publish(ctx, msg)` [messaging.go:81-108]:

1. Envelope → `nats.Msg{Subject, Data, Header}`; `messaging.Message.Key` rides the
   reserved `x-msg-key` header (restored into `Key` on consume; never leaks into
   `Headers`) [messaging.go:82-89]. Empty header map → nil header.
2. Load-test marker: `prop.Inject(ctx, …)` stamps the marker (canonical
   `X-LoadTest: 1`) so consumers recognise synthetic load; it is a no-op outside
   load-test traffic, and the header map is allocated on demand.
3. The driver DECLARES the publish's identity and routes it through the Conn's publish
   seam [messaging.go:100-107]: `PublishMsgContext` declares `operation(opPublish, subject)`
   on the ctx, injects the W3C `traceparent` from the executor's attempt ctx into
   `nm.Header` (so the consumer continues the trace), and the connection's resilience
   executor — the single emitter — opens the producer span (parented on the caller's
   active span via the Publish ctx), records the durations and writes the access log.
4. Raw `c.conn.PublishMsg` (async buffered write — returns before broker ack; use Flush
   for confirmation; see [nats.go docs](https://docs.nats.io/)). The executor wraps this
   same wire call, so the message is counted exactly once.

The driver path is guarded exactly like the raw-client path (§2.4): it rides
`PublishMsgContext`, so no publish is left un-declared or unprotected.

### 2.3 One consume, layer by layer

`sub.Subscribe(handler)` [messaging.go:123-143]: handler wrapped in `messaging.Recover`
(panic → error path, not SDK-goroutine crash) [messaging.go:127]; the driver routes the
subscription through `Conn.Consume` (empty group = plain subscription, non-empty = queue
group / competing consumers) [messaging.go:134-137]. Per message:

1. The driver DECLARES the consume's identity and routes it through the Conn's consume
   seam: `Consume` extracts the W3C `traceparent` from the envelope headers (mapped from
   `nm.Header`) into a fresh ctx, declares `operation(opConsume, subject)` on it, and the
   connection's resilience executor — the single emitter — opens the consumer span (child
   of the producer span), records the durations and writes the access log before calling
   the handler [command.go:158-165].
2. `X-LoadTest` header re-materialised into ctx as the load-test marker
   [messaging.go:134-137].
3. `fromNatsMsg`: multi-valued NATS headers flattened to single values (`Get` = first
   value wins) [messaging.go:169-180].
4. Handler runs; the executor records the outcome, including a handler error, so the
   consumer span is marked failed. Close = `Subscription.Unsubscribe`.

The driver path and any direct caller of `Conn.Consume(ctx, subject, queue, handler)`
share this exact seam — the driver adds only envelope conversion, so app code that
consumes raw `*nats.Msg` is instrumented identically and never double-counted.

Not instrumented: **JetStream consumes**, which have their own API surface. Use
`PublishMsgContext`/`Consume` for traced pub/sub.

### 2.4 The guard mechanism — what is and is NOT protected

NATS exposes no reject-capable middleware (unlike redis Hook / http RoundTripper), so the
resilience executor is driven at the call site rather than threaded in as an interceptor.
Every publish and consume DECLARES its operation (see observe.go) and runs under the executor
via `guard` [command.go:197-202] — and the executor is the **single emitter**: it reads the
declared operation off the ctx and opens the call span, records the call-level
`messaging.client.operation.duration` and attempt-level `messaging.client.attempt.duration`
histograms, the in-flight gauge, the `resilience.client.calls` counter, and the one access log.
The publish and consume operations are declared `NonIdempotent`, so a governance rule for the
label that carries a retry policy has its retry **suppressed** (with one warning per service):
re-publishing or re-running a handler is a second side effect, not a second attempt.

| Entry point | Emitted by | Resilience guard |
|---|---|---|
| `Conn.PublishMsg` / `Conn.PublishMsgContext` (raw-client path) | starter declares + executor emits | yes |
| `Conn.Consume(ctx, subject, queue, handler)` (raw-client path) | starter declares + executor emits | yes |
| messaging.Driver publish/subscribe (rides `PublishMsgContext` / `Consume`) | starter declares + executor emits | yes |
| `Conn.Publish` / `Conn.Request` / `Subscribe` / `QueueSubscribe` / JetStream | **no** | **no** |
| `Conn.PublishGuarded(ctx, subj, data)` | starter declares + executor emits (routes through `PublishMsgContext`) | yes |
| `Conn.RequestGuarded(ctx, subj, data, timeout)` | executor's fallback signals only | yes |

`RequestGuarded` declares no operation, so the executor emits this layer's fallback signals
(`resilience.client.duration`, a span named after the service) rather than a `messaging.client.*`
one — no operation span is named for it.

Wrap order inside `Governance.ExecutorFor` (see `cloud/governance/governance.go`, called by
[NewConn](client.go:86-93)): `mgr.ClientExecutorFor("nats", service)`
(armed from the injected `*resilience.Manager` bean; a `resilience.Unmanaged` observe-only
executor when the bundle is zero) → `fault.WrapClientExecutor` (fault injection, outermost). The manager's resolve step
wraps the backing executor with the observe emitter (see `resilience/observe.go`), so the declared
operation's span/counter/histogram/access-log are produced once, from the one point on the chain
that sees a whole call. On rejection the guarded call returns a resilience sentinel
(`ErrRateLimited` / `ErrCircuitOpen`) and the underlying publish/request is never invoked — proven
by [resilience_test.go:53-84]. The `service` is `nats:<url>` (per connection, not per subject)
[client.go:86-93], so limiter/breaker state is shared across all subjects on one connection.

`PublishGuarded` takes the caller's ctx and threads it into the executor and the call span
(via `PublishMsgContext`). `RequestGuarded` takes the caller's ctx but the timeout is per-attempt
inside `Request`, and it declares no operation, so it opens no operation span of its own.

### 2.5 Health

Each instance contributes a `health.Indicator` named `nats:<name>` unless its entry sets
`health.enabled=false` [starter.go]. The probe reads the bare client's `IsConnected()` directly
(no span, no limiter/breaker budget), so an instance between
reconnects reports down and recovers on its own. That is the right default for a dependency the app
cannot serve without; set `health.enabled=false` for an optional side connection that should not gate
readiness — the indicator is then not registered at all, rather than registered and always green.

---

## 3. Per-key behavior reference

All keys live under `spring.nats.instances.<name>.*`. The `tls` group key binds a nested shared
struct — its sub-keys belong to security, not this starter.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `url` | string | — | **Required** (`expr:"$ != ''"` [config.go:31]); comma-separated server list handed to `nats.Connect`. | Missing/empty → bind error at boot. |
| `name` | string | "" | Connection name reported to the server (not part of the governance label, which is `nats:<url>`) [driver.go:77]. | Empty name still works. |
| `username` / `password` | string | "" | Both set → `nats.UserInfo`. Orthogonal to other auth styles [driver.go:103-105]. | Username without password → empty password sent. |
| `token` | string | "" | → `nats.Token` [driver.go:106-108]. | Combined with username → last-applied nats option wins (NATS-defined). |
| `creds-file` | string | "" | JWT+nkey seed file → `nats.UserCredentials` [driver.go:109-111]. | Bad path → connect fails at boot. |
| `nkey-file` | string | "" | nkey seed → `nats.NkeyOptionFromSeed`; load failure is explained and fails boot [driver.go:112-118]. | Bad seed file → boot error. |
| `tls` | group | off | `tls.enabled=true` → `security.BuildClient()` → `nats.Secure`; BuildClient()==nil → bare `nats.Secure()` [driver.go:119-131]. ⚠ `BuildClient()` not `BuildServer()` — same client-TLS posture as other client starters. Sub-keys: `enabled`/`ca-file`/`cert-file`/`key-file`/`insecure-skip-verify`/`server-name` (security's tags). | TLS mismatch → connect error at boot. |
| `max-reconnects` | int | 60 | → `nats.MaxReconnects`; -1 = unlimited [driver.go:78]. | -1 with dead broker → reconnect loop forever (by design). |
| `reconnect-wait` | duration | 2s | Delay between reconnect attempts [driver.go:79]. | Too low → busy reconnect against a down cluster. |
| `connect-timeout` | duration | 5s | Bounds the **initial dial only** [driver.go:80]. | Too low → spurious boot failures on slow networks. |
| `jetstream` | group | — | Container for `enabled`. | — |
| `jetstream.enabled` | bool | false | Derives `jetstream.New(nc)` on the SAME connection; failure closes nc and fails boot; otherwise `Conn.JetStream` stays nil [driver.go:143-151]. | Enabled against a broker without `-js` → boot error. |

Grep reconciliation: the 13 distinct `value:` tags in this starter's Go files are exactly
`url`, `name`, `username`, `password`, `token`, `creds-file`, `nkey-file`, `tls`,
`max-reconnects`, `reconnect-wait`, `connect-timeout`, `jetstream`, `jetstream.enabled`
(as `${enabled:=false}`) — all tabled above; no extras either way.

---

## 4. Verification & fault drills

### 4.1 Broker-down fail-fast

```bash
docker stop nats
go run .          # boot aborts: "failed to connect nats: nats://127.0.0.1:4222"
```

And after boot: `docker stop nats` → Warn `nats disconnected`, app stays up, `HealthCheck(ctx, conn)`
returns an error; restart the broker → Info `nats reconnected to ...` [driver.go:88-97].

### 4.2 Guarded vs unguarded path

With governance on and a tight rate limit (§1 governance.yaml):

- `conn.Publish("s", b)` — always succeeds (no guard) [delegate.go:42].
- `conn.PublishGuarded(ctx, "s", b)` in a hot loop → rejects with `resilience.ErrRateLimited`
  once the burst is spent, and the rejected call never reaches the socket
  [resilience_test.go:53-65].
- Breaker drill: make publishes fail (stop broker after boot) until `error-threshold`
  trips → subsequent guarded calls return `ErrCircuitOpen` without invoking the publish
  [resilience_test.go:69-84].

### 4.3 Governance label check

The executor service is `nats:<url>` [client.go:89]. Scope your governance.yaml
rules to `nats:orders-service` (or a prefix) so the policy lands on exactly
this connection. Verify: wrapped-executor rejections emit a span + counter named by the
resilience-observe bridge (`system="nats"`) [command.go:179-181] — grep traces/metrics for
`nats` after a drill.

### 4.4 Message round-trip incl. driver mapping survival

Publish an envelope with `Payload` + `Headers{"tenant":"acme"}`; in the consumer assert:

- `m.Payload` survives byte-for-byte.
- `m.Headers["tenant"]` survives (string→nats.Header→string).
- `m.Headers["traceparent"]` present when tracing is live (injected by §2.2 step 4).
- `m.Key` round-trips via the reserved `x-msg-key` header (not visible in `m.Headers`).
- Multi-value headers: only the first value survives (single-valued envelope).

### 4.5 Metrics / span / log reads

- Access log: one structured record per declared publish/consume, written by the resilience
  emitter under the `nats`/`access` tag (level `brief` default; `detailed` adds payload bytes
  up to `maxArgBytes`). Failure → Warn; a success with a subject → Debug; a success without → Info.
- Metrics: `messaging.client.operation.duration` (call-level) and
  `messaging.client.attempt.duration` (per attempt) histograms plus the
  `messaging.client.active_requests` gauge, labelled `messaging.system="nats"`,
  `messaging.operation`, `status` — the subject stays a span/log detail and never a label;
  `resilience.client.calls` counts every call. Read via `curl -s :9370/metrics | grep -i nats`.
- Spans: one span per publish/consume, named `publish` / `consume`, carrying
  `messaging.system` / `messaging.operation` / `messaging.destination.name`, linked via the
  W3C header; [example-otel/](example-otel/) ships the full Jaeger check
  (`http://127.0.0.1:16686/api/traces?service=...`).
- Connection events: the `messaging.client.connection.state_changes` counter (kept in the
  starter, driven by the NATS client's own callbacks) plus async error / disconnect /
  reconnect / close lines under log tag `app_def` [driver.go:81-102].
- Shutdown: Drain lets in-flight subscriptions finish; `exec.Close()`'s error is
  returned (after Drain) to the destroy hook [client.go:105-113].

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot aborts `failed to connect nats` | broker down / wrong `url` / auth rejected (fail-fast first dial) [driver.go:132-136] | Start broker (`docker run ... nats:2.10 -js`), fix `url`/auth. |
| Boot aborts `failed to create jetstream context` | `jetstream.enabled=true` but broker started without `-js` [driver.go:143-151] | Start server with `-js` or disable the key. |
| `Conn.JetStream` is nil | `jetstream.enabled` unset | Set it; JS is derived lazily-but-at-boot from the same conn. |
| Consumer gets messages but no traces/metrics on consumes | using the raw `Conn.Subscribe`/`QueueSubscribe` delegations instead of `Conn.Consume` | Consume via `Conn.Consume(ctx, subject, queue, handler)` or messaging.Driver. |
| Guarded calls suddenly fail with sentinel errors | rate limit exhausted or breaker open — by design [command.go:187-192] | Check governance.yaml policy; breaker recovers after cool-down. |
| Producer trace never links to the caller's span | the publish used `PublishMsg` (no ctx parameter), so its span is a new root | Use `PublishMsgContext(ctx, msg)` — `PublishGuarded` already does. |
| `/readiness` reports `nats:<name>` down | the auto-reconnecting client is between reconnects | Expected; if this instance's connectivity should not gate readiness, set `health.enabled=false` on it. |
| Missing 2nd header value | driver flattens multi-value headers to the first value (single-valued envelope) | Carry the extra values in the payload or use the raw Conn API. |
| Reconnect storm in logs | `reconnect-wait` too low with dead cluster | Raise it; reconnect is the client's reliability mechanism. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 15 starter-local value tags (+ tls group sub-keys in security) |
| Required | 1 (`url`) |
| Quickstart external deps | 1 (nats; collector optional for observability) |
| "Watch out" entries | 4 |

Design suspects (audit ledger):

- **Fixed**: publish spans can be parented on the caller's trace via
  `PublishMsgContext` (the ctx-less `PublishMsg` keeps its documented new-root behaviour
  for compatibility — nats.PublishMsg has no ctx parameter); consumes are instrumented
  for any caller of `Conn.Consume`, not just the driver; a `health.Indicator` is wired
  per instance (`health.enabled`, see §2.5); `PublishGuarded` now threads the caller ctx
  into its call span; every publish/consume declares its operation and the resilience
  executor is the single emitter (no per-call instrumentation in the starter).
- **Open**: JetStream consumes untraced (separate API surface); multi-value headers
  flattened to first value; `RequestGuarded` declares no operation, so it emits only the
  executor's fallback signals.
- **Fixed**: none yet from the previous ledger — all prior suspects remain open.

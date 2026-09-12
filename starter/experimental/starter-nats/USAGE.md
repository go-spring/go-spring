# starter-nats Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`driver.go`) and the runnable [example/](example/) / [example-otel/](example-otel/).
**NATS semantics (core NATS, JetStream, queue groups, subject wildcards, drain) are
[nats.go's own documentation](https://docs.nats.io/)** — everything below is go-spring's increment.

**Activation**: every `spring.nats.instances.<name>` entry registers one `*Conn` bean named `<name>`
[starter.go:33-40]. No entries → starter inactive. Multi-instance only; no default singleton.

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
    └── govern.yaml
```

**go.mod** (deps that matter):

```
require (
    github.com/nats-io/nats.go    v1.38.0
    go-spring.org/spring          v1.3.x
    go-spring.org/starter-nats    latest
    go-spring.org/starter-actuator latest   // optional: probes + /metrics mount
    go-spring.org/starter-otel     latest   // optional: real trace/metric export
    go-spring.org/starter-governance latest // optional: runtime resilience/fault
)
```

**main.go**:

```go
package main

import (
    "demo/messaging"

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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

    StarterNats "go-spring.org/starter-nats"
)

func init() {
    // One messaging.Driver bean per NATS connection you want to bind.
    // TagArg("main") resolves the *Conn bean named "main" (autowire name
    // from config: the tag is the bean name).
    gs.Provide(func(conn *StarterNats.Conn) messaging.Driver {
        return StarterNats.NewDriver(conn)
    }, gs.TagArg("main")).Export(gs.As[gs.Rooter]())

    gs.Provide(newConsumer).Export(gs.As[gs.Rooter]())
}

type Consumer struct {
    Sub messaging.Subscriber `autowire:"?"`
}

func newConsumer(b messaging.Driver) *Consumer {
    sub, err := b.NewSubscriber(context.Background(), "orders.created", "workers")
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
govern.source.file.path=conf/govern.yaml
```

**conf/govern.yaml** (drills in §4 use this):

```yaml
govern:
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
  └─ gs.Module(gs.OnProperty("spring.nats"))              [starter.go:33]
gs.Run()
  ├─ config bind: each spring.nats.instances.<name> → Config (value tags; url expr-validated ≠ "")
  ├─ per name: Provide(newConn, IndexArg(1,ValueArg(name)), IndexArg(2,ValueArg(c)))
  │             .Name(name).Destroy(destroyConn).Caller(1)  [starter.go:36-40]
  ├─ newConn: Driver bean, selected per entry by ${spring.nats.instances.<name>.driver}
  │     (empty = `spring.nats.default.driver` then by type; set = by bean name — naming a missing bean fails startup);
  │     falls back to bundled DefaultDriver when none is
  │           present) → CreateClient (nats.Connect — FAIL-FAST probe; a broker
  │           that is down aborts boot)
  ├─ attach instrumentation: pubObs/subObs (module-local observe.go)
  │           [driver.go:155-156]
  ├─ jetstream.enabled → jetstream.New(nc); failure closes nc and fails boot
  │           [driver.go:158-164]
  ├─ applyResilience: fault.WrapExecutor(resilience.ExecutorFor("nats", resource))
  │           — no-op executor when governance is off
  │           [driver.go:166; command.go:167-172]
  ├─ Run / readiness
  └─ SIGTERM: destroyConn → exec.Close() (error returned after Drain) then Conn.Drain()
              — in-flight subscriptions finish, then the socket closes [client.go:70-74]
```

Fail-fast: `nats.Connect` performs the initial dial synchronously; wrong URL/auth/broker-down
exits boot with `failed to connect nats: <url>` [driver.go:122-126]. After boot, disconnects
are logged (`nats disconnected` Warn) and the client auto-reconnects [driver.go:57-59] —
`Healthy()` reflects the live `IsConnected()` state [client.go:62-64].

### 2.2 One publish, layer by layer (driver path)

`pub.Publish(ctx, msg)` [driver.go:58-71]:

1. Envelope → `nats.Msg{Subject, Data, Header}`; `messaging.Message.Key` rides the
   reserved `x-msg-key` header (restored into `Key` on consume; never leaks into
   `Headers`) [driver.go:59-73]. Empty header map → nil header.
2. Load-test marker: if `traffic.IsLoadTest(ctx)`, `X-LoadTest: 1` (canonical header name)
   is stamped so consumers recognise synthetic load [driver.go:62-67].
3. `Conn.PublishMsg` override [command.go:82-91]: `pubObs.Start(context.Background(),
   "publish", subject)` opens the producer span + duration/in-flight metric + access log.
   ⚠ span parent is `context.Background()` — the publish span is always a NEW ROOT, the
   caller's active trace is NOT continued (nats.PublishMsg carries no ctx; unfixable
   without an API change). Consume-side continuation is unaffected: the consumer span
   continues the publish span via the injected `traceparent` header (step 4). If you
   need the publish linked into the caller's trace, use the ctx-aware manual path:
   `ctx, sp := StartPublishSpan(ctx, msg)` + the embedded `conn.Conn.PublishMsg(msg)`
   (span-only; no metric/access log).
4. `injectW3C` puts `traceparent` into `msg.Header` so the consumer continues this span
   [command.go:51-56, 87].
5. Raw `c.Conn.PublishMsg` (async buffered write — returns before broker ack; use Flush
   for confirmation; see [nats.go docs](https://docs.nats.io/)).
6. `sp.End(err)` records outcome.

What this path does NOT get: resilience (guarded methods are separate, §2.4).

### 2.3 One consume, layer by layer (driver path)

`sub.Subscribe(handler)` [driver.go:79-116]: handler wrapped in `messaging.Recover`
(panic → error path, not SDK-goroutine crash) [driver.go:86-88]; `Subscribe` vs
`QueueSubscribe` on non-empty group (competing consumers) [driver.go:105-110]. Per message:

1. `startConsume` extracts W3C `traceparent` from `nm.Header`, opens the consumer span
   (child of the producer span) + metric + log [command.go:96-102; driver.go:91-93].
2. `X-LoadTest` header re-materialised into ctx as the load-test marker [driver.go:94-96].
3. `fromNatsMsg`: multi-valued NATS headers flattened to single values (`Get` = first
   value wins) [driver.go:138-149].
4. Handler runs; `sp.End(err)` [driver.go:97-99]. Close = `Subscription.Unsubscribe`
   [driver.go:119-124].

Documented gap: **direct `Conn.Subscribe` / JetStream consumes are NOT instrumented** —
only the driver callback opens consumer spans [command.go:28-32]. Manual escape hatch:
`StartPublishSpan` / `StartConsumeSpan` / `EndSpan` emit span-only (no metric/log)
[command.go:109-160], as used by example/.

### 2.4 The guard mechanism — what is and is NOT protected

NATS exposes no reject-capable middleware (unlike redis Hook / http RoundTripper), so the
resilience executor is reached only through opt-in **methods** [command.go:152-160]:

| Entry point | Observe | Resilience guard |
|---|---|---|
| `Conn.PublishMsg` (incl. driver publish) | span+metric+log | **no** |
| `Conn.Publish` / `Conn.Request` / `Subscribe` / `QueueSubscribe` / JetStream | **no** | **no** |
| `Conn.PublishGuarded(ctx, subj, data)` | span+metric+log (routes through `PublishMsg`) | yes |
| `Conn.RequestGuarded(ctx, subj, data, timeout)` | **no** | yes |

Wrap order inside `applyResilience` [command.go:167-172]: `ExecutorFor("nats", resource)` (governance
center-backed; transparent no-op when governance off; already carries the observe layer —
spans/counters/histograms for breaker trips, rejects, retries — the resilience core emits none) →
`fault.WrapExecutor` (fault injection, outermost). On rejection the guarded call returns
a resilience sentinel (`ErrRateLimited` / `ErrCircuitOpen`) and the underlying publish/request
is never invoked — proven by [resilience_test.go:63-84]. The `resource` is
`nats:<name>` (colon format; falls back to `nats:<url>` when name unset) (per connection, not per subject) [driver.go:166], so limiter/breaker
state is shared across all subjects on one connection.

`PublishGuarded` takes the caller's ctx (threaded into the executor; the producer span itself still
starts from `context.Background()` per the PublishMsg limitation above). `RequestGuarded` takes the
caller's ctx but the timeout is per-attempt inside `Request`.

---

## 3. Per-key behavior reference

All keys live under `spring.nats.instances.<name>.*`. The `tls` group key binds a nested shared
struct — its sub-keys belong to tlsconf, not this starter.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `url` | string | — | **Required** (`expr:"$ != ''"` [config.go:31]); comma-separated server list handed to `nats.Connect`. | Missing/empty → bind error at boot. |
| `name` | string | "" | Connection name reported to the server + **governance resource label component** [driver.go:166]. | Empty name still works; label uses "". |
| `username` / `password` | string | "" | Both set → `nats.UserInfo`. Orthogonal to other auth styles [driver.go:60-62]. | Username without password → empty password sent. |
| `token` | string | "" | → `nats.Token` [driver.go:63-65]. | Combined with username → last-applied nats option wins (NATS-defined). |
| `creds-file` | string | "" | JWT+nkey seed file → `nats.UserCredentials` [driver.go:66-68]. | Bad path → connect fails at boot. |
| `nkey-file` | string | "" | nkey seed → `nats.NkeyOptionFromSeed`; load failure is explained and fails boot [driver.go:69-76]. | Bad seed file → boot error. |
| `tls` | group | off | `tls.enabled=true` → `tlsconf.BuildClient()` → `nats.Secure`; BuildClient()==nil → bare `nats.Secure()` [driver.go:77-88]. ⚠ `Build()` not `BuildServer()` — same client-TLS posture as other client starters. Sub-keys: `enabled`/`ca-file`/`cert-file`/`key-file`/`insecure-skip-verify`/`server-name` (tlsconf's tags). | TLS mismatch → connect error at boot. |
| `max-reconnects` | int | 60 | → `nats.MaxReconnects`; -1 = unlimited [config.go:63; driver.go:49]. | -1 with dead broker → reconnect loop forever (by design). |
| `reconnect-wait` | duration | 2s | Delay between reconnect attempts [driver.go:50]. | Too low → busy reconnect against a down cluster. |
| `connect-timeout` | duration | 5s | Bounds the **initial dial only** [driver.go:51]. | Too low → spurious boot failures on slow networks. |
| `jetstream` | group | — | Container for `enabled`. | — |
| `jetstream.enabled` | bool | false | Derives `jetstream.New(nc)` on the SAME connection; failure closes nc and fails boot; otherwise `Conn.JetStream` stays nil [driver.go:157-164]. | Enabled against a broker without `-js` → boot error. |

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

And after boot: `docker stop nats` → Warn `nats disconnected`, app stays up, `Healthy()`
returns false; restart the broker → Info `nats reconnected to ...` [driver.go:57-65].

### 4.2 Guarded vs unguarded path

With governance on and a tight rate limit (§1 govern.yaml):

- `conn.Publish("s", b)` — always succeeds (no guard) [client.go:33-39].
- `conn.PublishGuarded(ctx, "s", b)` in a hot loop → rejects with `resilience.ErrRateLimited`
  once the burst is spent, and the rejected call never reaches the socket
  [resilience_test.go:63-77].
- Breaker drill: make publishes fail (stop broker after boot) until `error-threshold`
  trips → subsequent guarded calls return `ErrCircuitOpen` without invoking the publish
  [resilience_test.go:80-102].

### 4.3 Governance label check

The executor resource is `nats:<name>` (colon format; falls back to `nats:<url>` when name unset) [driver.go:166]. Scope your govern.yaml
rules to `nats:orders-service` (or a prefix) so the policy lands on exactly
this connection. Verify: wrapped-executor rejections emit a span + counter named by the
resilience-observe bridge (`system="nats"`) [command.go:170-172] — grep traces/metrics for
`nats` after a drill.

### 4.4 Message round-trip incl. driver mapping survival

Publish an envelope with `Payload` + `Headers{"tenant":"acme"}`; in the consumer assert:

- `m.Payload` survives byte-for-byte.
- `m.Headers["tenant"]` survives (string→nats.Header→string).
- `m.Headers["traceparent"]` present when tracing is live (injected by §2.2 step 4).
- `m.Key` round-trips via the reserved `x-msg-key` header (not visible in `m.Headers`).
- Multi-value headers: only the first value survives (single-valued envelope).

### 4.5 Metrics / span / log reads

- Access log: one structured record per instrumented publish/consume (level `brief` default;
  `detailed` adds payload bytes up to `maxArgBytes`).
- Metrics: messaging duration histogram + in-flight gauge with `system="nats"`,
  `destination` = subject — read via `curl -s :9370/metrics | grep -i nats`.
- Spans: producer `publish <subject>` (SpanKind producer), consumer `consume <subject>`
  linked via W3C header; [example-otel/](example-otel/) ships the full Jaeger check
  (`http://127.0.0.1:16686/api/traces?service=...`).
- Connection events: async error / disconnect / reconnect / close lines under log tag
  `app_def` [driver.go:52-65].
- Shutdown: Drain lets in-flight subscriptions finish; `exec.Close()`'s error is
  returned (after Drain) to the destroy hook [client.go:71-73].

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot aborts `failed to connect nats` | broker down / wrong `url` / auth rejected (fail-fast first dial) [driver.go:122-126] | Start broker (`docker run ... nats:2.10 -js`), fix `url`/auth. |
| Boot aborts `failed to create jetstream context` | `jetstream.enabled=true` but broker started without `-js` [driver.go:158-164] | Start server with `-js` or disable the key. |
| `Conn.JetStream` is nil | `jetstream.enabled` unset | Set it; JS is derived lazily-but-at-boot from the same conn. |
| Consumer gets messages but no traces/metrics on consumes | using raw `Conn.Subscribe` instead of the driver (documented gap [command.go:28-32]) | Consume via messaging.Driver, or hand-roll StartConsumeSpan. |
| Guarded calls suddenly fail with sentinel errors | rate limit exhausted or breaker open — by design [command.go:177-186] | Check govern.yaml policy; breaker recovers after cool-down. |
| Producer trace never links to the caller's span | PublishMsg span parent is `context.Background()` [command.go:79-91] — by design: nats.PublishMsg has no ctx parameter, so publish spans are new roots | Not fixable without an API change. Consume-side continuation still works (traceparent header). For caller-linked publishes use the ctx-aware manual path `StartPublishSpan(ctx, msg)` + embedded `PublishMsg`. |
| Missing 2nd header value | driver flattens multi-value headers to the first value (single-valued envelope) | Carry the extra values in the payload or use the raw Conn API. |
| Reconnect storm in logs | `reconnect-wait` too low with dead cluster | Raise it; reconnect is the client's reliability mechanism. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 14 starter-local value tags (+ tls group sub-keys in tlsconf) |
| Required | 1 (`url`) |
| Quickstart external deps | 1 (nats; collector optional for observability) |
| "Watch out" entries | 6 |

Design suspects (audit ledger):

- **Open**: publish span parented on
  `context.Background()` (documented limitation — nats.PublishMsg has no ctx; see §2.2
  step 3 for the ctx-aware manual alternative); direct `Conn.Subscribe` /
  JetStream surface consumes untraced; multi-value headers flattened to first value;
  README's "no wrapped connection" rationale contradicts the now-instrumented
  `Conn.PublishMsg` override; guarded methods use `context.Background()` (no
  deadline/cancellation) and get no observe instrumentation; no health.Indicator wired
  by the starter (`Healthy()` is caller-side only).
- **Fixed**: none yet from the previous ledger — all prior suspects remain open.

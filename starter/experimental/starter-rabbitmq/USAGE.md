# starter-rabbitmq Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`config.go`, `starter.go`, `client.go`, `command.go`, `driver.go`)
and the runnable [example/](example/) / [example-otel/](example-otel/). **AMQP 0-9-1 semantics
(exchanges, routing keys, queue durability, publisher confirms, heartbeats) are
[RabbitMQ's own documentation](https://www.rabbitmq.com/docs)** — everything below is
go-spring's increment.

**Activation**: the module registers under `gs.OnProperty("spring.rabbitmq")` [starter.go:34]
— a prefix check, so any `spring.rabbitmq.instances.*` property activates it. Multi-instance only:
each `spring.rabbitmq.instances.<name>` block yields one named `*amqp.Connection` bean; there is no
`__default__` singleton.

---

## 1. Complete worked project

A service with one publisher and one competing-consumer pair via the messaging driver, plus
the raw connection for AMQP features the driver does not model. File tree:

```
demo/
├── go.mod
├── main.go
├── messaging_app.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/rabbitmq/amqp091-go  v1.10.0
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-rabbitmq   latest
    go-spring.org/starter-actuator   latest   // optional: probes + /metrics
    go-spring.org/starter-otel       latest   // optional: real trace/metric export
    go-spring.org/starter-governance-file latest   // optional: runtime resilience/fault for GuardedPublish
)
```

**main.go**:

```go
package main

import (
    _ "demo/messaging_app" // app wiring lives here

    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-rabbitmq"
)

func main() { gs.Run() }
```

**messaging_app.go** — publisher + consumer via the driver, plus one guarded raw publish:

```go
package messaging_app

import (
    "context"

    amqp "github.com/rabbitmq/amqp091-go"
    "go-spring.org/cloud/messaging"
    "go-spring.org/log"
    "go-spring.org/spring/gs"

    StarterRabbitMQ "go-spring.org/starter-rabbitmq"
)

type App struct {
    Conn *amqp.Connection `autowire:"demo"`
}

func init() {
    // Export the app as a Rooter so the container instantiates it in prod even
    // though nothing else injects it (gs wires only root-reachable beans).
    gs.Provide(&App{}).Export(gs.As[gs.Rooter]())

    // The driver adapts the connection to the broker-neutral messaging.Driver.
    // App code then depends only on messaging.Publisher / messaging.Subscriber.
    gs.Provide(StarterRabbitMQ.NewDriver, gs.TagArg("demo"))
}

// Init runs after injection (gs.Rooter): start the competing consumers.
func (a *App) Init(ctx context.Context) error {
    // Driver-provided publisher: trace context + instrumentation ride automatically.
    b := StarterRabbitMQ.NewDriver(a.Conn, nil)
    pub, err := b.NewPublisher(ctx, "orders.created")
    if err != nil { return err }
    if err := pub.Publish(ctx, &messaging.Message{
        Key: "order-42", Payload: []byte(`{"id":42}`),
        Headers: map[string]string{"origin": "demo"},
    }); err != nil { return err }

    // Competing consumers: two subscribers on the same queue share deliveries.
    for i := 0; i < 2; i++ {
        sub, err := b.NewSubscriber(ctx, "orders.created", "")
        if err != nil { return err }
        if err := sub.Subscribe(ctx, handle); err != nil { return err }
    }

    // Raw path escape hatch: exchange routing + governance guard. GuardedPublish
    // declares the publish's identity and routes it through the resilience
    // executor, which emits the span/metrics and injects the W3C trace context
    // (see §2.4).
    ch, err := a.Conn.Channel()
    if err != nil { return err }
    defer ch.Close()
    if err := ch.ExchangeDeclare("logs", "direct", false, true, false, false, nil); err != nil {
        return err
    }
    pub2 := amqp.Publishing{Body: []byte("routed"), ContentType: "text/plain"}
    return StarterRabbitMQ.GuardedPublish(ctx, a.Conn, ch, "logs", "info", false, false, pub2)
}

func handle(ctx context.Context, msg *messaging.Message) error {
    log.Infof(ctx, log.TagAppDef, "got key=%s headers=%v body=%s", msg.Key, msg.Headers, msg.Payload)
    return nil // returning an error → Nack(requeue), see §2.3
}
```

**conf/app.properties** — the complete, commented surface:

```properties
# --- rabbitmq (two named instances would be spring.rabbitmq.instances.<b>.* etc.) -------
spring.rabbitmq.instances.demo.url=amqp://guest:guest@127.0.0.1:5672/
spring.rabbitmq.instances.demo.vhost=
spring.rabbitmq.instances.demo.heartbeat=10s

# TLS (off by default; amqps:// URL also enables it implicitly):
#spring.rabbitmq.instances.demo.tls.enabled=true
#spring.rabbitmq.instances.demo.tls.ca-file=/etc/ssl/rabbit-ca.pem
#spring.rabbitmq.instances.demo.tls.cert-file=/etc/ssl/rabbit-client.pem
#spring.rabbitmq.instances.demo.tls.key-file=/etc/ssl/rabbit-client.key
#spring.rabbitmq.instances.demo.tls.server-name=rabbit.example.com
#spring.rabbitmq.instances.demo.tls.insecure-skip-verify=false

# --- actuator + otel (same keys as example-otel/conf/app.properties) ---------
spring.actuator.addr=:9370
spring.observability.enable=true
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0
```

**Broker startup** (same image as the examples' docker-compose.yml):

```bash
docker run -d --name demo-rabbit -p 127.0.0.1:5672:5672 -p 127.0.0.1:15672:15672 rabbitmq:3-management
# or: cd starter-rabbitmq/example && docker compose up -d
```

**Verify** (example's `-manual` mode serves `/publish` and `/consume` on :9090):

```bash
cd starter-rabbitmq/example && ./check.sh        # full smoke: default + direct exchange,
                                                  # QoS+manual ack
# or interactively: docker compose up -d && sleep 10
go run . -manual
curl http://127.0.0.1:9090/publish    # OK
curl http://127.0.0.1:9090/consume    # value
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle timeline

```
import starter-rabbitmq
  └─ gs.Module(gs.OnProperty("spring.rabbitmq")) registers a group            [starter.go:34]
        │
gs.Run()
  ├─ bind: conf.BindEach over spring.rabbitmq.instances.* → one Config per instance,
  │   Provide newClient with .Name(<instance>).Destroy(destroyClient)         [starter.go:35-39]
  ├─ newClient (per instance):
  │   ├─ optional Driver bean (none → bundled DefaultDriver)                   [starter.go:91-94]
  │   ├─ Driver.CreateClient: TLS build + amqp.Dial/DialConfig — the TCP +
  │   │   AMQP handshake is synchronous, so a bad URL / wrong credentials /
  │   │   TLS mismatch fail the boot, not the first publish                   [starter.go:95-98]
  │   ├─ NotifyClose/NotifyBlocked bridged into go-spring's log (goroutines
  │   │   exit when amqp091 closes the channels on connection shutdown)       [starter.go:106-135]
  │   ├─ applyResilience: assemble — executor indexed by *amqp.Connection     [starter.go:138]
  │   └─ then probe (when ping=true): channel opened and closed to confirm the AMQP layer;
  │       a failed probe releases what was assembled                          [starter.go:148-158]
  ├─ ready: connection beans injectable as *amqp.Connection by instance name
  └─ SIGTERM: destroyClient → closeResilience (executor Close) then
      conn.Close(), which also drains the notifier goroutines                 [starter.go:166-169]
```

### 2.2 Wrap order of the resilience guard

The executor attached to each connection is built inside-out in `applyResilience`
[command.go:255-261]:

```
fault.WrapClientExecutor( mgr.ClientExecutorFor("rabbitmq", service), service, inj )     ← outermost
        │
   observe layer (applied inside resolve)                  ← wrapped around the governed executor
        │
   governed executor (limiter / breaker / retry)           ← around your call
        │
   your call (ch.PublishWithContext)                       ← innermost
```

So a guarded publish is fault-wrapped from the outside: an injected failure flows
through the observe layer (span + `resilience.*` metrics + access log) and the governed
executor's retry / limiter / breaker, exactly as a real failure would. The
executor is built from the injected `*resilience.Manager` / `*fault.Injector` beans —
the manager is required (this starter imports the package that registers it, so "governance off"
is `spring.governance.enabled=false`, never an absent bean), but a fresh manager yields a
transparent pass-through, and `guard` doesn't even find an executor unless one was stored
(command.go:273-282 pass-through).

**What is NOT guarded**: raw `ch.PublishWithContext` / `ch.Get` calls on a channel you opened
yourself, and queue/exchange declares and acks, all bypass resilience. Both the driver's
`Publish` (through `GuardedPublish`) and its consume handler (through `guard`) **are**
guarded, by the connection-scoped executor; give the service label (`rabbitmq:<vhost|url>`)
an all-zero rule to make every call path effectively bare.

### 2.3 One publish and one consume, layer by layer (driver path)

Publish [client.go:109-134]:

1. Envelope → `amqp.Publishing`: `Body` ← Payload, `Headers` ← string headers
   (toAMQPTable, nil when empty), `MessageId` ← `msg.Key`.
2. If the propagator's `IsLoadTest(ctx)`, the marker is stamped into AMQP header `x-loadtest`
   so the consumer recognises synthetic load.
3. `GuardedPublish` declares the publish's identity (`observability.Operation`, see
   observe.go) on the ctx and routes the call into the connection's resilience
   executor. The executor is the single emitter: it opens the "publish" span,
   injects the W3C trace context into `pub.Headers` from the attempt ctx (so the
   traceparent carries the executor's span), records
   `messaging.client.operation.duration` and the per-attempt
   `messaging.client.attempt.duration`, and writes the access log. No module-local
   emission runs on the driver path.
4. `PublishWithContext` to the default exchange (`""`) with the queue name as routing
   key; the executor records the outcome.

Consume [client.go:145-186]:

1. Handler wrapped in `messaging.Recover` — a panic becomes a normal error path
   instead of unwinding into the SDK goroutine.
2. `Consume(autoAck=false)`; a background loop ranges the delivery channel.
3. Per delivery: the upstream W3C trace is extracted from the delivery headers
   (`extractW3C`), the consume's identity is declared, and the handler runs under the
   connection's resilience executor via `guard` — which opens the "consume" span,
   records the metrics and writes the access log. The load-test marker is re-read
   from the AMQP headers and re-entered into the same ctx.
4. `fromDelivery` maps back: `Key` ← MessageId, string-valued headers only,
   `Timestamp` ← delivery timestamp.
5. Handler error → error log + `Nack(multiple=false, requeue=true)` (broker redelivers);
   success → `Ack(false)`. A failed ack/nack is logged as WARN (redelivery risk).

### 2.4 Guarded raw path

`GuardedPublish(ctx, conn, ch, exchange, key, mandatory, immediate, pub)` resolves the
executor **by connection** (a channel may outlive the bean; the executor is scoped to the
connection the starter created) and runs `ch.PublishWithContext` inside it. It declares the
publish's identity from `exchange` (falling back to `key` when the default exchange is
used) and injects the W3C trace context — the executor then emits the signals. On rejection
the publish is never invoked and the error is a resilience sentinel. There is no separate
manual tracing API: declaring + guarding through `GuardedPublish` is the whole raw-channel
surface.

---

## 3. Per-key behavior reference

All keys live under `spring.rabbitmq.instances.<name>.*`. Five own value tags
(config.go:28-55) plus the shared security (6) block = 11; 1 required.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|------------------------|------------------------------|
| `url` | string | — | **Required** (`expr:"$ != ''"`, config.go:33). `amqp://` or `amqps://`; the `amqps://` scheme alone forces the TLS-config dial path [driver.go:69-72]. | Empty → bind error; wrong credentials/TLS → **boot fails** (synchronous dial). |
| `vhost` | string | "" | Overrides the vhost parsed from the URL by being passed into `amqp.Config` [driver.go:72-76]. Also part of the governance service label. | Conflict between URL and vhost → AMQP handshake error at boot. |
| `heartbeat` | duration | 10s | `>0` (or TLS, or vhost set) switches `amqp.Dial` → `amqp.DialConfig` with `Heartbeat` [driver.go:72-80]. `0` = URL/server default. ⚠ With the plain-`Dial` branch (no TLS/vhost), a non-default heartbeat key silently forces the DialConfig branch — value still applies. | Too low → false connection drops under load; 0 → server default may outlive TCP idle timeouts. |
| `ping` | bool | false | Opt-in startup probe: a channel is opened and closed to confirm the AMQP layer is usable [starter.go:148-158]. | true → a TCP-connected but AMQP-broken endpoint aborts boot; false → surfaces on first channel/publish. |
| `tls.enabled` | bool | false | With `amqps://` implicitly, or explicitly, routes the built `*tls.Config` into the dial [driver.go:69-79]. | Plain `amqp://` + `tls.enabled=true` → TLS on a cleartext port → dial error at boot. |
| `tls.ca-file` / `cert-file` / `key-file` | string | "" | Custom CA / mTLS pair, loaded by the shared `security` block (uniform keys across starters, config.go:44-48). | Missing files → boot fails in TLS build [driver.go:64-68]. |
| `tls.server-name` | string | "" | SNI/verification name override. | Mismatch → x509 hostname error at boot. |
| `tls.insecure-skip-verify` | bool | false | Skips cert verification. | true in prod = silent MITM exposure. |

The `driver` key names the Driver bean for this entry: unset → assembly is owned by the
optional Driver bean injected by type (§2.1) or the bundled `DefaultDriver` (config.go /
driver.go); set → that bean by name, and naming a missing bean fails startup. When several
Driver beans coexist, an entry selects one by name: `spring.rabbitmq.instances.<name>.driver =
<bean-name>` (empty = fall back to the family-wide `spring.<family>.default.driver`, then to the single Driver bean by type).

Reconciled against `grep -rhoE 'value:"[^"]+"'` over the starter: own tags are exactly
`${url}`, `${vhost:=}`, `${heartbeat:=10s}`, `${ping:=false}`, `${tls}`; the
tls.* columns come from the shared cloud block bound through `${tls}`.

---

## 4. Verification & fault drills

### 4.1 Broker-down fail fast

```bash
docker stop demo-rabbit && go run .   # in example/: ./check.sh with broker down
# boot aborts with "failed to dial rabbitmq: ..." (driver.go:86) — not a lazy
# failure on first use; with ping=true the probe channel additionally catches a
# TCP-connected but AMQP-broken endpoint (starter.go:69-76).
```

### 4.2 Guarded vs unguarded path

With starter-governance-file and a fault rule on the rabbitmq service
(label `rabbitmq:<vhost>` (colon format; falls back to `rabbitmq:<url>` when vhost empty), starter.go:106):

1. `GuardedPublish` call → resilience-sentinel error, publish never reaches the channel,
   `resilience.*` metrics + `_app_rabbitmq_access` log record emitted.
2. Driver `Publish` under the same rule → the same resilience-sentinel error (it rides the
   connection-scoped executor, §2.2). Raw `PublishWithContext` on your own channel →
   unaffected (no executor hop). Opting out is service-level: an all-zero rule for
   `rabbitmq:<vhost|url>`.

### 4.3 Round-trip / message mapping survival

```bash
curl :9090/publish && curl :9090/consume        # body "value" survives
```

Driver-to-driver round trip: `Payload`, string `Headers`, `Key` (via MessageId) and
consume-side `Timestamp` survive. **Drops**: producer-set `msg.Timestamp` is never written
to `amqp.Publishing.Timestamp` [client.go:90-94]; non-string header values are dropped on
consume [client.go:183-186]; `ContentType`/`DeliveryMode`/priority are not modeled —
driver messages are transient on a non-durable queue, so a broker restart loses them
(see [durability](https://www.rabbitmq.com/docs/durability)).

### 4.4 Observables read-out

The starter **declares**; the resilience executor **emits**. The declared operation's
metric prefix is `messaging.client`, its bounded labels are
`messaging.system=rabbitmq` and `messaging.operation=publish|consume`, and the
(unbounded) destination rides in span attributes and log fields only — never a metric
label.

- Metrics (from the executor): `messaging.client.operation.duration` (whole call,
  retries included), the attempt-level `messaging.client.attempt.duration`,
  `messaging.client.active_requests`, plus `resilience.client.calls` with a
  `status` dimension; the call-level histograms carry `status` too. The declared
  `messaging.destination.name=<queue>` appears on spans/logs, not on these labels.
- Spans: `publish` / `consume` (named by the declared operation), linked across the
  broker by W3C headers.
- Access log: tag `_app_rabbitmq_access` (`log.RegisterAppTag("rabbitmq","access")`);
  failure → Warn, success with a destination → Debug, success without → Info.
- Connection-state counter (starter-local, NOT per-call):
  `messaging.client.connection.state_changes` with `messaging.system=rabbitmq`,
  `state=closed|blocked|unblocked`.
- Verify with example-otel: `docker compose up -d` (rabbitmq + jaeger), `go run .` — it
  self-verifies traces via the Jaeger API :16686 (example-otel/main.go:214-223).

```bash
curl -s :9370/metrics | grep -E 'messaging_client|rabbitmq'
```

### 4.5 Connection lifecycle events

Kill the broker while the app runs: a Warn log `rabbitmq connection closed: code=...
reason=... server=... recover=...` fires [starter.go:91-92]; memory-alarm throttling logs
`connection blocked` / `unblocked` [starter.go:96-101]. There is **no automatic reconnect**
and no health indicator — the process keeps running on a dead connection.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails: `failed to dial rabbitmq` | broker down / wrong URL / credentials | synchronous dial is fail-fast — fix env or URL (driver.go:86). |
| Boot fails: `failed to open probe channel` | TCP connects but AMQP layer broken (e.g. wrong vhost/permissions) | check vhost permissions for the user (only when `ping=true`; starter.go:69-76). |
| Works at boot, publishes later error; close/blocked Warn logs | broker died mid-run; no auto-reconnect | restart process or implement reconnect on the raw bean; watch the `connection closed` Warn. |
| Consumers get nothing | handler error → Nack(requeue) loop; check `rabbitmq driver handler error on %q` Error log | fix the handler; every error requeues forever — no DLQ (client.go:141-146). |
| Messages vanish after broker restart | driver queues are non-durable and messages transient | use the raw connection with durable declares (client.go:64,76; rabbitmq.com/docs/durability). |
| `Key`/headers "lost" between services | other producer wrote real AMQP routing headers / non-string values dropped on consume | headers are string-only in the envelope; Key rides MessageId (client.go:183-194). |
| No traces/metrics from driver | starter-otel not imported | add it; OTel globals are silent no-ops otherwise (command.go:57-59). |
| GuardedPublish returns resilience sentinel | rate-limit hit / circuit open / injected fault | read `resilience.*` metrics + access log; this is the governance contract (command.go:267-275). |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 10 (4 own + 6 tls) |
| Required | 1 (`url`) |
| Quickstart external deps | 1 (RabbitMQ) |
| "Watch out" entries | 6 |

Design suspects (for the audit ledger; none fixed since the last audit):

- Driver hardcodes queue declaration (non-durable, non-exclusive) with no config escape
  (client.go:64,76) — broker-restart data loss by default.
- `Key` ↔ `MessageId` is a lossy convention; routing keys — AMQP's native keying — are
  not modeled by the driver.
- A failed `Ack/Nack` is WARN-logged (client.go:143,145) — check logs if the broker
  redelivers despite successful handlers.
- Producer-set `msg.Timestamp` never reaches `amqp.Publishing.Timestamp` (client.go:90-94).
- Non-string AMQP header values silently dropped on consume (client.go:183-186).
- The driver path is now symmetric in governance (both publish and consume ride the
  executor), but a raw `ch.PublishWithContext` / `ch.Get` on a channel you opened
  yourself still bypasses it entirely.

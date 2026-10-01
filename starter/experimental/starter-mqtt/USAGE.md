# starter-mqtt Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`)
and the runnable [example/](example/) — file:line spot-checks in brackets below. **MQTT protocol
semantics are [MQTT's own documentation](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html),
the client API is [paho.mqtt.golang's](https://github.com/eclipse/paho.mqtt.golang)** —
everything below is go-spring's increment. Honest note: there is no `example-otel` in this
starter; the observability claims below are source-verified, not smoke-verified end-to-end.

**Activation**: any `spring.mqtt.instances.*` property — the module is `gs.Module(gs.OnProperty("spring.mqtt"))`,
a prefix check [starter.go:34]. Each `spring.mqtt.instances.<name>` entry creates one `mqtt.Client` bean
named `<name>` via `conf.BindEach` [starter.go:35-39]. There is no health indicator bean
(unlike redis/nats — see §6).

---

## 1. Complete worked project

A service that publishes sensor readings and consumes them through the broker-neutral `messaging.Driver`, with probes, metrics and runtime governance. File tree:

```
demo/
├── go.mod
├── main.go
├── service.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/eclipse/paho.mqtt.golang v1.5.0
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-mqtt       latest
    go-spring.org/starter-actuator   latest   // optional: probes + /metrics
    go-spring.org/starter-otel       latest   // optional: real trace/metric export
    go-spring.org/starter-governance-file latest   // optional: runtime resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/service"

    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-mqtt"
    _ "go-spring.org/starter-otel"
)

func main() { gs.Run() }
```

**service.go** — raw client for the guarded publish path, driver for the envelope path:

```go
package service

import (
    "context"
    "fmt"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "go-spring.org/cloud/messaging"
    "go-spring.org/spring/gs"
    StarterMQTT "go-spring.org/starter-mqtt"
)

type Service struct {
    // The starter provides the RAW paho client, named after the config entry.
    Client mqtt.Client `autowire:"a"`
}

func init() {
    // The driver is NOT auto-provided: adapt the raw client yourself and
    // export the bean. destination/source strings are MQTT topics.
    gs.Provide(StarterMQTT.NewDriver, gs.TagArg("a")).Export(gs.As[messaging.Driver]())

    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            // (a) guarded publish — declares the publish operation and routes
            //     it through the resilience executor (rate limit / breaker)
            //     attached to client "a"; the executor emits the span, metrics
            //     and access log.
            err := StarterMQTT.GuardedPublish(ctx, s.Client, "sensors/temp", 1, false, []byte("21.5"))
            _ = err

            // (b) driver consume — envelope API, payload-only mapping.
            b := StarterMQTT.NewDriver(s.Client)
            sub, _ := b.NewSubscriber(ctx, "sensors/temp", "")
            return sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
                fmt.Println("received:", string(m.Payload))
                return nil
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- mqtt client "a" --------------------------------------------------------
# Broker URL scheme picks the transport: tcp:// (MQTT) or ssl:// (MQTTS).
spring.mqtt.instances.a.broker=tcp://127.0.0.1:1883
spring.mqtt.instances.a.client-id=demo-publisher
# username / password / clean-session / keep-alive / connect-timeout: see §3.

# Last Will: broker publishes this when the client disconnects ungracefully.
spring.mqtt.instances.a.will.topic=demo/status
spring.mqtt.instances.a.will.payload=offline
spring.mqtt.instances.a.will.qos=1

# MQTTS: switch broker to ssl://host:8883 and enable the tls group:
# spring.mqtt.instances.a.tls.enabled=true
# spring.mqtt.instances.a.tls.ca-file=/etc/mqtt/ca.pem
# spring.mqtt.instances.a.tls.cert-file=/etc/mqtt/client-cert.pem
# spring.mqtt.instances.a.tls.key-file=/etc/mqtt/client-key.pem

# --- actuator + otel --------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # /metrics via actuator only
```

**Verify** (broker per the [example's docker-compose.yml](example/docker-compose.yml), which
starts a no-auth mosquitto on 127.0.0.1:1883):

```bash
docker compose -f example/docker-compose.yml up -d   # or: docker run -d -p 1883:1883 eclipse-mosquitto:2 mosquitto -c /mosquitto-no-auth.conf
go run .                        # boot fails fast if the broker is unreachable
# expected: "mqtt client initialized, broker=tcp://127.0.0.1:1883", then "received: 21.5"
curl -s :9370/metrics | grep -E 'messaging_client'   # resilience-emitted metrics
grep _app_mqtt_access app.log | tail -2              # access records
cd example && ./check.sh                             # full smoke (self-asserting)
mosquitto_sub -t 'demo/status' &                     # kill -9 the app → will "offline" arrives
```

The example's own smoke (`example/check.sh`) runs a QoS 1 pub/sub round-trip, exiting non-zero on any failure [example/check.sh:40-48].

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-mqtt
  └─ gs.Module(gs.OnProperty("spring.mqtt")) fires when any spring.mqtt.instances.* key exists
        └─ conf.BindEach("${spring.mqtt}") → one Config per <name> entry
              └─ Provide(newClient).Name(<name>).Destroy(destroyClient).Caller(1)   [starter.go:36-40]

gs.Run()
  ├─ ctor newClient [starter.go:75]:
  │    1. Driver bean injection — a company Driver bean, when present;
  │       nil (none) falls back to the bundled DefaultDriver           [starter.go:78-81]
  │       (selected per entry by ${spring.mqtt.instances.<name>.driver}: empty = `spring.mqtt.default.driver` then by type,
  │       set = by bean name — naming a missing bean fails startup)
  │    2. CreateClient: assembles paho options (broker, id,
  │       credentials, clean-session, keep-alive, connect-timeout),
  │       bridges connect/lost/reconnecting events into go-spring log  [driver.go:64-72],
  │       builds TLS (security BuildClient) and registers the will            [driver.go:74-85]
  │    3. CreateClient installs the governance executor while it builds the
  │       client — params.ExecutorFor("mqtt", "mqtt:<broker>"), indexed by
  │       client [driver.go:115-123] — so the client is complete when returned
  │    4. then probe: client.Connect() + token.Wait() — a dead broker,
  │       bad credentials or TLS mismatch abort the boot; a failed
  │       connect releases what was assembled                       [starter.go:99-110]
  ├─ readiness: no indicator bean — paho's auto-reconnect (left on) is the
  │  recovery path; connection state is observable via IsConnected() and the
  │  bridged lifecycle logs (warn on connection lost)                  [driver.go:67-69]
  └─ SIGTERM → destroyClient [starter.go:111-117]:
       closeResilience (Close the executor, error discarded)           [command.go:102-107]
       → client.Disconnect(250ms grace for in-flight work)
```

Note there is no separate `Init` phase: connection AND resilience both happen inside the
constructor, so a bean that injects successfully is always connected-and-guarded.

### 2.2 The guard/wrap mechanism — exact order and what is NOT guarded

paho.mqtt.golang ships no reject-capable middleware, so there is no transparent client
wrapper. Instead the seam is an executor attached at construction and **opt-in call-site
guards** [command.go:17-29]:

```
CreateClient [driver.go:115-123] (the assembly seam):
  exec = params.ExecutorFor("mqtt", "mqtt:<broker>")   // injected cloud.ClientParams{Resilience: mgr, Fault: inj}
  attachGuard(cl, exec, label) — stored in sync.Map keyed by the mqtt.Client value  [command.go:98-100]
  (params.ExecutorFor = fault.WrapClientExecutor(mgr.ClientExecutorFor("mqtt", label), label, inj)
   when mgr is present, else the observed-only resilience.Unmanaged)

GuardedPublish [command.go:134-141]:
  ctx = observability.WithOperation(ctx, operation(opPublish, topic))  // declare
  guard() → executor.Execute(ctx, call)                               [command.go:114-121]
  call = cl.Publish(...) + token.Wait() + token.Error()
```

Wrap order (outer→inner): **fault injection → resilience observe (reads the declared
operation off the ctx and emits the span, the call-level and attempt-level duration
histograms, and the access log) → resilience policy (limiter/breaker/retry) → paho Publish
→ token wait**. Rationale (source comments): paho manages its own queueing and reconnect, so
the executor is intentionally minimal — rate-limit the publish rate and short-circuit when
the broker is unhealthy. The service label is `mqtt:<broker-url>` — per broker, not per
topic [driver.go:121].

**What is NOT guarded** (verified):

- plain `client.Publish` called directly on the client bean — bypasses resilience entirely;
  only `GuardedPublish` and the driver's `Publish` route through the executor
  [command.go:134-141, client.go].
- raw `Subscribe` / `Unsubscribe` — no guard exists for subscription setup. A delivery is
  guarded only if the callback routes through `GuardedConsume` [command.go:160-162]; a bare
  callback is unobserved and unguarded.
- driver subscribe handlers: guarded only against panics (`messaging.Recover`
  converts a panic into an error) [client.go:98]; the error is then only logged
  [client.go:107-111]. Each delivery is otherwise declared and guarded through
  `GuardedConsume` [client.go:107-109].

The manager is required — this starter imports the package that registers it, so "governance off"
is `spring.governance.enabled=false`, never an absent bean; a standalone, non-gs caller
passes nil, and the client's executor degrades to the observed-only
`resilience.Unmanaged` (with a one-time warning) rather than to a silent no-op
[starter.go:87-88, driver.go:115-123].

### 2.3 One publish and one consume, layer by layer

**Guarded publish** `GuardedPublish(ctx, cl, "sensors/temp", 1, false, payload)`:

1. `observability.WithOperation(ctx, operation(opPublish, topic))` declares the publish
   identity (`messaging.system`, `messaging.operation`, topic in Detail) [command.go:135,
   observe.go:78-99].
2. `guard` resolves the executor for this client from the sync.Map [command.go:114-121].
3. fault injection check (spring.governance.client.fault.* policy, if enabled).
4. resilience observe opens the call span named `publish` and, once the call returns, emits
   the call-level `messaging.client.operation.duration`, the attempt-level
   `messaging.client.attempt.duration`, the `resilience.client.calls` counter and the one
   access log — all read from the declaration. Publish and consume are declared
   `NonIdempotent`, so a retry policy on the label is **suppressed** (one warning per service):
   re-publishing or re-running the handler is a second side effect, not a second attempt.
5. resilience policy: rate limiter / circuit breaker on service `mqtt:<broker>`;
   on rejection the sentinel error returns and **paho Publish is never invoked**.
6. `cl.Publish(topic, qos, retained, payload)` hands off to paho's outbound queue;
   `token.Wait()` blocks until the packet is written (QoS 0) or the PUBACK/PUBCOMP
   arrives (QoS 1/2) [command.go:137-139].

**Guarded consume** `GuardedConsume(ctx, cl, msg, handler)` is the counterpart for a
subscription: it declares the consume identity (topic from `msg.Topic()`)
and runs the handler under the same executor, so a delivery is observed exactly like a
publish [command.go:160-162]. The messaging.Driver path below uses it for its consume
callback.

**Driver consume** — `sub.Subscribe(ctx, handler)` on topic `sensors/temp`:

1. handler is wrapped in `messaging.Recover` (panic → error) [client.go:98].
2. `cl.Subscribe(topic, 1, callback)` + `token.Wait()` — errors (bad topic filter,
   no broker) return synchronously [client.go:99-114].
3. per delivery, the callback builds a `messaging.Message` from the paho message —
   **Payload only**; topic lives outside the envelope (it is the subscriber's fixed
   source), QoS/retained are not modeled, and Key/Headers/Timestamp do not exist on the
   wire — MQTT 3.1.1 packets carry no per-message metadata [client.go:52-57].
4. the callback then routes the message through `GuardedConsume`, which declares the
   consume identity (topic from `msg.Topic()`) and runs the handler under the client's
   resilience executor [client.go:101-111].
5. handler error → logged at Error level with the topic; no ack/nack, no redelivery
   (fire-and-forget callback) [client.go:107-111].
6. `sub.Close()` → `Unsubscribe(topic)` + wait; the token error is returned (the only
   error NOT discarded on this path) [client.go:117-121].

The driver is fixed at QoS 1 (`defaultQoS`) and `retained=false` on publish; retained
messages, custom QoS and wildcard subscriptions require the raw `mqtt.Client` bean
[client.go:33-37].

---

## 3. Per-key behavior reference

All keys live under `spring.mqtt.instances.<name>.` (per-instance prefix binding via `conf.BindEach`).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `broker` | string | — | **Required** (`expr:"$ != ''"`), e.g. `tcp://host:1883`, `ssl://` for MQTTS. Also becomes the resilience service label `mqtt:<broker>`. | Missing → bind error naming the instance. |
| `client-id` | string | "" | Presented to the broker; empty = library-generated. ⚠ MQTT brokers reject two live connections with the same client-id — scale replicas need distinct ids. | Duplicate ids → connect loop / kicked connections at runtime, not at boot. |
| `username` / `password` | string | "" | Broker auth. | Wrong → fail-fast connect error at boot [starter.go:66-69]. |
| `clean-session` | bool | true | Broker discards session state on disconnect. `false` + stable client-id gives queued-offline-message semantics. | See [MQTT spec §3.1](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html). |
| `keep-alive` | duration | 30s | PING interval. | Too long → slow dead-peer detection. |
| `connect-timeout` | duration | 10s | Bounds the boot-time Connect; `0` disables the timeout. | 0 + dead broker → boot hangs on the token wait. |
| `will.topic` | string | "" | Will is registered **only** when non-empty [driver.go:92-94]. | Setting payload/qos/retained without topic → all silently ignored (dead keys). |
| `will.payload` | string | "" | Will body. | — |
| `will.qos` | byte | 0 | Will QoS (0/1/2). | — |
| `will.retained` | bool | false | Broker retains the will. | — |
| `tls.enabled` | bool | false | Enables `security` client TLS; pair with an `ssl://` broker URL. | Plaintext broker + tls on → connect failure at boot. |
| `tls.ca-file` / `cert-file` / `key-file` | string | — | CA / mutual-TLS client material, `tls.Build()` at client creation [driver.go:83-90]. | Partial config → Build error at boot. |
| `tls.server-name` / `insecure-skip-verify` | string/bool | — | SNI override / skip verification. | — |

Reconciled with `grep -rhoE 'value:"[^"]+"'` over the starter: 13 distinct value tags →
7 flat keys + tls (6) + will (4) = **17 keys, 1 required**.

---

## 4. Verification & fault drills

### 4.1 Boot fail-fast (broker down)

```bash
docker stop <mosquitto> || true
go run .    # exits with "mqtt: connect failed broker=..." [starter.go:67]
docker start <mosquitto> && go run .   # boots, logs "mqtt client initialized" [starter.go:75]
```

### 4.2 Guarded vs unguarded path

With starter-governance-file configured, add a breaker/limiter policy for service
`mqtt:tcp://127.0.0.1:1883`:

```yaml
spring:
  governance:
    enabled: true
    resilience:
      mqtt:tcp://127.0.0.1:1883:
        rateLimiter: { limit: 1, period: 1s }
```

Hammer `GuardedPublish` → rejections surface as resilience sentinel errors and as
`_app_mqtt_access` records. The identical traffic through
plain `client.Publish` on the client bean is unaffected — the driver now rides the same
guard, so opting out is a service-level decision (an all-zero rule for
`mqtt:<broker>`, or governance off), not a per call site one
[command.go:134-162, client.go:78-83]. Policies hot-reload without restart
(governance center).

### 4.3 Message round-trip incl. driver mapping survival

Publish an envelope with Key/Headers/Timestamp set through the driver publisher, consume
with the driver subscriber: **only Payload survives**; Key/Headers/Timestamp arrive zero
— MQTT 3.1.1 has no metadata fields [client.go:52-57, client.go:100]. Round-trip the payload
and assert byte equality; anything beyond payload requires the raw client (e.g. encode
metadata into the payload yourself).

### 4.4 Observability reads

The starter DECLARES; the resilience layer EMITS. Everything below is produced by the
resilience executor from the declared operation — not by the starter.

- Access log: tag `_app_mqtt_access` (observe.go registers `app.mqtt.access`, carried on the
  declaration as `Operation.LogTag`). One record per guarded call: `messaging.system=mqtt`,
  `messaging.operation=publish|consume`, `messaging.destination.name=<topic>`,
  `status=<ok|error>`, `resilience.outcome=<rate_limited|...>` when protection refused the call, `duration_ms=...`; a failure at
  Warn with an `error` field, a success carrying a topic (Detail) at Debug, a topicless
  success at Info.
- Metrics, all under `messaging.client.*`: the call-level `operation.duration` histogram,
  the attempt-level `attempt.duration` histogram, the `active_requests` in-flight gauge
  (labels `messaging.system=mqtt`, `messaging.operation`, `status`), plus the resilience
  layer's own `resilience.client.calls` counter. The topic never labels a metric — it is
  Detail, so it reaches only the span and the log.
- Connection-state counter: `messaging.client.connection.state_changes`
  (`messaging.system=mqtt`, `state`), driven by paho's connect/lost/reconnecting callbacks;
  it stays starter-local and is NOT emitted by the resilience layer.

```bash
curl -s :9370/metrics | grep -E 'messaging_client_(operation_duration|attempt_duration|active_requests|connection_state_changes)'
```

- Spans: publish and consume each open a span named `publish` / `consume`. ⚠ The two sides
  are **independent traces** — W3C trace context cannot ride an MQTT 3.1.1 message
  [command.go:31-38]. All signals are silent no-ops without starter-otel's OTel globals.
- Lifecycle logs (tag `_app_def`): connected / reconnecting (Info), connection lost (Warn)
  [driver.go:72-86].

### 4.5 Will / graceful shutdown

```bash
mosquitto_sub -t 'demo/status' &
go run . &      # graceful SIGTERM: Disconnect(250), NO will fires
kill -9 <pid>   # ungraceful → will "offline" (retained per config) is published by the broker
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "mqtt: connect failed broker=..." | Broker unreachable / wrong credentials / TLS mismatch | Fail-fast connect is unconditional [starter.go:64-69]; fix connectivity or config. |
| Boot hangs (no error) | `connect-timeout=0` with a black-holed address | Keep a finite timeout; 0 disables it [config.go:51]. |
| Reconnect storm / client kicked | Duplicate `client-id` across replicas | Assign distinct ids (broker enforces uniqueness). |
| No breaker/limiter effect | Publishing via plain `client.Publish` | `GuardedPublish` and the driver's `Publish` are guarded [command.go:134-162]; switch call sites. |
| No traces/metrics/access records | starter-otel not imported, or expecting them from the driver | The resilience layer emits from the declared operation and rides the OTel globals; the driver emits nothing [client.go:47-50]. |
| Subscriber silent after broker restart | Subscription lost on unclean session drop | Re-subscription depends on clean-session / broker session; verify with the lifecycle logs [driver.go:76-81]. |
| Handler errors vanish | Driver logs them and moves on — no redelivery | Handle retries inside the handler [client.go:107-111]. |
| TLS keys seem ignored | Broker URL still `tcp://` | Use `ssl://` with `tls.enabled` [config.go:53-55]. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 17 (7 flat + tls 6 + will 4) |
| Required | 1 (`broker`) |
| Quickstart external deps | 1 (MQTT broker — mosquitto via docker compose) |
| "Watch out" entries | 6 |

Design suspects (audit ledger; none fixed since last pass):

- Driver handler errors are only logged; `Recover`'s comment now says exactly that
  (the earlier "nack/redelivery" claim, which MQTT 3.1.1's fire-and-forget callback
  cannot deliver, was corrected) [client.go:95-98].
- Governance is per-call-site opt-in (`GuardedPublish`) and undocumented in the README.
- No health indicator bean (family asymmetry: redis/nats provide one); `IsConnected()` is
  the only liveness signal and nothing probes it automatically.
- `schema.json` marks `will.qos` as type object and omits tls/will sub-keys
  [schema.json:49-53] — schema is stale vs config.go.

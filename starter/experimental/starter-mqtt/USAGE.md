# starter-mqtt Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`)
and the runnable [example/](example/) — file:line spot-checks in brackets below. **MQTT protocol
semantics are [MQTT's own documentation](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html),
the client API is [paho.mqtt.golang's](https://github.com/eclipse/paho.mqtt.golang)** —
everything below is go-spring's increment. Honest note: there is no `example-otel` in this
starter; the observability claims below are source-verified, not smoke-verified end-to-end.

**Activation**: any `spring.mqtt.*` property — the module is `gs.Module(gs.OnProperty("spring.mqtt"))`,
a prefix check [starter.go:34]. Each `spring.mqtt.<name>` entry creates one `mqtt.Client` bean
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
    go-spring.org/starter-governance latest   // optional: runtime resilience/fault policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/service"

    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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
            // (a) guarded publish — routed through the resilience executor
            //     (rate limit / breaker) attached to client "a".
            ctx, sp := StarterMQTT.StartPublishSpan(ctx, "sensors/temp")
            err := StarterMQTT.GuardedPublish(ctx, s.Client, "sensors/temp", 1, false, []byte("21.5"))
            StarterMQTT.EndSpan(sp, err)

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
spring.mqtt.a.broker=tcp://127.0.0.1:1883
spring.mqtt.a.client-id=demo-publisher
# username / password / clean-session / keep-alive / connect-timeout: see §3.

# Last Will: broker publishes this when the client disconnects ungracefully.
spring.mqtt.a.will.topic=demo/status
spring.mqtt.a.will.payload=offline
spring.mqtt.a.will.qos=1

# MQTTS: switch broker to ssl://host:8883 and enable the tls group:
# spring.mqtt.a.tls.enabled=true
# spring.mqtt.a.tls.ca-file=/etc/mqtt/ca.pem
# spring.mqtt.a.tls.cert-file=/etc/mqtt/client-cert.pem
# spring.mqtt.a.tls.key-file=/etc/mqtt/client-key.pem

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
curl -s :9370/metrics | grep -E 'messaging_client'   # observe-kit metrics
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
  └─ gs.Module(gs.OnProperty("spring.mqtt")) fires when any spring.mqtt.* key exists
        └─ conf.BindEach("${spring.mqtt}") → one Config per <name> entry
              └─ Provide(newClient).Name(<name>).Destroy(destroyClient).Caller(1)   [starter.go:36-40]

gs.Run()
  ├─ ctor newClient [starter.go:52]:
  │    1. Driver bean injection — a company Driver bean, when present;
  │       nil (none) falls back to the bundled DefaultDriver           [starter.go:55-58]
  │    2. CreateClient: assembles paho options (broker, id,
  │       credentials, clean-session, keep-alive, connect-timeout),
  │       bridges connect/lost/reconnecting events into go-spring log  [driver.go:64-72],
  │       builds TLS (tlsconf BuildClient) and registers the will            [driver.go:74-85]
  │    3. client.Connect() + token.Wait() — fail-fast probe: a dead
  │       broker, bad credentials or TLS mismatch abort the boot       [starter.go:70-75]
  │    4. applyResilience — attaches the governance executor, indexed
  │       by client; on failure the client is disconnected (250ms)     [starter.go:76-80]
  ├─ readiness: no indicator bean — paho's auto-reconnect (left on) is the
  │  recovery path; connection state is observable via IsConnected() and the
  │  bridged lifecycle logs (warn on connection lost)                  [driver.go:67-69]
  └─ SIGTERM → destroyClient [starter.go:85-92]:
       closeResilience (Close the executor, error discarded)           [command.go:137-142]
       → client.Disconnect(250ms grace for in-flight work)
```

Note there is no separate `Init` phase: connection AND resilience both happen inside the
constructor, so a bean that injects successfully is always connected-and-guarded.

### 2.2 The guard/wrap mechanism — exact order and what is NOT guarded

paho.mqtt.golang ships no hook/plugin extension point, so there is no transparent client
wrapper. Instead the seam is an executor attached at construction and **opt-in call-site
guards** [command.go:17-27]:

```
applyResilience [command.go:128-134]:
  exec = fault.WrapExecutor(resilience.ExecutorFor("mqtt:<broker>"))   // governance center
  exec = resilience.WrapExecutor(exec, "mqtt")                       // observe bridge
  stored in sync.Map keyed by the mqtt.Client value

GuardedPublish [command.go:166-172]:
  guard() → executor.Execute(ctx, "mqtt:<broker>", call)              [command.go:147-154]
  call = cl.Publish(...) + token.Wait() + token.Error()
```

Wrap order (outer→inner): **fault injection → resilience policy (limiter/breaker/retry) →
observe (span+metric+access log of the guarded call) → paho Publish → token wait**. Rationale
(source comments): paho manages its own queueing and reconnect, so the executor is
intentionally minimal — rate-limit the publish rate and short-circuit when the broker is
unhealthy [command.go:117-122]. The resource label is `mqtt:<broker-url>` — per broker, not
per topic [starter.go:70, resilience/config.go:151-155].

**What is NOT guarded** (verified):

- plain `client.Publish` called directly on the client bean — bypasses resilience entirely;
  only `GuardedPublish` and the driver's `Publish` route through the executor
  [command.go:156-172, client.go].
- `Subscribe` / `Unsubscribe` / subscription callbacks — no guard exists for them.
- driver subscribe handlers: guarded only against panics (`messaging.Recover`
  converts a panic into an error) [client.go:92]; the error is then only logged
  [client.go:95-97].

When governance is off, `ExecutorFor` yields a transparent no-op executor, so
`GuardedPublish` behaves exactly like plain publish + wait [command.go:123-127, 156-160].

### 2.3 One publish and one consume, layer by layer

**Guarded publish** `GuardedPublish(ctx, cl, "sensors/temp", 1, false, payload)` with the
span helper wrapped around it:

1. `StartPublishSpan(ctx, topic)` opens a producer observation named `publish` with
   `messaging.destination.name = topic` [command.go, observe.go].
2. `guard` resolves the executor for this client from the sync.Map [command.go:148-153].
3. fault injection check (govern.fault.* policy, if enabled).
4. resilience policy: rate limiter / circuit breaker on resource `mqtt:<broker>`;
   on rejection the sentinel error returns and **paho Publish is never invoked**
   [command.go:160-163].
5. observe bridge records the guarded call's outcome (span/metric/access log).
6. `cl.Publish(topic, qos, retained, payload)` hands off to paho's outbound queue;
   `token.Wait()` blocks until the packet is written (QoS 0) or the PUBACK/PUBCOMP
   arrives (QoS 1/2) [command.go:164-171].
7. `EndSpan(sp, err)` records the outcome and closes the observation [command.go:100-103].

**Driver consume** — `sub.Subscribe(ctx, handler)` on topic `sensors/temp`:

1. handler is wrapped in `messaging.Recover` (panic → error) [client.go:92].
2. `cl.Subscribe(topic, 1, callback)` + `token.Wait()` — errors (bad topic filter,
   no broker) return synchronously [client.go:93-100].
3. per delivery, the callback builds a `messaging.Message` from the paho message:
   **Payload only**. Topic lives outside the envelope (it is the subscriber's fixed
   source), QoS/retained are not modeled, and Key/Headers/Timestamp do not exist on the
   wire — MQTT 3.1.1 packets carry no per-message metadata [client.go:47-52].
4. handler error → logged at Error level with the topic; no ack/nack, no redelivery
   (fire-and-forget callback) [client.go:95-97].
5. `sub.Close()` → `Unsubscribe(topic)` + wait; the token error is returned (the only
   error NOT discarded on this path) [client.go:103-107].

The driver is fixed at QoS 1 (`defaultQoS`) and `retained=false` on publish; retained
messages, custom QoS and wildcard subscriptions require the raw `mqtt.Client` bean
[client.go:33-45].

---

## 3. Per-key behavior reference

All keys live under `spring.mqtt.<name>.` (per-instance prefix binding via `conf.BindEach`).

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `broker` | string | — | **Required** (`expr:"$ != ''"`), e.g. `tcp://host:1883`, `ssl://` for MQTTS. Also becomes the resilience resource label `mqtt:<broker>`. | Missing → bind error naming the instance. |
| `client-id` | string | "" | Presented to the broker; empty = library-generated. ⚠ MQTT brokers reject two live connections with the same client-id — scale replicas need distinct ids. | Duplicate ids → connect loop / kicked connections at runtime, not at boot. |
| `username` / `password` | string | "" | Broker auth. | Wrong → fail-fast connect error at boot [starter.go:66-69]. |
| `clean-session` | bool | true | Broker discards session state on disconnect. `false` + stable client-id gives queued-offline-message semantics. | See [MQTT spec §3.1](https://docs.oasis-open.org/mqtt/mqtt/v3.1.1/os/mqtt-v3.1.1-os.html). |
| `keep-alive` | duration | 30s | PING interval. | Too long → slow dead-peer detection. |
| `connect-timeout` | duration | 10s | Bounds the boot-time Connect; `0` disables the timeout. | 0 + dead broker → boot hangs on the token wait. |
| `will.topic` | string | "" | Will is registered **only** when non-empty [driver.go:92-94]. | Setting payload/qos/retained without topic → all silently ignored (dead keys). |
| `will.payload` | string | "" | Will body. | — |
| `will.qos` | byte | 0 | Will QoS (0/1/2). | — |
| `will.retained` | bool | false | Broker retains the will. | — |
| `tls.enabled` | bool | false | Enables `tlsconf` client TLS; pair with an `ssl://` broker URL. | Plaintext broker + tls on → connect failure at boot. |
| `tls.ca-file` / `cert-file` / `key-file` | string | — | CA / mutual-TLS client material, `tls.Build()` at client creation [driver.go:83-90]. | Partial config → Build error at boot. |
| `tls.server-name` / `insecure-skip-verify` | string/bool | — | SNI override / skip verification. | — |
| `governance` | bool | true | Attaches the resilience/fault executor for the instance; guards both `GuardedPublish` and the driver's `Publish` (same resource label). Transparent no-op when the governance center is off. | `false` → all call paths run bare, govern.* rules never apply. |

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

With starter-governance configured, add a breaker/limiter policy for resource
`mqtt:tcp://127.0.0.1:1883`:

```yaml
govern:
  enabled: true
  resilience:
    mqtt:tcp://127.0.0.1:1883:
      rateLimiter: { limit: 1, period: 1s }
```

Hammer `GuardedPublish` → rejections surface as resilience sentinel errors and as
`_app_mqtt_access` records. The identical traffic through
plain `client.Publish` on the client bean is unaffected — the driver now rides the same
guard, so the opt-out is per instance (`governance=false`), not per call site
opt-in [command.go:147-172, client.go:73-77]. Policies hot-reload without restart
(governance center).

### 4.3 Message round-trip incl. driver mapping survival

Publish an envelope with Key/Headers/Timestamp set through the driver publisher, consume
with the driver subscriber: **only Payload survives**; Key/Headers/Timestamp arrive zero
— MQTT 3.1.1 has no metadata fields [client.go:47-52, client.go:94]. Round-trip the payload
and assert byte equality; anything beyond payload requires the raw client (e.g. encode
metadata into the payload yourself).

### 4.4 Observability reads

- Access log: tag `_app_mqtt_access` (observe.go registers `app.mqtt.access`). One
  record per span-helper observation: `operation=publish|consume duration_ms=...`
  (+ topic), errors at Warn with an `error` field.
- Metrics: `messaging.client.operation.duration` (s) and `messaging.client.active_requests`,
  attributes `messaging.system=mqtt`, `messaging.operation`, and
  `messaging.destination.name` (topic) on spans [observe.go].

```bash
curl -s :9370/metrics | grep -E 'messaging_client_operation_duration|messaging_client_active_requests'
```

- Spans: producer span named `publish`, consumer span named `consume`. ⚠ The two sides are
  **independent traces** — W3C trace context cannot ride an MQTT 3.1.1 message [command.go:44-47].
  All three signals are silent no-ops without starter-otel's OTel globals.
- Lifecycle logs (tag `_app_def`): connected / reconnecting (Info), connection lost (Warn)
  [driver.go:73-81].

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
| No breaker/limiter effect | Publishing via plain `client.Publish`, or `governance=false` on the instance | `GuardedPublish` and the driver's `Publish` are guarded [command.go:156-172]; switch call sites / re-enable. |
| No traces/metrics/access records | starter-otel not imported, or expecting them from the driver | Helpers ride the OTel globals; the driver emits nothing [client.go:47-52]. |
| Subscriber silent after broker restart | Subscription lost on unclean session drop | Re-subscription depends on clean-session / broker session; verify with the lifecycle logs [driver.go:76-81]. |
| Handler errors vanish | Driver logs them and moves on — no redelivery | Handle retries inside the handler [client.go:95-97]. |
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

- Driver handler errors are only logged; `Recover`'s comment claims "nack/redelivery"
  that MQTT 3.1.1's fire-and-forget callback cannot deliver [client.go:90-97].
- Governance is per-call-site opt-in (`GuardedPublish`) and undocumented in the README.
- No health indicator bean (family asymmetry: redis/nats provide one); `IsConnected()` is
  the only liveness signal and nothing probes it automatically.
- `schema.json` marks `will.qos` as type object and omits tls/will sub-keys
  [schema.json:49-53] — schema is stale vs config.go.

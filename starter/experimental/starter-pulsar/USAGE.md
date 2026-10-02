# starter-pulsar Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`config.go`, `starter.go`, `client.go`, `command.go`, `driver.go`,
`observe.go`, `messaging.go`) and the runnable [example/](example/) / [example-otel/](example-otel/) — file:line
spot-checks in brackets below. **Pulsar semantics (subscriptions, keyed messages, properties,
retention/redelivery) are [Pulsar's own documentation](https://pulsar.apache.org/docs/next/client-libraries-go/)**
— everything below is go-spring's increment.

**Activation**: any `spring.pulsar.instances.*` key — the module registers under
`gs.OnProperty("spring.pulsar")`, a prefix check [starter.go:38]. Each `spring.pulsar.instances.<name>`
entry creates one **raw `pulsar.Client` bean named `<name>`** [starter.go:39-44] — there is no
wrapper type, by design: pulsar exposes nothing worth wrapping beyond the client itself
[client.go:17-23].

---

## 1. Complete worked project

A producer AND consumer service via the messaging.Driver, with native metrics, OTel tracing and
governance. File tree:

```
demo/
├── go.mod
├── main.go
├── messaging.go
└── conf/
    └── app.properties
```

**go.mod** (deps that matter):

```
require (
    github.com/apache/pulsar-client-go latest
    go-spring.org/spring               v1.3.x
    go-spring.org/starter-pulsar       latest
    go-spring.org/starter-actuator     latest   // optional: readiness + OTel metrics mount
    go-spring.org/starter-otel         latest   // optional: real trace export
    go-spring.org/starter-governance-file   latest   // optional: breaker/limiter policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/messaging"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-otel"
    _ "go-spring.org/starter-pulsar"
)

func main() { gs.Run() }
```

**messaging.go** — driver-based publish + consume, plus the guarded raw path:

```go
package messaging

import (
    "context"

    "github.com/apache/pulsar-client-go/pulsar"
    "go-spring.org/cloud/messaging"
    "go-spring.org/spring/gs"
    StarterPulsar "go-spring.org/starter-pulsar"
)

func init() {
    // Adapt the injected raw client to the broker-neutral Driver. gs.TagArg("main")
    // picks the spring.pulsar.instances.main instance; the driver bean is unnamed here.
    gs.Provide(StarterPulsar.NewDriver, gs.TagArg("main"))

    gs.Provide(func(b messaging.Driver) (gs.Rooter, error) {
        // Publisher: one producer for the topic, created lazily by the driver.
        pub, err := b.NewPublisher(context.Background(), "persistent://public/demo/orders")
        if err != nil {
            return nil, err
        }
        // Subscriber: "orders-group" becomes the Pulsar Shared subscription name.
        sub, err := b.NewSubscriber(context.Background(),
            "persistent://public/demo/orders", "orders-group")
        if err != nil {
            return nil, err
        }
        // Handler error → Nack → Pulsar redelivers [driver.go:147-151].
        err = sub.Subscribe(context.Background(), func(ctx context.Context, m *messaging.Message) error {
            return process(ctx, m) // Key/Payload/Headers/Timestamp all survived the round trip
        })
        if err != nil {
            return nil, err
        }

        return func(ctx context.Context) error {
            // Driver publish: the driver declares the publish (topic/direction)
            // and runs it under the resilience executor, which emits the span,
            // the messaging.client.* metrics and the access log, and injects the
            // W3C trace context into the message properties [driver.go].
            return pub.Publish(ctx, &messaging.Message{
                Key:     "user-42",                    // becomes the Pulsar message key
                Payload: []byte(`{"amt":100}`),
                Headers: map[string]string{"trace-ctx": "biz"}, // becomes Properties
            })
        }, nil
    })
}

// The guarded RAW path: Producer.Send via GuardedSend is declared and
// resilience-wrapped. GuardedSend declares the operation itself, so it needs no
// manual span helper around it — the resilience layer emits the span.
func guarded(ctx context.Context, cl pulsar.Client, p pulsar.Producer) error {
    msg := &pulsar.ProducerMessage{Payload: []byte("x"), Key: "user-42"}
    id, err := StarterPulsar.GuardedSend(ctx, cl, p, msg)
    _ = id
    return err
}
```

**conf/app.properties** — the complete surface used above:

```properties
# --- pulsar client (instance "main") ----------------------------------------
spring.pulsar.instances.main.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.main.ping=true
# Lookup against a non-partitioned topic succeeds even if absent, so any
# ordinary topic is a safe probe target on a fresh standalone cluster.
spring.pulsar.instances.main.health-check-topic=persistent://public/demo/orders
spring.pulsar.instances.main.operation-timeout=30s
spring.pulsar.instances.main.connection-timeout=5s

# --- native Prometheus metrics (pulsar_client_*, per-instance registry) ----
spring.pulsar.instances.main.metrics.enabled=true
spring.pulsar.instances.main.metrics.port=9091
spring.pulsar.instances.main.metrics.path=/metrics

# --- actuator + otel ---------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=0        # OTel metrics via actuator only

# --- governance (breaker/limiter for service pulsar:pulsar://127.0.0.1:6650)
spring.governance.source.file.path=conf/governance.yaml
```

**Start Pulsar** (same as [example/docker-compose.yml](example/docker-compose.yml)):

```bash
docker run -d --name pulsar -p 127.0.0.1:6650:6650 -p 127.0.0.1:8080:8080 \
    apachepulsar/pulsar:3.2.0 bin/pulsar standalone
# readiness gate (port 6650 opens well before the broker is usable — example/check.sh:36-44):
for i in $(seq 1 60); do curl -fsS http://127.0.0.1:8080/admin/v2/brokers/health \
    | grep -qi ok && break; sleep 1; done
```

**Verify**:

```bash
go run .                                   # ping probe aborts boot if broker is dead
curl -s :9091/metrics | grep pulsar_client_ # native client metrics
curl -s :9370/metrics | grep messaging.client # declared-operation metrics (call + attempt)
grep -E '_app_pulsar|pulsar' app.log        # driver + client log lines (tag _app_def)
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-pulsar
  └─ gs.Module(OnProperty("spring.pulsar")) fires when any spring.pulsar.instances.* key exists
        └─ conf.BindEach("${spring.pulsar}") → one Config per <name> entry   [starter.go:38-46]
              └─ Provide(newClient, IndexArg name+config, IndexArg 3,?Driver).Name(<name>)
                   .Destroy(destroyClient)

gs.Run()
  ├─ ctor newClient [starter.go:89]:
  │    1. optional Driver bean — none → bundled DefaultDriver              [starter.go:92-95]
  │    2. d.CreateClient(c, cloud.ClientParams{Resilience: mgr, Fault: inj}):
  │       ClientOptions, auth (mTLS>token-file>token), TLS, native
  │       Prometheus registry + :port /metrics server, log bridge,
  │       pulsar.NewClient, then attachGuard — params.ExecutorFor("pulsar",
  │       "pulsar:<url>") indexed by client, so the client is COMPLETE when
  │       returned; mgr/inj are the injected *resilience.Manager /
  │       *fault.Injector                                  [driver.go:76-115, command.go:225]
  │    3. then probe (Ping): cl.TopicPartitions(HealthCheckTopic) — a lookup
  │       that exercises address+auth+TLS without producing; failure →
  │       closeResilience + cl.Close + metrics shutdown + boot error     [starter.go:108-116]
  ├─ readiness: no health indicator exists — the probe is boot-time only
  └─ SIGTERM → destroyClient [client.go:45]: closeResilience (executor Close)
       → cl.Close() (releases all producers/consumers) → shutdownMetrics (:port server)
```

**Assembly extension point**: client assembly is owned by a `Driver` (interface,
`driver.go:62-72`): `CreateClient(ctx, c Config, params cloud.ClientParams) (pulsar.Client, error)`.
The driver returns the client COMPLETE — it applies the governance executor itself (via
`attachGuard`) while building, so nothing patches the client afterwards. A company/umbrella starter
may provide its own `Driver` as an **optional container bean** (a
`gs.Provide(func() StarterPulsar.Driver{...})`, so it can inject config bound from the properties
file at wiring time); every instance under `spring.pulsar` is then built through it, and a custom
driver attaches the executor by calling `attachGuard` (same package) before returning. When no such
bean exists the starter falls back to the bundled `DefaultDriver` (`driver.go:74-115`) inside
assembly (`starter.go:92-95`). When several Driver beans coexist, an entry selects one by
name: `spring.pulsar.instances.<name>.driver = <bean-name>` (empty = inject the single Driver bean by
type; naming a missing bean fails startup).

Note `newLogger()` bridges every pulsar-internal log line (connect/reconnect/lookup failures)
into go-spring's log under tag `_app_def` with a `pulsar: ` prefix [driver.go:208-223].

### 2.2 The guard/wrap mechanism — exact order and what is NOT guarded

The resilience executor attached in the ctor is driven through **one seam**, and
each call DECLARES its operation before entering it — the starter declares, the
resilience layer emits:

```
GuardedSend(ctx, cl, producer, msg)                       [command.go]
  ├─ observability.WithOperation(ctx, operation("publish", producer.Topic()))
  │     — declares the direction (span name), messaging.system/operation labels
  │       and the topic as Detail (never a label)
  └─ guard: clientGuards.Load(cl)                         [command.go]
       ├─ not found (driver skipped attachGuard) → producer.Send runs inline
       └─ found → exec.Execute(ctx, injectW3C + send)      — fault-injector outermost
                  (under governance), resilience observer inside it; the observer
                  emits span + messaging.client.* metrics + access log from the
                  declaration; rejection returns a resilience sentinel and the
                  send never reaches the wire
```

The consumer direction declares the same way inside the driver's receive loop
(`operation("consume", source)`), then runs the handler under `guard`
[messaging.go].

The executor attached by `attachGuard` [command.go] comes from
`params.ExecutorFor("pulsar", service)` (`service` = `pulsar:<url>`), where `params` is the
`cloud.ClientParams{Resilience: mgr, Fault: inj}` the ctor hands the driver — `mgr` being the
injected `*resilience.Manager`: with a manager it returns the fully assembled executor (core
breaker/limiter/retry wrapped by the resilience observer — outcome counters + access log) wrapped
outermost by fault injection from the injected `*fault.Injector` (the injected error flows through
the inner retry loop, so the breaker counts it); with the zero bundle it degrades to an
observed-only, loudly-unmanaged executor.

**NOT guarded** (each deliberate, per source comments):
- `producer.SendAsync` — intentionally untouched; the async path has no synchronous outcome
  to reject [command.go].
- `CreateProducer`/`Subscribe`/`TopicPartitions` — lifecycle calls, only the Ping probe
  covers them at boot.
- The manual `StartProducerSpan`/`StartConsumerSpan` helpers — the app's own spans for a raw
  send it drives directly; a send routed through `GuardedSend` is already spanned by the
  resilience layer from the declaration, so do not wrap both around one send.

### 2.3 One driver publish, layer by layer

`pub.Publish(ctx, msg)` with `Key: "k"`, `Headers: h`, load-test marker on ctx
[messaging.go:104-131]:

1. Header copy: if the propagator's `IsLoadTest(ctx)`, headers are **copied** (caller's map never
   mutated) and `x-load-test=1` is added [messaging.go:110-118].
2. Envelope → `pulsar.ProducerMessage`: `Payload`, `Properties` (= headers), and **Key only
   when non-empty** [messaging.go:119-125]. Not mapped: `Timestamp` (messaging envelope has one;
   Pulsar sets publish time server-side) and any Pulsar-specific field (OrderingKey,
   DeliverAt…).
3. `GuardedSend` declares the publish (`operation("publish", producer.Topic())`): the
   direction names the span, `messaging.system`/`messaging.operation` are the bounded
   labels, and the topic is per-call Detail — it reaches the span and the log, never a
   metric label.
4. `producer.Send(attemptCtx, pm)` — synchronous, blocks until broker ack; the W3C trace
   context is injected into `pm.Properties` from the attempt ctx, so the traceparent
   carries the executor's span.
5. The resilience observer (the single emitter) records the outcome on
   `messaging.client.operation.duration` (call level), `messaging.client.attempt.duration`
   (per retry) and `messaging.client.active_requests`, and writes one access log. Publish and
   consume are declared `NonIdempotent`, so a retry policy on the label is **suppressed** (one
   warning per service): re-sending or re-running the handler is a second side effect, not a
   second attempt.

### 2.4 One driver consume, layer by layer

`sub.Subscribe(handler)` starts one background loop [messaging.go:156-196]:

1. Handler is wrapped in `messaging.Recover` — a panic becomes a normal error → Nack →
   redelivery, never an SDK-goroutine crash [messaging.go:159-161].
2. Loop ctx derives from `context.WithoutCancel(ctx)` — Close cancels it explicitly, the
   caller's ctx cancellation does not [messaging.go:157].
3. `c.Receive` → the loop extracts the upstream W3C trace from `msg.Properties()`
   (`extractTraceContext`), declares the consume (`operation("consume", source)`) onto the
   handler ctx, and runs the handler under `guard` — so the resilience observer opens the
   consumer span, records the metrics and writes the access log from the declaration.
4. Load-test marker: if the producer stamped `x-load-test` in Properties, the handler ctx is
   re-marked via `prop.WithLoadTest(ctx)` (the `prop.Extract` above).
5. `fromPulsarMsg` maps back: Pulsar `Key()` → envelope Key, `Payload()`, `Properties()` →
   Headers (including the injected `traceparent` — consumer headers gain keys), `PublishTime()`
   → Timestamp [driver.go]. Both directions preserve Key and Properties.
6. Handler error → `Nack` (redelivery per Shared-subscription semantics) + error log;
   success → `Ack(msg)`, a failed ack is logged as WARN (redelivery risk) [driver.go].
7. A `Receive` error that is not ctx-cancellation is logged and the loop retries [driver.go].

Close order: cancel loop ctx → wait for `done` (in-flight handler finishes) → `consumer.Close()`
[messaging.go:198-207]; publisher Close just calls `producer.Close()` and **discards its
error** [messaging.go:134-136].

---

## 3. Per-key behavior reference

All keys live under `spring.pulsar.instances.<name>.` (BindEach per-instance binding, NOT the
absolute-property Pool rule). 18 value tags found by grep — table covers every one.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `url` | string | — | **Required** (`expr:"$ != ''"`). `pulsar://` plaintext or `pulsar+ssl://` TLS. Also becomes the resilience service label `pulsar\|<url>` [starter.go:76]. | Missing/empty → BindEach boot error. |
| `operation-timeout` | duration | 30s | Producer/subscribe/lookup timeout passed to ClientOptions [driver.go:72]. | Too low → intermittent CreateProducer failures. |
| `connection-timeout` | duration | 5s | TCP connect timeout [driver.go:73]. | — |
| `token` | string | — | JWT token value, OR a file path when `token-from-file=true` [driver.go:86-90]. ⚠ Auth priority: mTLS cert+key beats token if both set. | Wrong token → Ping probe fails at boot (when `ping=true`). |
| `token-from-file` | bool | false | Switches `token` to path interpretation. | true with a literal token → file-open failure. |
| `tls-trust-certs-file` | string | — | PEM CA bundle for broker verification [driver.go:74]. | Missing on `pulsar+ssl://` → handshake failure at probe. |
| `tls-cert-file` | string | — | Client cert; with `tls-key-file` also becomes the mTLS auth provider via `NewAuthenticationTLS` [driver.go:84-85]. ⚠ Only cert without key → silently no auth. | — |
| `tls-key-file` | string | — | Client private key pairing with cert [driver.go:76]. | — |
| `tls-allow-insecure` | bool | false | Disables server cert verification. Never in production. | true → MITM exposure. |
| `tls-validate-hostname` | bool | false | Hostname-in-cert verification; default preserves pulsar-client-go's default [config.go:63-66]. | — |
| `ping` | bool | false | Opt-in startup `TopicPartitions` probe [starter.go:107-114]. | true → dead broker aborts boot; false → surfaces only on first produce. |
| `health-check-topic` | string | `persistent://public/default/__health_check` | Probe target; lookup on a non-partitioned topic succeeds even when absent [config.go:72-76]. | Partitioned/garbage topic name → probe error blocks boot. |
| `metrics` | group | — | Struct binding `value:"${metrics}"` [config.go:79]. | — |
| `metrics.enabled` | bool | true | Starts the per-instance `/metrics` server and wires the dedicated registry [driver.go:97-101]. | false → no `pulsar_client_*` anywhere. |
| `metrics.port` | int | 9091 | Port of that server. ⚠ Fixed default: every metrics-enabled instance MUST get a distinct port; collision = second server's listen fails silently (error swallowed [command.go:96]). | Two instances, one port → one metrics endpoint silently dead. |
| `metrics.path` | string | `/metrics` | HTTP path on that server [command.go:88]. | — |

The `driver` key names the Driver bean for this entry: unset → assembly is owned by the
optional Driver bean injected by type (see §2.1) or the bundled `DefaultDriver`; set → that
bean by name, and naming a missing bean fails startup.

`schema.json` states `metrics.enabled` default `false` while the code default is `true` —
trust the code.

---

## 4. Verification & fault drills

### 4.1 Boot-time ping probe

```bash
docker stop pulsar && go run .    # boot aborts: "pulsar broker probe failed on pulsar://..."
docker start pulsar && go run .   # boots once the admin health endpoint answers (§1 gate)
```

### 4.2 Message round-trip incl. driver mapping survival

Publish `Key="user-42"`, `Headers={"h1":"v1"}` per §1; in the handler log
`m.Key, m.Headers["h1"], string(m.Payload)` — all three survive, plus injected
`traceparent` in Headers. Verify with the example's smoke path (`example/check.sh`).

### 4.3 Guarded vs unguarded (governance label check)

```yaml
# conf/governance.yaml
spring:
  governance:
    enabled: true
    resilience:
      breaker:
        enabled: true
        min-calls: 4
        failure-rate: 50
```

Service label is `pulsar:pulsar://127.0.0.1:6650` [starter.go:76]. Kill the broker, then
hammer `GuardedSend` — or driver `Publish`, which routes through the same seam — → after the
threshold the breaker opens, calls fail fast with a resilience sentinel, and the
`messaging.client.*` (declared-operation) and `resilience.client.*` counters appear. The
unguarded contrast is `producer.SendAsync` and the lifecycle calls (`CreateProducer` /
`Subscribe`): they block into the client's own retry/timeout, no sentinel, no breaker — that
contrast IS the §2.2 boundary.

### 4.4 Metrics / span / log reads

- Native: `curl -s :9091/metrics | grep pulsar_client_` (producer/consumer/connection stats;
  separate per-instance registry so instances never collide [command.go:85-100]).
- OTel: the declared operations emit spans named `publish` / `consume` from the resilience
  layer (traces link via W3C context in Properties), plus `messaging.client.operation.duration`
  and `messaging.client.attempt.duration`; manual app spans are named `pulsar.produce` /
  `pulsar.consume <topic>` with `messaging.system=pulsar` [command.go]. Check Jaeger
  (`:16686`) after traffic.
- Logs: driver handler/receive errors and all bridged client-internal lines land under the
  `_app_def` tag with `pulsar: ` prefix.

### 4.5 Shutdown drill

SIGTERM → destroyClient closes the executor, the client (all producers/consumers), and the
metrics server [client.go:44-58]. Subscriber Close drains its loop before consumer.Close
[messaging.go:198-207]. Watch the log for a clean exit; :9091 stops serving.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "pulsar broker probe failed" | Broker down / wrong url / auth / TLS | Fix connectivity; 6650 open ≠ ready, gate on `:8080/admin/v2/brokers/health`. |
| Second instance has no /metrics | `metrics.port` collision; listen failure WARN-logged [command.go:96] | Assign distinct ports. |
| No traces | starter-otel not imported | Import it; all helpers are silent no-ops without it. |
| Messages redelivered though handler succeeded | Ack failed (WARN logged) [messaging.go] | Check broker ack permission; suspect listed in §6. |
| Consumer never gets messages | Wrong subscription name / Shared vs topic semantics | `group` maps 1:1 to subscription; empty group derives `go-spring-<topic>` [messaging.go:77-92]. |
| Expecting token auth, broker refuses | mTLS cert+key set → token ignored (priority order) [driver.go:83-90] | Remove cert/key or disable mTLS requirement. |
| Breaker never trips under load | Traffic goes through `producer.SendAsync` or the manual helpers — unguarded (§2.2) | Route the synchronous send through `GuardedSend` (driver `Publish` already does). |
| Handler panic kills nothing but message reappears | Recover converts panic → Nack | Expected; fix the handler. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 17 value tags |
| Required | 1 (`url`) |
| Quickstart external deps | 1 (Pulsar standalone) |
| "Watch out" entries | 8 |

Design suspects (audit ledger): `metrics.port` fixed default 9091 collides across instances
and with other apps, and the listen failure is swallowed; both driver Publish and the
driver's consume loop now declare their operation and run under the same executor as the raw
path, so the starter emits nothing per call — the resilience layer is the single emitter
(`SendAsync` and the lifecycle calls remain unguarded); a failed
consumer `Ack` is WARN-logged; `producer.Close()` has no
error return so publisher Close cannot fail; no runtime health indicator
(the ping probe is boot-only — a broker dying later is invisible to actuator); `schema.json`
`metrics.enabled` default disagrees with code (true).

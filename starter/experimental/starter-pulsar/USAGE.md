# starter-pulsar Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`config.go`, `starter.go`, `client.go`, `command.go`, `driver.go`,
`driver.go`) and the runnable [example/](example/) / [example-otel/](example-otel/) — file:line
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
    go-spring.org/starter-governance   latest   // optional: breaker/limiter policy
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "demo/messaging"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
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
            // Driver publish: span + trace-context injection built in [driver.go:98-101].
            return pub.Publish(ctx, &messaging.Message{
                Key:     "user-42",                    // becomes the Pulsar message key
                Payload: []byte(`{"amt":100}`),
                Headers: map[string]string{"trace-ctx": "biz"}, // becomes Properties
            })
        }, nil
    })
}

// The guarded RAW path: only Producer.Send via GuardedSend is resilience-wrapped.
func guarded(ctx context.Context, cl pulsar.Client, p pulsar.Producer) error {
    msg := &pulsar.ProducerMessage{Payload: []byte("x"), Key: "user-42"}
    ctx, span := StarterPulsar.StartProducerSpan(ctx, msg) // manual span helper
    id, err := StarterPulsar.GuardedSend(ctx, cl, p, msg)
    StarterPulsar.EndSpan(span, err)
    _ = id
    return err
}
```

**conf/app.properties** — the complete surface used above:

```properties
# --- pulsar client (instance "main") ----------------------------------------
spring.pulsar.instances.main.url=pulsar://127.0.0.1:6650
spring.pulsar.instances.main.fail-fast=true
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

# --- governance (breaker/limiter for resource pulsar:pulsar://127.0.0.1:6650)
govern.source.file.path=conf/govern.yaml
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
go run .                                   # fail-fast probe aborts boot if broker is dead
curl -s :9091/metrics | grep pulsar_client_ # native client metrics
curl -s :9370/metrics | grep messaging      # driver-path per-message metrics
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
  ├─ ctor newClient [starter.go:56-85]:
  │    1. optional Driver bean — none → bundled DefaultDriver              [starter.go:61-63]
  │    2. d.CreateClient: ClientOptions, auth (mTLS>token-file>token), TLS,
  │       native Prometheus registry + :port /metrics server, log bridge,
  │       pulsar.NewClient                                              [driver.go:59-105]
  │    3. FailFast probe: cl.TopicPartitions(HealthCheckTopic) — a lookup
  │       that exercises address+auth+TLS without producing; failure →
  │       cl.Close + metrics shutdown + boot error                       [starter.go:69-76]
  │    4. applyResilience: fault.WrapExecutor(resilience.ExecutorFor("pulsar", "pulsar:<url>"))
  │       → indexed by client                                           [command.go:229-230]
  ├─ readiness: no health indicator exists — the probe is boot-time only
  └─ SIGTERM → destroyClient [client.go:44-49]: closeResilience (executor Close)
       → cl.Close() (releases all producers/consumers) → shutdownMetrics (:port server)
```

**Assembly extension point**: client assembly is owned by a `Driver` (interface, `driver.go:27-38`).
A company/umbrella starter may provide its own `Driver` as an **optional container bean** (a
`gs.Provide(func() StarterPulsar.Driver{...})`, so it can inject config bound from the properties
file at wiring time); every instance under `spring.pulsar` is then built through it. When no such
bean exists the starter falls back to the bundled `DefaultDriver` (`driver.go:40-105`) inside
assembly (`starter.go:61-63`). When several Driver beans coexist, an entry selects one by
name: `spring.pulsar.instances.<name>.driver = <bean-name>` (empty = inject the single Driver bean by
type; naming a missing bean fails startup).

Note `newLogger()` bridges every pulsar-internal log line (connect/reconnect/lookup failures)
into go-spring's log under tag `_app_def` with a `pulsar: ` prefix [driver.go:208-223].

### 2.2 The guard/wrap mechanism — exact order and what is NOT guarded

The resilience executor attached in the ctor is only driven through **one seam**:

```
GuardedSend(ctx, cl, producer, msg)                       [command.go:263-274]
  └─ guard: resilienceExecs.Load(cl)                      [command.go:243-250]
       ├─ not found (governance off) → producer.Send runs inline, identical to raw
       └─ found → exec.Execute(ctx, "pulsar:<url>", send)  — fault-injector outermost
                  (fault.WrapExecutor), resilience observer inside it; rejection returns
                  a resilience sentinel and the send never reaches the wire
```

Wrap order inside `applyResilience` [command.go:229]: `resilience.ExecutorFor("pulsar", resource)`
returns the fully assembled executor (core breaker/limiter/retry wrapped by the resilience
observer — outcome counters + access log) → wrapped outermost by `fault.WrapExecutor` (runtime
fault injection; the injected error flows through the inner retry loop, so the breaker counts it).

**NOT guarded** (each deliberate, per source comments):
- `producer.SendAsync` — intentionally untouched; the async path has no synchronous outcome
  to reject [command.go:261-263].
- ~~driver `Publish`~~ — **now guarded**: the driver routes through `GuardedSend` with the
  client-scoped executor [driver.go], so driver publishes get span+trace injection *and*
  breaker/limiter/fault. Give the resource label (`pulsar:<url>`) an all-zero rule to make
  every call path effectively bare.
- Consumer `Receive`/handlers — no consumer-side protection exists.
- `CreateProducer`/`Subscribe`/`TopicPartitions` — lifecycle calls, only the FailFast probe
  covers them at boot.

### 2.3 One driver publish, layer by layer

`pub.Publish(ctx, msg)` with `Key: "k"`, `Headers: h`, load-test marker on ctx
[driver.go:81-102]:

1. Header copy: if `traffic.IsLoadTest(ctx)`, headers are **copied** (caller's map never
   mutated) and `x-load-test=1` is added [driver.go:85-90].
2. Envelope → `pulsar.ProducerMessage`: `Payload`, `Properties` (= headers), and **Key only
   when non-empty** [driver.go:91-97]. Not mapped: `Timestamp` (messaging envelope has one;
   Pulsar sets publish time server-side) and any Pulsar-specific field (OrderingKey,
   DeliverAt…).
3. `startProduce` opens the module-local observation (span `publish`, duration/in-flight
   metrics, access log) and injects W3C trace context into `pm.Properties`.
4. `producer.Send(ctx, pm)` — synchronous, blocks until broker ack.
5. `sp.End(err)` records the outcome on all three signals.

### 2.4 One driver consume, layer by layer

`sub.Subscribe(handler)` starts one background loop [driver.go:119-155]:

1. Handler is wrapped in `messaging.Recover` — a panic becomes a normal error → Nack →
   redelivery, never an SDK-goroutine crash [driver.go:121-123].
2. Loop ctx derives from `context.WithoutCancel(ctx)` — Close cancels it explicitly, the
   caller's ctx cancellation does not [driver.go:123].
3. `c.Receive` → `startConsume` extracts the upstream trace from `msg.Properties()` and opens
   a consumer-span child [command.go:195-198].
4. Load-test marker: if the producer stamped `x-load-test` in Properties, the handler ctx is
   re-marked via `traffic.WithLoadTest` [driver.go:141-143].
5. `fromPulsarMsg` maps back: Pulsar `Key()` → envelope Key, `Payload()`, `Properties()` →
   Headers (including the injected `traceparent` — consumer headers gain keys), `PublishTime()`
   → Timestamp [driver.go:171-178]. Both directions preserve Key and Properties.
6. Handler error → `Nack` (redelivery per Shared-subscription semantics) + error log;
   success → `Ack(msg)`, a failed ack is logged as WARN (redelivery risk) [driver.go:145-151].
7. A `Receive` error that is not ctx-cancellation is logged and the loop retries [driver.go:131-137].

Close order: cancel loop ctx → wait for `done` (in-flight handler finishes) → `consumer.Close()`
[driver.go:157-168]; publisher Close just calls `producer.Close()` and **discards its error**
[driver.go:104-107].

---

## 3. Per-key behavior reference

All keys live under `spring.pulsar.instances.<name>.` (BindEach per-instance binding, NOT the
absolute-property Pool rule). 18 value tags found by grep — table covers every one.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `url` | string | — | **Required** (`expr:"$ != ''"`). `pulsar://` plaintext or `pulsar+ssl://` TLS. Also becomes the resilience resource label `pulsar\|<url>` [starter.go:76]. | Missing/empty → BindEach boot error. |
| `operation-timeout` | duration | 30s | Producer/subscribe/lookup timeout passed to ClientOptions [driver.go:72]. | Too low → intermittent CreateProducer failures. |
| `connection-timeout` | duration | 5s | TCP connect timeout [driver.go:73]. | — |
| `token` | string | — | JWT token value, OR a file path when `token-from-file=true` [driver.go:86-90]. ⚠ Auth priority: mTLS cert+key beats token if both set. | Wrong token → FailFast probe fails at boot. |
| `token-from-file` | bool | false | Switches `token` to path interpretation. | true with a literal token → file-open failure. |
| `tls-trust-certs-file` | string | — | PEM CA bundle for broker verification [driver.go:74]. | Missing on `pulsar+ssl://` → handshake failure at probe. |
| `tls-cert-file` | string | — | Client cert; with `tls-key-file` also becomes the mTLS auth provider via `NewAuthenticationTLS` [driver.go:84-85]. ⚠ Only cert without key → silently no auth. | — |
| `tls-key-file` | string | — | Client private key pairing with cert [driver.go:76]. | — |
| `tls-allow-insecure` | bool | false | Disables server cert verification. Never in production. | true → MITM exposure. |
| `tls-validate-hostname` | bool | false | Hostname-in-cert verification; default preserves pulsar-client-go's default [config.go:63-66]. | — |
| `fail-fast` | bool | true | Startup `TopicPartitions` probe [starter.go:68-75]. | false → dead broker surfaces only on first produce. |
| `health-check-topic` | string | `persistent://public/default/__health_check` | Probe target; lookup on a non-partitioned topic succeeds even when absent [config.go:72-76]. | Partitioned/garbage topic name → probe error blocks boot. |
| `metrics` | group | — | Struct binding `value:"${metrics}"` [config.go:79]. | — |
| `metrics.enabled` | bool | true | Starts the per-instance `/metrics` server and wires the dedicated registry [driver.go:97-101]. | false → no `pulsar_client_*` anywhere. |
| `metrics.port` | int | 9091 | Port of that server. ⚠ Fixed default: every metrics-enabled instance MUST get a distinct port; collision = second server's listen fails silently (error swallowed [command.go:69-71]). | Two instances, one port → one metrics endpoint silently dead. |
| `metrics.path` | string | `/metrics` | HTTP path on that server [command.go:62]. | — |

The `driver` key names the Driver bean for this entry: unset → assembly is owned by the
optional Driver bean injected by type (see §2.1) or the bundled `DefaultDriver`; set → that
bean by name, and naming a missing bean fails startup.

`schema.json` states `metrics.enabled` default `false` while the code default is `true` —
trust the code.

---

## 4. Verification & fault drills

### 4.1 Boot-time fail-fast

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
# conf/govern.yaml
govern:
  enabled: true
  resilience:
    breaker:
      enabled: true
      min-calls: 4
      failure-rate: 50
```

Resource label is `pulsar:pulsar://127.0.0.1:6650` [starter.go:76]. Kill the broker, then:
hammer `GuardedSend` → after the threshold the breaker opens, calls fail fast with a
resilience sentinel, and resilience outcome counters appear;
hammer driver `Publish` instead → every call blocks into the client's own retry/timeout, no
sentinel, no breaker. That contrast IS the §2.2 boundary.

### 4.4 Metrics / span / log reads

- Native: `curl -s :9091/metrics | grep pulsar_client_` (producer/consumer/connection stats;
  separate per-instance registry so instances never collide [command.go:59-74]).
- OTel: driver-path spans `publish` / `consume` (traces link via W3C context in Properties);
  manual helpers emit `pulsar.produce` / `pulsar.consume <topic>` with
  `messaging.system=pulsar` [command.go:99-134]. Check Jaeger (`:16686`) after traffic.
- Logs: driver handler/receive errors and all bridged client-internal lines land under the
  `_app_def` tag with `pulsar: ` prefix.

### 4.5 Shutdown drill

SIGTERM → destroyClient closes the executor, the client (all producers/consumers), and the
metrics server [client.go:44-58]. Subscriber Close drains its loop before consumer.Close
[driver.go:157-168]. Watch the log for a clean exit; :9091 stops serving.

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "pulsar broker probe failed" | Broker down / wrong url / auth / TLS | Fix connectivity; 6650 open ≠ ready, gate on `:8080/admin/v2/brokers/health`. |
| Second instance has no /metrics | `metrics.port` collision; listen failure WARN-logged [command.go:69-71] | Assign distinct ports. |
| No traces | starter-otel not imported | Import it; all helpers are silent no-ops without it. |
| Messages redelivered though handler succeeded | Ack failed (WARN logged) [driver.go:158] | Check broker ack permission; suspect listed in §6. |
| Consumer never gets messages | Wrong subscription name / Shared vs topic semantics | `group` maps 1:1 to subscription; empty group derives `go-spring-<topic>` [driver.go:63-66]. |
| Expecting token auth, broker refuses | mTLS cert+key set → token ignored (priority order) [driver.go:83-90] | Remove cert/key or disable mTLS requirement. |
| Breaker never trips under load | Traffic goes through driver Publish or SendAsync — unguarded (§2.2) | Route through GuardedSend. |
| Handler panic kills nothing but message reappears | Recover converts panic → Nack | Expected; fix the handler. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 17 value tags |
| Required | 1 (`url`) |
| Quickstart external deps | 1 (Pulsar standalone) |
| "Watch out" entries | 8 |

Design suspects (audit ledger): `metrics.port` fixed default 9091 collides across instances
and with other apps, and the listen failure is swallowed; driver Publish is now guarded
through the same executor as the raw path (Subscribe/consume remains unguarded); a failed
consumer `Ack` is WARN-logged; `producer.Close()` has no
error return so publisher Close cannot fail; no runtime health indicator
(fail-fast is boot-only — a broker dying later is invisible to actuator); `schema.json`
`metrics.enabled` default disagrees with code (true).

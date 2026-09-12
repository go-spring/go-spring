# starter-kafka Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`)
and the runnable [example/](example/), [example-cloudnative/](example-cloudnative/),
[example-otel/](example-otel/) — file:line spot-checks in brackets below. **Kafka broker
semantics and the franz-go API belong to [franz-go's documentation](https://github.com/twmb/franz-go)
and [kafka.apache.org](https://kafka.apache.org/documentation/)** — below is go-spring's increment.

**Activation**: any `spring.kafka.instances.*` key (the module is `gs.Module(gs.OnProperty("spring.kafka"))`,
a prefix check [starter.go:38]). Each `spring.kafka.instances.<name>` entry creates one `*kgo.Client` bean
named `<name>`. **No health indicator is registered by the starter** — see §4.1 for the app-side
pattern.

---

## 1. Complete worked project

A producer + consumer service using the messaging driver, with health probes, metrics, tracing and runtime governance. Files: `go.mod`, `main.go`, `service.go`, `conf/app.properties`.

**go.mod** (deps that matter): `github.com/twmb/franz-go/pkg/kgo`, `go-spring.org/spring`,
`go-spring.org/cloud`, `go-spring.org/starter-kafka`, plus optional `starter-actuator`
(probes + /metrics), `starter-otel` (trace/metric export), `starter-governance` (rate limit/breaker).

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance"
    _ "go-spring.org/starter-kafka"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — publish and consume `messaging.Message` envelopes through the driver, plus the
health-indicator escape hatch:

```go
package service

import (
    "context"
    "time"

    "github.com/twmb/franz-go/pkg/kgo"
    "go-spring.org/cloud/actuator/health"
    "go-spring.org/cloud/messaging"
    "go-spring.org/spring/gs"
    StarterKafka "go-spring.org/starter-kafka"
)

type Service struct {
    Client *kgo.Client `autowire:"a"` // raw client stays available for admin/transactions
}

// Health* implements health.Indicator; the starter registers none itself, so
// the app exports the Ping probe (readiness+startup groups, critical).
func (s *Service) HealthName() string { return "kafka:a" }
func (s *Service) HealthGroups() []health.Group { return []health.Group{health.GroupReadiness, health.GroupStartup} }
func (s *Service) IsCritical() bool { return true }
func (s *Service) CheckHealth(ctx context.Context) error {
    pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    return s.Client.Ping(pingCtx)
}

func init() {
    gs.Provide(func(s *Service) (messaging.Driver, error) {
        return StarterKafka.NewDriver(s.Client), nil
    })
    gs.Provide(func(b messaging.Driver) gs.Runner {
        return func(ctx context.Context) {
            pub, _ := b.NewPublisher(ctx, "hello")
            _ = pub.Publish(ctx, &messaging.Message{
                Key: "k1", Payload: []byte("value"),
                Headers: map[string]string{"origin": "demo"},
            })
            // group comes from client config — the group arg here is dead (§2.3)!
            sub, _ := b.NewSubscriber(ctx, "hello", "")
            _ = sub.Subscribe(ctx, func(ctx context.Context, m *messaging.Message) error {
                return nil // errors are only logged — see §2.3
            })
        }
    })
}
```

**conf/app.properties** — the complete, commented surface actually used above:

```properties
# --- kafka instance "a" (producer + consumer in one client) ------------------
spring.kafka.instances.a.brokers=127.0.0.1:9092
spring.kafka.instances.a.topic=hello
spring.kafka.instances.a.group=hello-group
spring.kafka.instances.a.producer.required-acks=all
spring.kafka.instances.a.producer.compression=snappy

# SASL / TLS are off against a plaintext dev broker; on a secured cluster set
# sasl.enabled/mechanism/username/password + tls.enabled/ca-file (see §3).

# --- actuator + otel --------------------------------------------------------
spring.http.server.enabled=false
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.metrics.exporter=prometheus

# --- governance (rate limit on the sync produce path) -----------------------
# NOTE: governance RULES go in conf/govern.properties, referenced by govern.source.file.path in app.properties (see starter-governance USAGE).
govern.enabled=true
govern.driver=default
govern.default.enabled=true
govern.default.rate-limit=8
```

**Verify** (broker startup mirrors [example/docker-compose.yml](example/docker-compose.yml) —
KRaft mode, auto-create topics, port 127.0.0.1:9092):

```bash
docker compose up -d                        # bitnami/kafka:3.7, KRaft single node
# wait for the port, broker boot is slow (~30s)
go run .                                    # boot fails fast if brokers unreachable
curl -s :9370/readyz | jq .                 # kafka:a component (app-side indicator)
grep kafka.access app.log | tail -3         # publish/consume access records
```

The self-asserting smoke test is [example/check.sh](example/check.sh); the governance/health/
dync combo is [example-cloudnative/](example-cloudnative/) (`go run .` prints health /
round-trip / resilience / hot-reload lines and exits 0).

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-kafka
  └─ gs.Module(OnProperty("spring.kafka")) fires when any spring.kafka.instances.* key exists
        └─ conf.BindEach("${spring.kafka}") → one Config per <name> entry
              └─ Provide(newClient, IndexArg(name,c), IndexArg(3,?Driver)).Name(<name>) [starter.go:39-45]
              .Destroy(destroyClient)

gs.Run()
  ├─ ctor newClient [starter.go:63]:
  │   1. optional Driver bean — none → bundled DefaultDriver                [starter.go:67-68]
  │   2. d.CreateClient: full client assembly (see §2.2)                    [driver.go:57]
  │   3. Ping with 10s timeout — bad brokers/credentials/TLS fail the
  │      boot instead of the first produce                                [starter.go:51,76]
  │   4. applyResilience: fault.WrapExecutor(resilience.ExecutorFor(
  │      "kafka", "kafka:<brokers>")), indexed by
  │      client pointer in package-level sync.Maps                        [command.go:99-105]
  ├─ no Init hook; the *kgo.Client bean is ready after the ctor
  ├─ Destroy(destroyClient) [starter.go:95-102]:
  │   closeResilience (executor Close) → Flush(10s ctx) → Close.
  │   ⚠ a Flush failure is logged as ERROR (messages may be LOST) and returned — buffered
  │   records that never reached the broker are lost on shutdown.
```

**Assembly extension point**: client assembly is owned by a `Driver` (interface, `driver.go:33-44`).
A company/umbrella starter may provide its own `Driver` as an **optional container bean** (a
`gs.Provide(func() StarterKafka.Driver{...})`, so it can inject config bound from the properties
file at wiring time); every instance under `spring.kafka` is then built through it. When no such
bean exists the starter falls back to the bundled `DefaultDriver` (`driver.go:47-95`) inside
assembly (`starter.go:67-68`). When several Driver beans coexist, an entry selects one
by name: `spring.kafka.instances.<name>.driver = <bean-name>` (empty = inject the single Driver
bean by type; naming a missing bean fails startup).

The resilience registries keyed by `*kgo.Client` pointer are why `GuardedProduceSync` can be a
free function taking the raw bean — the guard resolves the executor without wrapping the client
type [command.go:118-125].

### 2.2 Client assembly — what DefaultDriver installs, in order

`DefaultDriver.CreateClient` builds one `kgo.NewClient` call [driver.go:67-104]:

1. `kgo.SeedBrokers(strings.Split(c.Brokers, ",")...)` — brokers is a CSV of seeds; the client
   learns the full cluster itself.
2. `kgo.WithHooks(append(kt.Hooks(), newObserveHook())...)` [driver.go:74] — **kotel
   (tracer+meter) hooks first, then the log-only access hook**. Rationale: kotel owns spans and
   metrics, so the access hook emits only the access log (observe.go) — no duplicate signals.
3. `kgo.WithLogger(newLogger())` — franz-go internal logs (broker connects, request failures,
   reconnects) bridge into go-spring's log at tag `log.TagAppDef`, Info level threshold
   [driver.go:173-193].
4. `kgo.ConsumerGroup` / `kgo.ConsumeTopics` from `group`/`topic` config — **fixed at
   construction**; that is a franz-go constraint the driver inherits (§2.3).
5. SASL mechanism, TLS (`c.TLS.BuildClient()`), producer options (compression/acks/batch/linger).

The startup ping and resilience wiring are deliberately **not** in the driver — they are the
starter's lifecycle concerns [driver.go:62-66 comment].

### 2.3 One publish and one consume through the driver, layer by layer

`Publish(ctx, msg)` on a publisher bound to topic `hello` [client.go:78-94]:

1. Envelope → `kgo.Record`: `Topic` = the publisher's bound destination, `Value` = Payload,
   `Headers` = the envelope headers (nil when empty), `Key` only when non-empty [client.go:79-86].
   ⚠ `msg.Timestamp` is **not** mapped — the broker stamps the record.
2. OTel propagator injects W3C trace context into record headers via `recordCarrier`
   [client.go:87] (no-op without starter-otel); if `traffic.IsLoadTest(ctx)`, the load-test
   marker rides a record header so the consumer recognises synthetic load [client.go:90-92].
4. `GuardedProduceSync(ctx, p.cl, rec).FirstErr()` — synchronous produce routed through the
   same resilience executor the raw client API uses (a no-op pass-through when no govern rule
   matches this client's resource label) [client.go:93-99], broker ack / rejection surfaced to
   the caller.
5. Inside the client the hooks fire: kotel produce span + metric; observeHook
   `OnProduceRecordBuffered` opens an access-log record named `publish` and
   `OnProduceRecordUnbuffered` ends it with the outcome and buffered→unbuffered duration
   [command.go:57-69]. Access log tag: `kafka.access` (`RegisterAppTag("kafka","access")` in observe.go).

`Subscribe(ctx, handler)` on a subscriber bound to source `hello` [client.go:109-144]:

1. `messaging.Recover` converts handler panics into the error path [client.go:112].
2. One background goroutine polls `cl.PollFetches` on a context derived from the Subscribe ctx
   via `context.WithoutCancel` + a cancel func held for Close [client.go:113].
3. Poll errors are logged (tag `log.TagAppDef`), `context.Canceled` suppressed [client.go:121-127].
4. Per record, filtered by `rec.Topic != s.topic` when a source was given — a source matching
   none of the configured topics silently filters everything [client.go:129-131].
5. Trace context extracted from record headers; load-test marker restored onto the message ctx
   [client.go:132-136].
6. Handler invoked with `fromRecord(rec)`: Key/Payload/Headers/Timestamp all survive the mapping
   [client.go:172-186]. ⚠ a handler error is **only logged** — no nack/redelivery in this driver
   (franz-go group consumption commits regardless; design suspect, §6).
7. `Close` cancels the loop and waits for `done`, idempotent via `sync.Once` [client.go:146-156].

Two driver traps inherited from the franz-go construction constraint [client.go:43-51 comment]:
`NewSubscriber(ctx, source, group)` **silently drops the `group` argument** (use the client's
`group` config, client.go:68), and one client is a single consumer — one client bean per logical consumer.

### 2.4 GuardedProduceSync — the resilience seam, exact semantics

franz-go's async `Produce` returns immediately, so only the synchronous path can be wrapped
[command.go:21-22,126-137 comments]. `GuardedProduceSync(ctx, cl, recs...)` [command.go:138-154]:

- guard resolves the executor attached to `cl`; without one it runs inline — identical to
  `cl.ProduceSync`. (An executor is always attached now; with governance off or no rule
  matching the label it is a transparent no-op, so the two paths are equivalent.)
- With a rule matching, the call passes through the assembled executor
  `fault.WrapExecutor(resilience.ExecutorFor("kafka", resource))` [command.go:100] — runtime
  fault injection and resilience outcome metrics around the produce; resource label
  `kafka:<brokers>` (format `prefix:name`, [resilience/config.go:151-158]).
- On rejection (rate-limit / open breaker) the produce is **never invoked**; the rejection error
  is encoded as a per-record error so `.FirstErr()` surfaces it like a produce failure
  [command.go:145-151]. example-cloudnative asserts bursts get `resilience.ErrRateLimited`.
- What is **not** guarded: raw `ProduceSync`/`Produce` called directly on the client bean and
  the entire consume/poll path (passive). The driver's publish **is** guarded (§2.3 step 4).
  To make a client effectively ungoverned, give its resource label (`kafka:<brokers>`) a
  govern rule with every knob at zero — a Rule replaces the default wholesale, so an all-zero
  rule is a pass-through.

---

## 3. Per-key behavior reference

All keys live under `spring.kafka.instances.<name>.*` — ctor-arg binding via `conf.BindEach` (real
per-instance prefix binding). `value:` tags reconciled against source: 20 keys total.

### 3.1 Core

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `brokers` | string | — | **Required** (`expr:"$ != ''"` [config.go:30]); CSV of seed brokers; also becomes the resilience resource label `kafka:<brokers>`. | Empty → boot error; a wrong-but-reachable host fails the 10s startup Ping. |
| `topic` | string | "" | Passed as `kgo.ConsumeTopics` — consumer topics fixed at construction; the driver subscriber filters by it. Empty = produce-only client. | Produce works, consume never delivers (no topic subscribed). |
| `group` | string | "" | Passed as `kgo.ConsumerGroup`; group semantics are Kafka's own (offsets, rebalancing — see kafka.apache.org). ⚠ the driver's `NewSubscriber` group arg is dead — this key is the only group switch. | Empty + topic set = ungrouped (random-group / eager) consumption; offsets not committed. |

The `driver` key names the Driver bean for this entry: unset → assembly is owned by the
optional Driver bean injected by type (see §2.1) or the bundled `DefaultDriver`; set → that
bean by name, and naming a missing bean fails startup. (`driver` in a conf below refers to the
governance-resilience `govern.driver` selecting a rule source, not this starter.)

### 3.2 SASL

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `sasl.enabled` | bool | false | Gates mechanism assembly [driver.go:83-89]. | Enabled without credentials → Ping auth failure at boot. |
| `sasl.mechanism` | string | `plain` | `plain` / `scram-sha-256` / `scram-sha-512`, case-insensitive [driver.go:107-118]. | Any other value → CreateClient error at boot. |
| `sasl.username` / `sasl.password` | string | "" | Passed to the mechanism. | Wrong → 10s Ping failure at boot. |

### 3.3 TLS

Shared `tlsconf` block — property names uniform across starters [config.go:43-47].

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `tls.enabled` | bool | false | `c.TLS.BuildClient()` → `kgo.DialTLSConfig` [driver.go:90-97]. | — |
| `tls.cert-file` / `tls.key-file` | string | "" | mTLS client cert pair. | Half a pair → `tls.Build` boot error. |
| `tls.ca-file` | string | "" | CA to verify the broker. | Missing against a private CA → Ping TLS failure. |
| `tls.server-name` | string | "" | SNI/verification name. | Mismatch → verification failure. |
| `tls.insecure-skip-verify` | bool | false | Skips verification. | true in prod = silent MITM exposure. |

### 3.4 Producer

Zero values keep franz-go defaults [config.go:79-80].

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `producer.compression` | string | "" | `none`/`gzip`/`snappy`/`lz4`/`zstd`, case-insensitive [driver.go:153-167]. | Other value → CreateClient error at boot. |
| `producer.required-acks` | string | `all` | `all`→AllISRAcks; `leader`/`none` also **disable idempotent writes** (protocol requirement) [driver.go:132-141]. | Other value → boot error; weakening acks silently drops idempotence. |
| `producer.max-batch-bytes` | int32 | 0 | `kgo.ProducerBatchMaxBytes` when >0. | Below broker's message max → produce errors per record. |
| `producer.linger` | duration | 0s | `kgo.ProducerLinger` when >0; throughput/latency tradeoff. | — |

---

## 4. Verification & fault drills

### 4.1 Health (app-side indicator)

The starter registers no indicator; the app exports one (as in §1 / example-cloudnative
[example.go:76-97]):

```bash
curl -s :9370/readyz | jq .          # component "kafka:a", readiness+startup groups, critical
docker stop starter-kafka            # Ping probe fails
curl -s :9370/readyz                 # 503 OUT_OF_SERVICE
```

Placement in readiness/startup (never liveness — a broker outage must not restart the pod) is the app's call.

### 4.2 Round-trip + which fields survive the driver mapping

```bash
go run .    # §1 service: publish Key=k1 Payload=value Header origin=demo, consume prints it
grep kafka.access app.log | tail -2   # publish record (with duration) + consume record
```

Surviving fields: Key, Payload, Headers, and the broker-stamped Timestamp on consume
[client.go:172-186]. Dropped: `msg.Timestamp` on publish; a `Headers` entry colliding with the
W3C trace keys or the load-test header is overwritten by the inject step [client.go:87-92].

### 4.3 Guarded vs unguarded produce

```bash
# example-cloudnative with govern.default.rate-limit=8:
go run .    # prints "resilience: N produce admitted, M rejected with ErrRateLimited"
```

The same burst via the **driver** publisher is limited identically — it rides the same executor
(§2.3 step 4); only a direct raw `ProduceSync` on the client bean bypasses it. Flip `govern.*` in the watched source to change policy without
restart (governance center hot-reload).

### 4.4 Governance label check

The resource label is `kafka:<brokers>` — exactly the `brokers` string, not a per-topic or
per-client-name label [starter.go:83]. Two client beans sharing one broker list share one
limiter/breaker; verify via the resilience outcome counters
(`curl -s :9370/metrics | grep resilience`) while hammering `GuardedProduceSync`.

### 4.5 Traces / metrics / log tags

- kotel spans + `messaging.*` metrics ride starter-otel's globals; naming follows the kotel
  plugin (see [kotel](https://github.com/twmb/franz-go/tree/main/plugin/kotel)). example-otel
  ships OTLP gRPC → Jaeger on :4317; check the UI for linked produce/consume spans (trace
  context rides record headers, §2.3 steps 2/5).
- Access log tag `kafka.access` (rendered `_app_kafka_access`): op names `publish`
  (duration-carrying) and `consume` (duration-less — no paired start hook
  [command.go:40-42,71-75]).
- franz-go client-internal logs (reconnects, request failures) under `log.TagAppDef`,
  bridged at Info threshold [driver.go:177-192].

### 4.6 Broker-down fail-fast

```bash
docker compose down && go run .   # boot fails within ~10s: "failed to ping kafka: <brokers>"
```

Mid-run broker loss: poll errors in the log (tag `log.TagAppDef`) and per-call produce errors;
franz-go reconnects automatically (its own semantics).

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "failed to ping kafka" | Unreachable brokers / wrong SASL / TLS mismatch | Fix connectivity or credentials; the 10s probe is unconditional. |
| Boot fails "unsupported kafka sasl mechanism / required-acks / compression" | Typo in an enum key | Exact-match enums (case-insensitive); correct the value. |
| Driver consumer never receives | `NewSubscriber` source ≠ configured `topic`, or `topic` empty | Source must equal the client's `topic`; silent filter otherwise. |
| Driver consumer group "ignored" | `NewSubscriber` group arg is dead | Set `spring.kafka.instances.<name>.group` (fixed at construction). |
| No rate limit despite govern.* on | Calling raw `ProduceSync` on the client bean | Only `GuardedProduceSync` and the driver publisher are guarded. |
| No traces/metrics | starter-otel not imported | kotel rides the OTel globals; import starter-otel. |
| No access log lines | log tag filtered | Check the `kafka.access` tag filter. |
| Handler errors vanish after a log line | By design: no nack/redelivery in this driver | Build retry/redelivery in the handler or use retry.go from messaging. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 17 (3 core + 4 sasl + 6 tls + 4 producer) |
| Required | 1 (`brokers`) |
| Quickstart external deps | 1 (Kafka broker) |
| "Watch out" entries | 6 |

Design suspects (audit ledger; carried over from the previous edition, none fixed since):

- driver drops `NewSubscriber`'s `group` arg and silently filters on `source` mismatch [client.go:68,129-131] — violates fail-fast.
- consume handler errors only logged, no nack/redelivery [client.go:137-139] — contradicts the Recover comment's framing [client.go:110-112].
- ~~`destroyClient` discards the Flush error~~ fixed: Flush failure now logs an ERROR naming
  the data-loss consequence and propagates out of the destroy hook.
- no health indicator from the starter (family asymmetry: go-redis/redigo register one).
- resilience executors keyed by `*kgo.Client` pointer in package-level sync.Maps [command.go:81-85] — a custom Driver returning a wrapped client would silently skip the guard.

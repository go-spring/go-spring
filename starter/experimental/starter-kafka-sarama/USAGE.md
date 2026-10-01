# starter-kafka-sarama Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `driver.go`,
`logger.go`, `scram.go`) and the runnable [example/](example/) / [example-otel/](example-otel/) —
file:line spot-checks in brackets below. **Kafka protocol and sarama API semantics are
[sarama's own documentation](https://github.com/IBM/sarama) and
[kafka.apache.org](https://kafka.apache.org/documentation/)** — everything below is go-spring's
increment.
**Activation**: any `spring.kafka-sarama.instances.*` property (the module registers under
`gs.OnProperty("spring.kafka-sarama")`, a prefix check [starter.go:38]). Each
`spring.kafka-sarama.instances.<name>` entry creates exactly one `sarama.Client` bean named `<name>`
[starter.go:39-43]. The prefix is deliberately distinct from the franz-go
[starter-kafka](../starter-kafka) (`spring.kafka`) — the two are never imported together
[config.go:26-28].

**There is no messaging.Driver here** (unlike starter-kafka): publish/consume is native sarama
on top of the shared client bean. The starter only DECLARES each operation's identity at the
publish/consume seams; the signals themselves are EMITTED by the resilience layer. This is the
#1 design suspect — see §6.
---

## 1. Complete worked project

One service with a producer and a partition consumer on topic `hello`, plus tracing, access
log and governance-wrapped sends. Files: `go.mod`, `main.go`, `service.go`,
`conf/app.properties`.
**go.mod** (deps that matter):

```
require (
    github.com/IBM/sarama       latest
    go-spring.org/spring        v1.3.x
    go-spring.org/starter-kafka-sarama latest
    go-spring.org/starter-otel    latest   // optional: real trace/metric export
    go-spring.org/starter-governance-file latest // optional: resilience/fault policy
    go-spring.org/starter-actuator latest  // optional: probes + /metrics mount
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-governance-file"
    _ "go-spring.org/starter-kafka-sarama"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — the entire Kafka surface of the app:

```go
package service

import (
    "context"

    "github.com/IBM/sarama"
    "go-spring.org/cloud/traffic"
    "go-spring.org/spring/gs"
    StarterKafkaSarama "go-spring.org/starter-kafka-sarama"
)

type Service struct {
    // Always the raw sarama.Client bean. Producers/consumers are derived per use
    // via sarama's *FromClient constructors — one pool/metadata cache serves every
    // role (DESIGN.md §2).
    Client sarama.Client `autowire:"main"`
    // The process's load-test convention, injected when the application provides a
    // traffic.Propagator bean; nil → the seams use the default convention.
    Prop traffic.Propagator `autowire:"?"`
}

func init() {
    gs.Provide(func(s *Service) gs.Runner {
        return func(ctx context.Context) error {
            return s.publish(ctx, "hello", "value")
        }
    })
}

// publish sends one record with trace context in the headers and the send
// declared and routed through the governance executor (breaker/rate-limit/retry),
// which emits the span, metrics and access log.
func (s *Service) publish(ctx context.Context, topic, value string) error {
    producer, err := sarama.NewSyncProducerFromClient(s.Client)
    if err != nil {
        return err
    }
    defer producer.Close() // derived beans close BEFORE the client's Destroy
    // Wrap to declare the publish and route it through the executor; the wrapper
    // injects the W3C trace context + load-test marker either way, so wrapping is
    // always safe (command.go):
    producer = StarterKafkaSarama.WrapSyncProducer(s.Client, producer, s.Prop)

    msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.StringEncoder(value)}
    _, _, err = producer.SendMessage(msg)
    return err
}
```

**conf/app.properties** — the complete, commented surface used above:

```properties
# --- kafka client -----------------------------------------------------------
spring.kafka-sarama.instances.main.brokers=127.0.0.1:9092
# Must match the target cluster for consumer groups etc. to behave (sarama
# negotiates protocol features from this); empty = sarama's own default.
spring.kafka-sarama.instances.main.version=3.7.0

# --- producer tuning --------------------------------------------------------
spring.kafka-sarama.instances.main.producer.compression=snappy
spring.kafka-sarama.instances.main.producer.required-acks=all

# --- optional: SASL + TLS (dev broker is plaintext) -------------------------
#spring.kafka-sarama.instances.main.sasl.enabled=true
#spring.kafka-sarama.instances.main.sasl.mechanism=scram-sha-512
#spring.kafka-sarama.instances.main.sasl.username=user
#spring.kafka-sarama.instances.main.sasl.password=pass
#spring.kafka-sarama.instances.main.tls.enabled=true
#spring.kafka-sarama.instances.main.tls.ca-file=/path/ca.pem
#spring.kafka-sarama.instances.main.tls.cert-file=/path/client.pem
#spring.kafka-sarama.instances.main.tls.key-file=/path/client.key

# --- observability (starter-otel) -------------------------------------------
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- actuator ---------------------------------------------------------------
spring.actuator.addr=:9370
```

**Start Kafka** (KRaft single node, same as [example/docker-compose.yml](example/docker-compose.yml)):

```bash
cd demo && docker compose up -d   # bitnami/kafka:3.7, PLAINTEXT on 127.0.0.1:9092, auto-create topics
```

**Verify** (smoke-isomorphic with [example/check.sh](example/check.sh)):

```bash
go run .                                         # boot FAILS FAST if metadata fetch yields no brokers
grep 'kafka sarama client initialized' app.log   # [client.go:70]
grep _app_kafka_access app.log | tail -1         # one record per publish/consume observation
curl -s :9090/metrics | grep messaging_client    # operation + attempt duration histograms + in-flight
```
---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-kafka-sarama
  ├─ init: sarama.Logger = go-spring log bridge   [starter.go:33] (all sarama events → Info, tag _app)
  └─ gs.Module(OnProperty("spring.kafka-sarama")) fires on any spring.kafka-sarama.instances.* key
        └─ conf.BindEach → one Config per <name> entry
              └─ Provide(newClient, IndexArg name, IndexArg config, IndexArg 3,?Driver).Name(<name>)
                   .Destroy(destroyClient)        [starter.go:40-44]

gs.Run()
  ├─ newClient [client.go:70]:
  │    1. optional Driver bean — none → bundled DefaultDriver  [client.go:73-75]
  │    2. d.CreateClient: sarama.NewConfig + version/SASL/TLS/producer opts,
  │       then sarama.NewClient(brokers) — DIALS seed brokers and fetches
  │       metadata, so bad brokers/credentials/TLS fail HERE, not on first use
  │       [client.go:53-69 comment, driver.go:71-112]
  │    3. attachExecutor (inside the driver, the moment the client is built):
  │       params.ExecutorFor("kafka", "kafka:<brokers>") — the governed executor
  │       (fault-wrapped) when the container is present, the observed-only
  │       resilience.Unmanaged one (with a one-time warning) when it is not —
  │       indexed by client in a sync.Map. Governance is applied IN THE
  │       CONSTRUCTOR, so the driver returns the client COMPLETE and there is
  │       no post-hoc patch step [driver.go CreateClient, command.go attachExecutor]
  │    4. then defensive fail-fast: len(Brokers())==0 → closeResilience +
  │       close + boot error                                            [client.go:86-89]
  ├─ derived beans are YOURS: sarama.New*FromClient wherever you inject the client
  └─ SIGTERM → Destroy: closeResilience (exec.Close, forget maps) → cl.Close()
       [client.go:96-101, command.go closeResilience]
```

**Assembly extension point**: client assembly is owned by a `Driver` (interface, `driver.go:44-66`).
A company/umbrella starter may provide its own `Driver` as an **optional container bean** (a
`gs.Provide(func() StarterKafkaSarama.Driver{...})`, so it can inject config bound from the
properties file at wiring time); every instance under `spring.kafka-sarama` is then built through
it. `CreateClient` receives the container's `cloud.ClientParams` bundle and returns the client
complete — it must attach the executor it builds from `params` the same way the bundled driver does
(see `attachExecutor`). When no such bean exists the starter falls back to the bundled
`DefaultDriver` (`driver.go:67-112`) inside assembly (`client.go:73-75`). When several Driver beans
coexist, an entry selects one by name:
`spring.kafka-sarama.instances.<name>.driver = <bean-name>` (empty = inject the
single Driver bean by type; naming a missing bean fails startup).

Derived producers/consumers are not container beans — close them yourself before the app
shuts down (`defer producer.Close()` in the publish path is the intended pattern, see
[example/example.go:72-76]). `sarama.Client.Close` releases the shared broker connections.
### 2.2 The declare/guard mechanism — exact order and what is NOT guarded

For a producer, `WrapSyncProducer(cl, p, prop)` resolves the executor stashed for `cl` and
returns a wrapper that DECLARES each send and drives it through that executor. The wrapper is
always applied (also when governance is off) because it is also where the W3C trace context
and the load-test marker are injected — propagation is not emission and must happen either
way; with no executor the call still runs, just inline with no span. Declared-and-guarded
surface:

```
SendMessage / SendMessages
  → declare: observability.WithOperation(ctx, operation("publish", msg.Topic))
  → fault.WrapClientExecutor (injected faults when spring.governance.client.fault enabled)
    → resilience executor wrapper (span + outcome counter + call/attempt duration + access log)
      → resilience executor (breaker / rate limit / retry, policy from governance center)
        → inject W3C trace context + load-test marker into msg.Headers
          → inner p.SendMessage (real sarama)
```

**Not guarded**: `Close` and the whole transaction family
(`BeginTxn`/`CommitTxn`/`AbortTxn`/`AddOffsetsToTxn`/`AddMessageToTxn`/`TxnStatus`/
`IsTransactional`) delegate directly — they are control-plane, not the protected data path.
Also not declared: `sarama.AsyncProducer` (no wrapper exists), and any producer you forget to
wrap. `SendMessage` takes no `context.Context` (sarama's API is context-free), so the
declaration is built from the message alone and starts from `context.Background()` — the
publish span is a NEW ROOT, and a per-call deadline is expressed via resilience
`AttemptTimeout`/`MaxDuration`. Consumption is declared separately through `Consume` (§2.4).
### 2.3 One publish, layer by layer

`publish(ctx, "hello", "value")` on a wrapped producer with tracing live:

1. `SendMessage` declares the operation `publish` on topic `hello`
   (`observability.WithOperation`) and enters the executor — the starter emits nothing
   itself (observe.go). Attribute namespace: `messaging.system=kafka`,
   `messaging.operation=publish`, `messaging.destination.name=<topic>`.
2. The resilience layer opens the publish span and, at the call level, records
   `messaging.client.operation.duration` (retries+backoff included) plus
   `messaging.client.active_requests`; at the attempt level it records
   `messaging.client.attempt.duration`. It writes one access log on the `kafka` access tag.
   Publish and consume are declared `NonIdempotent`, so a retry policy on the label is
   **suppressed** (one warning per service): re-sending or re-running the handler is a second
   side effect, not a second attempt.
3. Inside the attempt, the W3C propagator injects `traceparent`/`tracestate` into
   `msg.Headers`. ⚠ Injection **drops any pre-existing header with the same key** so
   re-injection stays idempotent — do not stash data under `traceparent`.
4. `prop.Inject` stamps the marker header too (a no-op outside load-test traffic)
   — the consumer side re-derives the flag from it (§2.4).
5. sarama sends and (with the starter-forced `Producer.Return.Successes=true`
   [driver.go:72]) returns partition/offset after the broker ack (`required-acks=all` →
   WaitForAll [driver.go:131]).
6. The executor closes the span/log/metric from the outcome it saw. The publish span is a
   new root (SendMessage carries no ctx); the `traceparent` injected into the headers is
   what links the producer to the consumer.
### 2.4 One consume, layer by layer

Receiving a `*sarama.ConsumerMessage` (partition consumer here; the same seam fits a
ConsumerGroup handler):

1. `Consume(ctx, cl, msg, prop, handle)` **extracts** the upstream trace context from
   `msg.Headers` — the consume span is a child of the producer's, provided both sides use
   the seams. Extraction never mutates the message (`consumerCarrier.Set` is a no-op).
2. The load-test marker header is read back into the ctx (`prop.Extract`) —
   downstream business code and clients branch on it without any HTTP header.
3. The operation `consume` on the topic is declared and `handle` runs under the executor,
   which emits the observation (same metric/log namespace as publish; the consume span's
   parent comes from the message header).
4. Your handler runs and commits offsets. Offset commits are entirely
   your/consumer-group's concern — the starter never touches them.

---

## 3. Per-key behavior reference

All keys under `spring.kafka-sarama.instances.<name>.` (15 total incl. the tls/sasl
groups; binding is per-instance prefix binding via `conf.BindEach`, NOT absolute-property
field injection).

### 3.1 Core

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `brokers` | string | — | **Required** (`expr:"$ != ''"` [config.go:32]); comma-separated seed list [driver.go:103]. Also becomes the governance service label `kafka:<brokers>` verbatim [command.go:221] — different orderings/spellings of the same cluster are DIFFERENT labels. | Missing/empty → bind error at boot. Typo'd broker → sarama.NewClient fails at boot (fail-fast). |
| `version` | string | "" (sarama default) | Parsed with `sarama.ParseKafkaVersion`; gates protocol features (headers, SASL mechanisms, consumer groups) [driver.go:65-71]. | Unparseable → boot error `invalid kafka version`. Too low → feature errors at first use. |

The `driver` key names the Driver bean for this entry: unset → assembly is owned by the
optional Driver bean injected by type (see §2.1) or the bundled `DefaultDriver`; set → that
bean by name, and naming a missing bean fails startup.

### 3.2 SASL (`sasl.*`) — [config.go:64-77], [driver.go:101-118]

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `sasl.enabled` | bool | false | Gates the whole block; false ignores the other three keys. | Left off against a SASL broker → boot fails at metadata fetch. |
| `sasl.mechanism` | string | `plain` | `plain` / `scram-sha-256` / `scram-sha-512` (case-insensitive); SCRAM wires an xdg-go/scram client generator per handshake [scram.go:56-63]. ⚠ SCRAM needs a `version` high enough — check [sarama docs](https://github.com/IBM/sarama). | Any other value → boot error `unsupported kafka sasl mechanism` (no silent PLAIN fallback [driver.go:99-100]). |
| `sasl.username` / `sasl.password` | string | "" / "" | Copied into `cfg.Net.SASL` [driver.go:103-104]. | Wrong → SASL handshake fails at boot (fail-fast). |

### 3.3 TLS (`tls.*`) — shared `cloud/security` block [config.go:42-46]

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `tls.enabled` | bool | false | On: `c.TLS.BuildClient()` → `cfg.Net.TLS` [driver.go:81-89]. | Off against a TLS listener → handshake fails at boot. |
| `tls.cert-file` / `tls.key-file` | string | "" | Client cert pair (mTLS). ⚠ Both or neither. | Half a pair → `tls.Build` error at boot. |
| `tls.ca-file` | string | "" | CA to verify the broker. | Missing against a private CA → verify error at boot. |
| `tls.server-name` | string | "" | SNI/verification name. | Mismatch → verify error at boot. |
| `tls.insecure-skip-verify` | bool | false | Skips broker cert verification. | true in prod → silent MITM exposure. |

### 3.4 Producer

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `producer.compression` | string | "" (sarama: none) | `none`/`gzip`/`snappy`/`lz4`/`zstd`, case-insensitive [driver.go:143-158]. | Other value → boot error `unsupported kafka compression`. |
| `producer.required-acks` | string | `all` | `all`→WaitForAll, `leader`→WaitForLocal, `none`→NoResponse [driver.go:129-138]. | Other value → boot error `unsupported kafka required-acks`. `none` loses messages silently on leader fail. |

⚠ Forced-without-keys: the starter always sets `Producer.Return.Successes=true` and
`Consumer.Offsets.Initial=OffsetOldest` [driver.go:72-73] — there is no config key to change
either (design suspect, §6). New consumer groups therefore start at the **oldest** offset.

---

## 4. Verification & fault drills

### 4.1 Boot fail-fast (broker down)

```bash
docker compose stop kafka
go run .    # exits non-zero: sarama.NewClient cannot fetch metadata
grep 'no brokers after metadata fetch\|failed to create kafka client' app.log   # [client.go:57-63]
docker compose start kafka
```

The process never reaches "serving" with a dead/unauthenticated broker — no first-produce
surprise [client.go:42-46].
### 4.2 Guarded vs unguarded paths

With starter-governance-file and a breaker/rate-limit policy on service `kafka|127.0.0.1:9092`:

```go
wrapped := StarterKafkaSarama.WrapSyncProducer(cl, producer, prop)
wrapped.SendMessage(msg)   // rejections visible in _app_kafka_access + resilience counters
producer.SendMessage(msg)  // RAW handle still unguarded — the wrapper does not patch p
```

Drill: hammer past the rate limit → wrapped calls return the resilience error
(partition/offset `-1/-1`); raw calls keep going. Consumption declared through
`Consume` is governed too (same `kafka:<brokers>` label) — a rate-limited consume returns
the resilience sentinel from `Consume` and your handler never runs.
### 4.3 Message round-trip incl. header survival

Publish then consume on the same topic (partition consumer from oldest, as in
[example/example.go:89-112]):

- `msg.Value` survives verbatim (`sarama.StringEncoder` → `string(msg.Value)`).
- `traceparent` header injected by `WrapSyncProducer` is readable on the consumer side —
  Jaeger shows `publish` → `consume` as one trace (the [example-otel](example-otel/)
  smoke asserts exactly this against the Jaeger API).
- Load-test marker: publish under a marked ctx (e.g. echo's loadtest middleware upstream),
  then in the consumer the propagator's `IsLoadTest(ctx)` is true after `Consume` has
  extracted it.
- ⚠ A pre-set `traceparent`/marker header on the producer message is REPLACED, not merged.
  There is no message-Key mapping anywhere (no driver) — `msg.Key` is whatever you set.
### 4.4 Observables read-out

```bash
grep _app_kafka_access app.log | tail      # messaging.system=kafka messaging.operation=publish|consume messaging.destination.name=... status/duration_ms
curl -s :9090/metrics | grep messaging_client   # operation + attempt duration histograms + in-flight, attrs messaging.system=kafka
# spans: "publish"/"consume", attr messaging.destination.name=<topic>
```

Sarama's own connection events arrive as Info lines prefixed `kafka:` on the default app
tag [logger.go:39-51] — broker connects, metadata refresh, reconnects.
### 4.5 Governance label check

The executor label is literally `kafka|` + the `brokers` string [command.go:221]. A policy must
target that exact spelling; confirm with:

```bash
grep 'resilience' app.log | grep 'kafka|127.0.0.1:9092'
```

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails `failed to create kafka client` | Brokers unreachable / SASL/TLS mismatch | Fail-fast by design [client.go:56-59]; fix connectivity/credentials; check the bridged `kafka:` sarama lines just above. |
| Boot fails `kafka client has no brokers after metadata fetch` | Broker list resolves but cluster metadata empty | Defensive check [client.go:60-64]; check advertised listeners on the broker. |
| Boot fails `invalid kafka version` / `unsupported kafka ... mechanism/compression/required-acks` | Typo in `version`/`sasl.mechanism`/`producer.compression`/`producer.required-acks` | Values are exact-match enums [driver.go:66,115,137,156]. |
| No traces / no `_app_kafka_access` | starter-otel not imported, or the publish/consume not declared | Declaration is opt-in at the seam — wrap with `WrapSyncProducer` / run through `Consume`; import starter-otel (otherwise OTel globals are no-ops). |
| Governance policy never fires | Service label mismatch, or producer not wrapped | Label is `kafka:<brokers>` verbatim [command.go:221]; wrap via `WrapSyncProducer` — the raw handle stays unguarded. |
| Consumer re-reads the whole topic | Starter forces `Consumer.Offsets.Initial=OffsetOldest` [driver.go:73] | Commit offsets properly in your consumer-group handler; no config key exists to change this default. |
| `SyncProducer` returns `ErrOutOfBrokers` at runtime | Broker restarted and version/credentials changed post-boot | sarama reconnects automatically; if credentials changed, restart the app (client config is boot-time). |
| Breaker trips but consumer keeps failing | Consume declared through `Consume`, so rejections surface there | The executor returns the resilience sentinel before your handler runs; check for `resilience.IsRejection(err)` and (for a partition consumer) do not commit the offset so the message is redelivered. |
| Duplicate-header confusion on consumer | Producer-side injection drops same-key headers | Don't use `traceparent`/load-test keys for app data [command.go:140-149]. |

---

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 14 (core 2 + sasl 4 + tls 6 + producer 2) |
| Required | 1 (`brokers`) |
| Quickstart external deps | 1 (Kafka; a collector for full observability) |
| "Watch out" entries | 6 |

Design suspects (audit ledger; none fixed since the last pass):

- **No messaging.Driver** — the only MQ starter in the family without one; publication
  declaration is manual at the seam, and the "driver Key mapping" concern is structurally
  absent (there is no mapping layer to drop a Key).
- `WrapSyncProducer`/`SendMessage` declare from the message and start at
  `context.Background()` — the publish span is a new root; per-call deadlines only via
  resilience `AttemptTimeout`/`MaxDuration`.
- No consumer-group/subscribe seam → the consume side is a per-message `Consume` call the
  caller wires by hand; nothing declares a subscription for you.
- `Return.Successes`/`OffsetOldest` silently override sarama defaults with no config keys
  [driver.go:72-73].
- Governance service label uses the raw `brokers` string — same cluster spelled differently
  yields distinct breaker scopes [command.go:221].
- Resilience executor index is keyed by the `sarama.Client` interface value in a `sync.Map`
  — safe for one-client-per-name, but a wrapper Driver returning per-call wrapper objects
  would break `WrapSyncProducer`/`Consume`'s lookup.
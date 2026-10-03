# starter-rocketmq Usage — Reference

Detailed usage reference. Overview: [README.md](README.md). All behavior claims are verified
against the starter source (`starter.go`, `config.go`, `client.go`, `command.go`, `messaging.go`,
`observe.go`, `driver.go`) and the runnable [example/](example/) / [example-otel/](example-otel/) — file:line
spot-checks in brackets below. **RocketMQ semantics (topics, consumer groups, tags, retry,
clustering vs broadcasting) are [rocketmq-client-go's documentation](https://github.com/apache/rocketmq-client-go)
and [RocketMQ's own docs](https://rocketmq.apache.org/docs/)** — everything below is go-spring's
increment. The client library is `github.com/apache/rocketmq-client-go/v2` (the remoting client,
not the 5.x gRPC `rocketmq-clients`).

**Activation**: any `spring.rocketmq.instances.*` key (the module registers `OnProperty("spring.rocketmq")`,
a prefix check [starter.go:40]). Each `spring.rocketmq.instances.<name>` entry creates one
`*StarterRocketmq.Client` bean named `<name>`.

---

## 1. Complete worked project

A service that produces and consumes through the driver (broker-neutral envelopes), with raw-SDK
escape hatch, actuator and OTel. File tree:

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
    github.com/apache/rocketmq-client-go/v2 v2.1.2
    go-spring.org/spring             v1.3.x
    go-spring.org/starter-rocketmq   latest
    go-spring.org/starter-actuator   latest   // optional: /metrics mount
    go-spring.org/starter-otel       latest   // optional: real trace/metric export
)
```

**main.go**:

```go
package main

import (
    "go-spring.org/spring/gs"
    _ "go-spring.org/starter-actuator"
    _ "go-spring.org/starter-otel"
    _ "demo/service"
)

func main() { gs.Run() }
```

**service.go** — driver path for the business messages, guarded raw path for one critical send:

```go
package service

import (
    "context"

    "github.com/apache/rocketmq-client-go/v2/primitive"
    "go-spring.org/cloud/messaging"
    "go-spring.org/log"
    "go-spring.org/spring/gs"
    StarterRocketmq "go-spring.org/starter-rocketmq"
)

type Service struct {
    // The wrapper bean, named after the config entry (spring.rocketmq.instances.a).
    Client *StarterRocketmq.Client `autowire:"a"`
}

func init() {
    gs.Provide(func(s *Service) (gs.Runner, error) {
        driver := StarterRocketmq.NewDriver(s.Client, nil)

        sub, err := driver.NewSubscriber(context.Background(), "orders", "order-workers")
        if err != nil {
            return nil, err
        }
        if err := sub.Subscribe(context.Background(), func(ctx context.Context, msg *messaging.Message) error {
            log.Info(ctx, "order received", log.String("key", msg.Key))
            return nil // nil = ConsumeSuccess; error = ConsumeRetryLater (broker redelivers)
        }); err != nil {
            return nil, err
        }

        return func(ctx context.Context) {
            pub, err := driver.NewPublisher(ctx, "orders")
            if err != nil {
                log.Error(ctx, "publisher", log.Any("error", err))
                return
            }
            _ = pub.Publish(ctx, &messaging.Message{
                Key:     "order-42",
                Payload: []byte(`{"id":42}`),
                Headers: map[string]string{"from": "demo"},
            })
            _ = pub.Close()

            // Critical send: routed through GuardedSend like the driver path (§2.2).
            p, _ := s.Client.NewProducer()
            msg := primitive.NewMessage("orders", []byte("urgent"))
            _, err = StarterRocketmq.GuardedSend(ctx, s.Client, p, msg)
            log.Info(ctx, "guarded send done", log.Any("error", err))
        }, nil
    })
}
```

**conf/app.properties**:

```properties
# --- rocketmq --------------------------------------------------------------
spring.rocketmq.instances.a.name-servers=127.0.0.1:9876
spring.rocketmq.instances.a.send-timeout=5s
spring.rocketmq.instances.a.ping=true          # TCP probe of the name server at boot
# spring.rocketmq.instances.a.access-key=...         # ACL: must pair with secret-key
# spring.rocketmq.instances.a.secret-key=...

# --- actuator + otel -------------------------------------------------------
spring.actuator.addr=:9370
spring.observability.service-name=demo
spring.observability.trace.exporter=otlp-grpc
spring.observability.trace.endpoint=127.0.0.1:4317
spring.observability.trace.insecure=true
spring.observability.metrics.exporter=prometheus
spring.observability.metrics.port=9090

# --- governance (rate limit / breaker for rocketmq:127.0.0.1:9876) ---------
spring.governance.source.file.path=conf/governance.yaml
```

**Start RocketMQ** (copy [example/docker-compose.yml](example/docker-compose.yml) +
[example/broker.conf](example/broker.conf); the `brokerIP1=127.0.0.1` + `listenPort=10911`
advertise pair and `autoCreateTopicEnable=true` matter for host-side clients):

```bash
docker compose -p gs-rocketmq-demo up -d        # namesrv :9876 + broker :10911
docker exec rmqbroker sh mqadmin updateTopic -n namesrv:9876 -c DefaultCluster -t orders
go run .
```

⚠ A push consumer's Subscribe queries topic route info eagerly and fails on a missing topic, and
`autoCreateTopicEnable` only kicks in on the first *produce* — create the topic before the app
starts (this is why [example/check.sh](example/check.sh) gates on `mqadmin topicList` visibility).

**Verify**:

```bash
grep _app_rocketmq_access app.log | tail -2   # driver publish/consume records
curl -s :9090/metrics | grep messaging.client # declared-operation metrics (call + attempt)
curl -s :9370/healthz
```

---

## 2. Assembly & timing

### 2.1 Bean lifecycle

```
import starter-rocketmq
  └─ gs.Module(OnProperty("spring.rocketmq")) fires when any spring.rocketmq.instances.* key exists
        └─ conf.BindEach("${spring.rocketmq}") → one Config per <name> entry   [starter.go:40-48]
              └─ Provide(newClient).Name(<name>).Destroy((*Client).Close)      [starter.go:42-45]

gs.Run()
  ├─ ctor newClient [starter.go:57]:
  │    1. pair-check access-key/secret-key (one-sided → boot error)           [starter.go:60-62]
  │    2. Driver.CreateClient — the optional Driver bean (selected per entry by
  │       ${spring.rocketmq.instances.<name>.driver}: empty = `spring.rocketmq.default.driver` then by type; set = by bean name —
  │       naming a missing bean fails startup), or the bundled
  │       DefaultDriver when none is provided: installs the rlog→go-spring log
  │       bridge (process-global, exactly once via sync.Once) and delegates to
  │       NewClient, which fixes the identity (name servers + Config) AND attaches
  │       the governance executor in the same step — params.ExecutorFor("rocketmq",
  │       service) over the injected *resilience.Manager / *fault.Injector beans,
  │       bundled by newClient into cloud.ClientParams        [driver.go, client.go]
  │    3. Ping probe (opt-in): TCP dial, first reachable addr wins, 3s budget
  │       per address; runs AFTER assembly, failure → boot error (Client closed)
  ├─ app injects *Client wherever `autowire:"<name>"` appears
  ├─ app creates producers/consumers/driver at its own pace (each registered
  │  on the Client under a mutex)                                             [client.go:104-157]
  └─ SIGTERM → Client.Close: release the executor first, then every registered
     producer and consumer Shutdown; shutdown errors are logged, not returned  [client.go:162-184]
```

Notes verified in source:

- The probe is a TCP dial, **not** a broker round trip — it catches wrong addresses, not ACL or
  credential errors (probe loop [driver.go:148-155] stops at the first success; see the README's design notes).
- Producers are created *started* (`p.Start()` inside NewProducer [client.go:116]); consumers are
  returned **unstarted** — call Subscribe then Start yourself, or let the driver do it
  [client.go:130-135].
- If Close races a concurrent NewProducer/NewPushConsumer, the newcomer is shut down and
  `errClosedClient` returned [client.go:122-125]; after Close, both constructors fail fast
  [client.go:105-110].
- There is **no health.Indicator** (family-wide decision; see the README's Health section).

### 2.2 The guard — exact wrap order and what is NOT guarded

The resilience executor attached by the constructor (`NewClient`) is driven
through **one seam**, and each call DECLARES its operation before entering it —
the starter declares, the resilience layer emits:

```
GuardedSend(ctx, cl, producer, msg)                       [command.go]
  ├─ observability.WithOperation(ctx, operation("publish", msg.Topic))
  │     — declares the direction (span name), messaging.system/operation labels
  │       and the topic as Detail (never a label)
  └─ cl.execute                                            [client.go]
       ├─ executor is a pass-through (governance off) or absent (bare Client)
       │     → producer.SendSync runs inline, identical to raw
       └─ exec.Execute(ctx, injectTraceContext + send)      — fault.Injector outermost
                  (fault.WrapClientExecutor), resilience observer inside it; the
                  observer emits span + messaging.client.* metrics + access log
                  from the declaration; rejection returns a resilience sentinel
                  and the send never reaches the wire
```

The consumer direction declares the same way inside the driver's receive callback
(`operation("consume", topic)`), then runs the handler under `cl.execute`
[messaging.go].

Wrap order inside `params.ExecutorFor("rocketmq", service)` [cloud/governance/governance.go], called
from `NewClient`: `mgr.ClientExecutorFor("rocketmq", service)` — mgr being the injected
`*resilience.Manager` — returns the fully assembled executor (core breaker/limiter/retry wrapped by
the resilience observer — outcome counters + access log) → wrapped outermost by
`fault.WrapClientExecutor(..., inj)` (runtime fault injection from the injected `*fault.Injector`;
the injected error flows through the inner retry loop, so the breaker counts it). With no
`*resilience.Manager` the bundle degrades to `resilience.Unmanaged` — observed only, warned once.

- Both the driver's `Publish` (via `GuardedSend`) and its consume path share this one seam. **NOT
  guarded**: raw `SendSync` called directly on a producer, and by design
  `SendAsync`/`SendOneWay` [command.go]. The manual `StartProducerSpan`/`StartConsumerSpan`
  helpers are the app's own spans for a raw send it drives directly — a send routed through
  `GuardedSend` is already spanned by the resilience layer from the declaration, so do not wrap
  both around one send.
- Service label is `rocketmq:<name-servers>` — the comma-joined `name-servers` list, same
  convention as the kafka starters. Two config entries pointing at the same name-server cluster
  therefore share one governance service (the `name-servers` segment never reaches the label;
  see §6).
- With governance off, `mgr.ClientExecutorFor` yields a transparent pass-through and `GuardedSend`
  behaves exactly like `SendSync` [command.go, client.go] (proved by TestExecutePassThrough
  [rocketmq_test.go:47-53]).
- Rejections (rate limit / open breaker) return a resilience sentinel error and **never invoke the
  send** (proved by TestExecuteRateLimit [rocketmq_test.go:57-69]).

### 2.3 One publish, layer by layer (driver path)

`pub.Publish(ctx, &messaging.Message{Key, Payload, Headers})` [messaging.go]:

1. A `primitive.Message` is built for the publisher's fixed topic; `Key` (single string) becomes
   the message keys via `WithKeys`; each Header entry becomes a user property.
2. If the ctx carries the load-test marker, `x-loadtest=1` is added to the user properties
   [traffic.go:60].
3. `GuardedSend` declares the publish (`operation("publish", msg.Topic)`): the direction names the
   span, `messaging.system`/`messaging.operation` are the bounded labels, and the topic is
   per-call Detail — it reaches the span and the log, never a metric label.
4. `producer.SendSync(attemptCtx, msg)` — synchronous, blocks until broker ack; the W3C trace
   context is injected into the message user properties from the attempt ctx, so the traceparent
   carries the executor's span.
5. The resilience observer (the single emitter) records the outcome on
   `messaging.client.operation.duration` (call level), `messaging.client.attempt.duration`
   (per retry) and `messaging.client.active_requests`, and writes one access log
   (`_app_rocketmq_access`).

### 2.4 One consume, layer by layer (driver path)

The SDK push-consumer goroutines invoke the starter's handler [messaging.go]:

1. `messaging.Recover` was pre-wrapped at Subscribe so a handler panic becomes a normal
   error (nack/redelivery) instead of unwinding the SDK goroutine.
2. Per message: the driver extracts the upstream W3C trace from the user properties
   (`extractTraceContext`), declares the consume (`operation("consume", topic)`) onto the handler
   ctx, and runs the handler under `cl.execute` — so the resilience observer opens the consumer
   span, records the metrics and writes the access log from the declaration.
3. The `x-loadtest` property is mapped back onto the ctx, so the handler sees
   the propagator's `IsLoadTest(ctx)`.
4. `fromMessageExt` builds the envelope: `Key` from the KEYS property, `Payload` = body,
   `Headers` = **all** user properties, `Timestamp` from StoreTimestamp [messaging.go].
5. Handler error → logged at Error + `ConsumeRetryLater` (broker redelivers per RocketMQ retry
   semantics); success → `ConsumeSuccess` [messaging.go].

Known mapping drops (both directions) — all verified in messaging.go:

| Field | Publish (envelope → RocketMQ) | Consume (RocketMQ → envelope) |
|---|---|---|
| Key | single string → single message key | KEYS property read back as one string; multi-key producers get a joined value, not the original list |
| Headers | entries → user properties, verbatim | ALL properties returned, including SDK-internal ones (`traceparent`, KEYS, `x-loadtest`) — they leak into `msg.Headers` |
| Tags | **no mapping** — `messaging.Message` has no tag concept; driver publishes untagged messages | **no mapping** — subscription hardcodes selector `TAG *`; tag-filtered consumption needs the raw client |
| Timestamp | not sent | StoreTimestamp (broker store time), not BornTimestamp |
| Topic | fixed at NewPublisher | not surfaced on the envelope |

The round trip that does survive cleanly: Payload, custom Headers, Key (single), and the trace
context — exactly what [example/main.go] asserts and what TestFromMessageExt covers
[rocketmq_test.go:102-117].

---

## 3. Per-key behavior reference

All keys live under `spring.rocketmq.instances.<name>.` (per-instance prefix binding via `conf.BindEach`,
not the absolute-property Pool rule). Seven value tags in the starter — reconciled with the
grep, no extras on either side.

| Key | Type | Default | Behavior / interactions | Misconfiguration consequence |
|-----|------|---------|-------------------------|------------------------------|
| `name-servers` | []string | — | **Required** (`expr:"len($) > 0"` [config.go:35]). Address list of the RocketMQ NameServer. Also feeds the ping probe. | Missing/empty → bind-time validation error. |
| `instance-name` | string | `""` | Distinguishes clients on one host. Empty is safe: the SDK rewrites "DEFAULT" to a per-producer/consumer unique name (see the README's design notes). Set it to make remoting clients share one connection pool. | Unnecessary explicit value → shared pool where you wanted isolation. |
| `access-key` | string | `""` | ACL access key. ⚠ Must pair with `secret-key` — one-sided fails the boot with an error naming the client [starter.go:60-62]. | One-sided → boot error; wrong value → first produce/consume fails (probe is TCP-only). |
| `secret-key` | string | `""` | ACL secret key pairing with access-key [config.go:48]. | Same as above. |
| `send-timeout` | duration | `3s` | Stamped onto every producer (`WithSendMsgTimeout`) [client.go:73]. | Too low → sync sends time out under load. |
| `retry` | int | `2` | Producer-internal retries before a sync send fails (`WithRetry`); 2 = up to 3 attempts [client.go:74, config.go:55]. This is the SDK's loop and the only one that fires: the publish/consume operations are declared `NonIdempotent`, so a governance-side retry rule for the label is suppressed. | Large value + slow broker → latency amplification. |
| `ping` | bool | `false` | Opt-in TCP dial against the name server list at bean creation; first reachable address satisfies it, 3s per dial [driver.go:146-160]. | true → wrong addresses abort boot; false → surface only on first use. |

---

## 4. Verification & fault drills

### 4.1 Round trip incl. driver mapping field survival

Run [example/check.sh](example/check.sh) (compose with `-p` isolation, topic creation, marker
gating — see it for why each gate exists), or manually:

```bash
go run .                                  # expect "Response from server: value"
```

The example asserts Payload and custom Headers survive the round trip [example/main.go:110-120].
To assert Key survival yourself, log `msg.Key` in the handler — expect the same single string.

### 4.2 Broker-down ping probe

```properties
spring.rocketmq.instances.a.name-servers=127.0.0.1:19876   # nothing listening
spring.rocketmq.instances.a.ping=true                      # opt in to the boot probe
```

Boot fails with "rocketmq name server probe failed on ..." [starter.go:74-79] when `ping=true`;
the default (`ping=false`) lets the same config boot fine and fail on first use. Note the probe
proves reachability only — a valid TCP endpoint with wrong ACL still boots.

### 4.3 Guarded vs unguarded

With a governance rate-limit policy on service `rocketmq:127.0.0.1:9876` (label = `rocketmq:` +
comma-joined name-servers — check it matches your governance rules):

- Hammer `GuardedSend` — or the driver's `Publish`, which routes through the same `GuardedSend`
  seam — → rejections return `resilience.ErrRateLimited`, the send is never invoked, and
  `resilience.client.calls` records appear with `resilience.outcome=rate_limited`, alongside one access log
  per call (`_app_rocketmq_access`, or `_app_resilience` on the fallback path)
  [cloud/resilience/observe.go].
- Raw `SendSync` called directly on a producer is unguarded (§2.2): it is the app's own path and
  has no executor around it. That contrast IS the §2.2 boundary.

### 4.4 Governance label check

```bash
curl -s :9090/metrics | grep resilience_calls
# attribute resilience.service / service label must be "rocketmq:127.0.0.1:9876" —
# the name-server list, NOT the config entry name (§2.2)
```

### 4.5 Metrics / span / log reads

- Driver path (declared operations, emitted by the resilience layer): spans named `publish` /
  `consume`, histogram `messaging.client.operation.duration` (call level, unit s),
  `messaging.client.attempt.duration` (attempt level, one record per retry) and up-down counter
  `messaging.client.active_requests`; attributes `messaging.system=rocketmq`,
  `messaging.operation=publish|consume`, plus `status` and the topic as span/log detail. The
  `resilience.client.calls` counter always counts the call. Access log tag `_app_rocketmq_access`:
  fields `messaging.operation`, `messaging.destination.name` (truncated to 512), `status`,
  `duration_ms`; success with a topic at Debug, error at Warn.
- Guarded path (undeclared fallback): histogram `resilience.client.duration` and counters
  `resilience.client.calls` / `resilience.client.breaker.state_change`, log tag `_app_resilience`.
- Manual helpers (raw client): spans `rocketmq.produce` / `rocketmq.consume <topic>` from tracer
  `go-spring.org/starter-rocketmq` with `messaging.system/destination.name/operation` attributes
  [command.go]; W3C context rides the user properties via `msgCarrier` (round-trip proved by
  TestMsgCarrierRoundTrip [rocketmq_test.go:74-98]). [example-otel](example-otel/main.go) verifies
  the linked spans land in Jaeger (`:16686`, service `rocketmq-otel-example`).
- Load-test marker drill: publish under a ctx marked by the traffic layer; inside the consumer
  handler the propagator's `IsLoadTest(ctx)` is true via the `x-loadtest` property [messaging.go].

---

## 5. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| Boot fails "name server probe failed" | Wrong/unreachable `name-servers` | Fix the list; the probe dials each address with a 3s budget [driver.go:149]. |
| Boot fails "access-key and secret-key must be set together" | One-sided ACL key [starter.go:60] | Set both or neither. |
| Subscribe fails right after topic creation | Route not yet visible on the name server (heartbeat lag, up to 60s) | Gate on `mqadmin topicList -n namesrv:9876` before starting the app (see check.sh); example-otel retries Subscribe 20×500ms for the same reason. |
| Consumer silently receives nothing | Topic missing at Subscribe time, or wrong consumer group | Create the topic up front; remember the group is the competing-consumers unit. |
| No resilience effect on a raw `SendSync` | The raw send path is not guarded; only `GuardedSend` and the driver are | Route protected sends through `GuardedSend`, or use the driver (§2.2). |
| Everything works, no traces | starter-otel not imported | All OTel helpers are silent no-ops without it [command.go:46-48]. |
| SDK connection/rebalance logs missing | Level config filters them | They arrive through the bridge at tag `app` with a `rocketmq:` prefix [driver.go:124-139]; Fatal maps to Error [driver.go:114-116]. |
| Consumer keeps redelivering | Handler returns error → ConsumeRetryLater forever | Fix the handler; there is no DLQ wiring in the driver — use RocketMQ's retry/DLQ semantics with the raw client if needed. |
| Duplicate processing after redeploy | Same group + rebalancing; StoreTimestamp-based envelope | Expected RocketMQ clustering behavior; see official docs on consumer groups. |

## 6. Design Health

| Metric | Value |
|--------|-------|
| Config keys | 7 (all in Config) |
| Required | 1 (`name-servers`) |
| Quickstart external deps | 2 (namesrv + broker, one compose) |
| "Watch out" entries | 6 |

Design suspects (audit ledger; from the previous doc unless noted):

- No TLS config surface though the SDK supports it (carried over, unfixed).
- Driver Subscribe hardcodes a `TAG *` selector; no tag sub-expression support, and
  `messaging.Message` cannot carry tags at all (§2.4 table) (carried over, unfixed).
- Driver Timestamp uses StoreTimestamp, not born time (carried over, unfixed).
- Consume envelope leaks SDK-internal user properties (`traceparent`, KEYS, `x-loadtest`) into
  `Headers`, and multi-key round trips return a joined string (§2.4; new).
- The `name-servers` segment of the service label is dead — `ResourceLabel` returns at the
  client name, so the passed address list can never appear in the label (§2.2; new; the previous
  doc's `rocketmq|<name>|<name-servers>` label claim was wrong and is corrected here).
- Fixed: the driver publish and consume paths now declare their operation and run under the same
  resilience executor as the raw `GuardedSend` path, so the starter emits nothing per call and the
  produce-path protection/observability asymmetry is gone (§2.2, §2.3, §2.4); raw direct
  `SendSync` stays unguarded by design.

# starter-kafka-sarama Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-kafka-sarama` is a Client-archetype starter (`starter/DESIGN.md`
§2.2) that provisions Sarama `sarama.Client` instances. It coexists with
`starter-kafka` (franz-go) under the shared `spring.kafka` config prefix —
switching implementations is a blank-import swap, no config surgery
(`project_starter_kafka_sarama`).

## 1. Responsibilities & Boundaries

- Binds each `spring.kafka.instances.<name>` entry to one
  `sarama.Client` bean via `gs.Group`. No single-instance default.
- The exposed bean is `sarama.Client` — producer, consumer group, admin
  clients are constructed by callers on top of the shared client, so a
  single connection pool serves every downstream role.
- Deliberately not wrapped in a `*Conn` type: unlike NATS's dual API,
  Sarama has one `sarama.Client` interface as the natural bean.
- No OTel producer/consumer hook. Sarama does not expose an interceptor
  seam; `otelsarama` is deprecated and locked to Shopify's fork
  (`project_starter_kafka_sarama`). Tracing lives at the call site.

## 2. Key Abstractions & Seams

- **Shared prefix, not shared bean.** `spring.kafka` is the seam
  (`feedback_websocket_shared_config_prefix`): both sarama and franz-go
  read the same tree, but only one is imported per process. Switching
  is `_ "…/starter-kafka"` ↔ `_ "…/starter-kafka-sarama"`.
- **Single client for all roles.** Producers / consumer groups / admin
  built off the shared `sarama.Client` reuse its metadata cache and
  broker connections. The starter does not pre-create these; callers
  build them where they own the lifecycle.
- **TLS / SASL are config-driven.** Enabled flags gate blocks; when
  `sasl.enabled=true`, `mechanism` picks `PLAIN` / `SCRAM-SHA-256` /
  `SCRAM-SHA-512`.
- **Producer knobs are Sarama-native.** `producer.required-acks`,
  `producer.idempotent`, `producer.compression` map directly to
  `sarama.Config` fields — no abstraction over Sarama semantics.

## 3. Constraints

- **`Brokers` is required.** No localhost fallback; empty list is
  rejected at boot.
- **Sarama version pinning matters.** `version` must be set (e.g.
  `2.6.0`) for anything beyond the baseline protocol — features (SASL
  mechanisms, headers, idempotent producer) require a minimum protocol
  version.
- **No OTel producer/consumer wrapping.** If tracing is required,
  callers add span helpers at publish / consume boundaries; the starter
  will not silently modify `sarama.Config` to insert interceptors that
  do not exist.
- **`destroy = Close`.** `sarama.Client.Close` releases broker
  connections. Callers that built consumer groups / producers on top
  must close those first (their own lifecycle).

## 4. Trade-offs / Alternatives Rejected

- **Bundling producer / consumer beans — rejected.** Each caller's
  producer/consumer group has its own topic, partitioning, error
  handler, and rebalance strategy; a one-size-fits-all bean would hide
  more than it saves. The starter exposes `sarama.Client`; callers own
  the specialised beans.
- **`otelsarama` hook — rejected.** Deprecated upstream, locked to a
  vendor fork (Shopify); accepting it would tie the starter to a
  discontinued path.

# messaging Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`messaging` is the zero-dependency stdlib abstraction that gives Go-Spring
Spring Cloud Stream-equivalent broker independence: publish and consume
through one envelope + interface pair, and let starters bind concrete brokers
underneath. Broker starters (`starter-nats`, `starter-kafka`,
`starter-kafka-sarama`, `starter-pulsar`, `starter-rabbitmq`,
`starter-mqtt`) implement `Binder` over their native clients.

## 1. Responsibilities & Boundaries

- Broker-neutral programming model: `Message`, `Publisher`, `Subscriber`,
  `Binder`. Nothing broker-specific leaks into the interfaces.
- Deliberately **no** functional Supplier/Function/Consumer generic layer.
  Excessive abstraction obscures the raw-client escape hatch that broker
  starters still expose (JetStream, admin, transactions, ...).
- Not the tracing library, not the retry / DLQ library. Trace context piggy-
  backs on `Headers`; retry / requeue / nack semantics are broker-specific
  and documented per starter.
- Not the schema registry. `Payload` is opaque `[]byte`; encoding lives one
  layer up.

## 2. Key Abstractions & Seams

- `Message` — `{Key, Payload, Headers, Timestamp}`. Nil-safe `Header` /
  `SetHeader` because `Headers` doubles as a W3C `TextMapCarrier`.
- `Handler = func(ctx, *Message) error`. A non-nil return signals delivery
  failure; how it surfaces (nack, redelivery, log) is broker-specific.
- `Publisher.Publish` / `.Close`, `Subscriber.Subscribe` / `.Close`. Both are
  bound to their destination / source at construction — a Publisher writes
  to one place, a Subscriber reads from one place.
- `Binder` opens `Publisher` / `Subscriber` from one broker connection. The
  string destination / source / group is interpreted **in the broker's own
  terms** (subject, topic, queue, consumer-group, subscription-name).
- `RegisterBinder` / `GetBinder` / `MustGetBinder` — the driver-registry
  parity seam (same shape as `discovery.Register`, `resilience.RegisterDriver`
  — panics on empty name, nil, or duplicate). Real broker binders are
  connection-bound and are typically wired as beans via `NewBinder(conn)`;
  the registry is kept for callers that want a single process-wide binder
  chosen by configured name.

## 3. Constraints (do not break)

- **`Headers` is nil-safe on read but allocated on write**. Producers may
  leave it zero; binders inject trace context via `SetHeader`.
- **Instance-bound**. Binders are constructor-wired (`NewBinder(conn)`) so
  the same broker connection powers publishers and subscribers; do not
  reintroduce a global "default binder" that owns a client.
- **Zero third-party imports** in this package. Broker SDKs live only in
  starters that implement `Binder`.
- **`group == ""` semantics**: broadcast where the broker supports it (NATS
  fanout, JetStream ephemeral); brokers whose queue is intrinsically a
  competing-consumer group (RabbitMQ) ignore the parameter — starter
  documents its exact interpretation.

## 4. Trade-offs / Alternatives Rejected

- **No functional Supplier/Function/Consumer sugar layer**. It would trap
  users behind an over-abstracted API and complicate the escape hatch. The
  raw-client bean is retained instead.
- **`Subscribe` returns after establishment, not on delivery completion**.
  Long-lived delivery loops belong to the binder implementation.
- **MQTT (3.1.1) intentionally does not use `Key`/`Headers`/`Timestamp`**.
  The wire has no per-message metadata; the starter documents payload-only
  and skips trace-context propagation.
- **Kafka (franz-go) constraint**: topics/group are fixed on the client, so
  one client bean = one logical consumer; a starter that needs many consumer
  groups builds multiple clients (or uses the sarama variant).

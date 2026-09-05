# starter-rocketmq Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A Client-archetype starter (`starter/DESIGN.md` §2.2) for RocketMQ, adapted
to the fact that rocketmq-client-go v2 has no unified client entity. It
follows the messaging-starter family conventions established by
starter-pulsar / starter-kafka.

## 1. Responsibilities & Boundaries

- **Owns**: bean lifecycle for the `spring.rocketmq.<name>` group (multi-
  instance, container-managed teardown), common-option application to every
  producer/consumer it creates, the rlog bridge into go-spring's log, the
  fail-fast name server probe, the `messaging.Driver` adapter, OTel span
  helpers, and the `GuardedSend` resilience seam.
- **Does not own**: message serialization (payload stays `[]byte`), ordering
  semantics (the driver is concurrently-consumed; use the raw client for
  orderly/transactional messaging), topic administration, and broker health
  probing (see §3).

## 2. Key Abstractions & Seams

- **`Client` wrapper bean** — rocketmq-client-go constructs producers and
  consumers independently; there is no shared `rocketmq.Client` object like
  pulsar's. The bean is therefore a starter-owned wrapper that holds the name
  server list, credentials and instance name once, applies them to everything
  it creates, and registers every producer/consumer for teardown on Close.
- **`Driver` (driver.go)** — the construction seam: registry + DefaultDriver
  that assembles the wrapper. The rlog bridge is process-global, so it is
  installed exactly once inside DefaultDriver rather than per instance.
- **`NewDriver` (driver.go)** — adapts to `messaging.Driver`: one started
  producer per publisher, one started push consumer per subscriber
  (`Subscribe` before `Start`, per the SDK's contract). A handler error maps
  to `ConsumeRetryLater` (the broker redelivers), nil to `ConsumeSuccess`.
  W3C trace context and the load-test marker ride the user properties via
  `msgCarrier` (a `propagation.TextMapCarrier` over `primitive.Message`,
  mirroring kafka-go's `recordCarrier`).
- **`GuardedSend` (command.go)** — the resilience seam: the SDK exposes no
  reject-capable middleware, so the governance executor is an opt-in wrapper
  on the synchronous `SendSync` path, resolved through the neutral
  `resilience.ExecutorFor` / `fault.InjectorFor` seams (no coupling to
  starter-governance).

## 3. Constraints

- The SDK's default instance name is rewritten to `PID#nano` per
  producer/consumer when left at "DEFAULT" (`internal.ChangeInstanceNameToPID`),
  so leaving `instance-name` empty is safe for multi-producer processes; an
  explicit `instance-name` makes the underlying remoting clients share one
  connection pool instead.
- `Subscribe` must precede `Start` on a push consumer; the wrapper's
  `NewPushConsumer` therefore returns an unstarted consumer and the driver
  performs the dance internally.
- The fail-fast probe is a TCP dial, not a broker round trip: RocketMQ's
  remoting layer connects lazily and a dial is the only probe that is cheap,
  side-effect-free, and topology-independent. It catches wrong addresses, not
  ACL errors.
- No `health.Indicator`: consistent with starter-kafka/starter-pulsar —
  there is no cheap broker probe that works across ACL'd and routed
  deployments; an app-level indicator (producer round trip) is documented in
  the README instead.
- ACL keys are validated in pairs at bind time; one-sided credentials fail
  fast with an explanation.

## 4. Trade-offs / Alternatives Rejected

- **Wrapper bean vs. raw producer bean**: a default-producer bean would hide
  consumer construction and force group choices into config; the wrapper keeps
  the raw SDK as the escape hatch for both directions and matches how the
  SDK actually composes.
- **rocketmq-client-go v2 (remoting) vs. rocketmq-clients gRPC (5.x proxy)**:
  the gRPC client is RC-quality with an awkward module layout (`+incompatible`
  root) and requires a 5.x proxy; the v2 client is the Apache-blessed stable
  path and works against both 4.x and 5.x clusters via the NameServer
  protocol.

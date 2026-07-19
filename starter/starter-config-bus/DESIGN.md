# starter-config-bus Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

`starter-config-bus` sits in the integration layer as a *composed* starter:
it does not open a listener and does not run a config provider. Instead it
adds a **Spring-Cloud-Bus-style refresh broadcast** on top of an existing NATS
connection contributed by `starter-nats`, so a change published once refreshes
every instance in the fleet.

## 1. Responsibilities & Boundaries

- Subscribes to a shared NATS subject and, on every incoming signal, re-runs
  the application-wide property refresh via `*gs.PropertiesRefresher`.
- Publishes refresh signals through the same subject via `ConfigBus.Publish`,
  so an admin action, a webhook, or a management endpoint can force a
  coordinated refresh.
- Carries refresh **signals only**, never configuration content. The remote
  config-center starters (nacos/etcd/consul/vault/k8s/file) remain the single
  source of truth; the bus only tells subscribers "reload from your own
  sources now".
- Explicitly does **not** own the NATS connection. It injects a
  `*StarterNats.Conn` by name (default `config-bus`) and lets `starter-nats`
  handle lifecycle and close.

## 2. Key Abstractions & Seams

- **`ConfigBus` bean.** Registered as a named root object (`configBus`)
  exported as `gs.Rooter`, so it is always instantiated even if the app does
  not inject it explicitly. `Init` calls `subscribe`; `Destroy` calls
  `Unsubscribe`.
- **`RefreshEvent` payload.** `{prefix, origin}`. `Prefix` is the only field
  that influences dispatch; `Origin` is opt-in metadata for observability
  and never controls behavior.
- **Prefix-scoped subscription.** `Config.WatchPrefixes` is a comma-separated
  list. A broadcast applies to an instance when its `Prefix` is empty (full
  fleet), or the subscriber's `WatchPrefixes` is empty (subscribe to
  everything), or the event's prefix overlaps a watched prefix in either
  direction — so a `"db"` subscriber reacts to a `"db.pool"` event and vice
  versa.
- **Transport by instance name.** `Conn` is injected via
  `autowire:"${spring.config.bus.nats-instance:=config-bus}"`, so the app
  chooses which NATS instance under `spring.nats.instances.*` carries the
  bus.

## 3. Constraints

- **Signals, not payloads.** A malformed message logs a warning and is
  dropped; a missing prefix means "refresh everyone". The application must
  never rely on message bodies as configuration.
- **Refresh failures are logged, not raised.** `RefreshProperties` errors are
  logged and the bus continues to accept future signals; a broken refresh
  must not silently unsubscribe the instance from the fleet.
- **Named bean is load-bearing.** Same rule as the config-provider starters:
  `gs.Rooter` is `any`, so `configBus` must not go under `__default__`.
- **Transitive dep on `starter-nats`.** The bus references
  `go-spring.org/starter-nats` and, via `Conn`, obtains JetStream when the
  underlying connection has it — but the bus itself only uses core pub/sub.

## 4. Trade-offs / Alternatives Rejected

- **Broadcasting configuration content — rejected.** It would make the bus a
  second source of truth and race with each subscriber's own config-center
  watch. Keeping the payload to a hint preserves "one source of truth per
  key".
- **Choosing a different transport (Kafka/Redis) — deferred.** NATS was
  chosen for the first implementation because a Go-Spring app is highly
  likely to already run it; a second transport can be a peer starter with
  the same subject/prefix model.
- **JetStream durable subscriptions — deliberately not used.** A missed
  broadcast is recoverable: an instance's own remote config watcher will
  observe the underlying change on its next tick, and admins can always
  republish. Durability would add operational cost without changing the
  correctness model.

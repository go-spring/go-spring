# starter-kafka Cloud-Native Example

A flagship for starter-kafka composing the cloud-native capability set around a
Kafka (franz-go) client — a **produce → consume round-trip** as the client op,
guarded by **resilience**, with **health** and **dynamic config**.

## Capabilities

- **Round-trip**: the client op is a synchronous produce through the broker
  followed by a consume back off the same topic — the MQ archetype that proves
  broker reachability end to end.
- **Resilience**: with `spring.kafka.instances.a.resilience.enabled`, every synchronous
  produce runs through `GuardedProduceSync` → the builtin `"default"` executor; a
  burst over `rate-limit` is rejected with `ErrRateLimited`. Consume is not
  guarded (franz-go's poll path is passive).
- **Health**: the starter registers no `health.Indicator`, so the app exports its
  own (a broker `Ping` probe) and `starter-actuator` aggregates it on `:9370` —
  `/readyz` reflects broker reachability.
- **Dynamic config**: a `gs.Dync[string]` field is bound to a watched file; the
  `file-watch` provider hot-reloads it with no restart.
- **Observability**: kotel spans/metrics + the observe access-log hook ride the
  OTel globals installed by `starter-otel` when present.

Discovery is intentionally omitted: Kafka seeds its clients with bootstrap
brokers (`spring.kafka.instances.a.brokers`), so there is no service-name to resolve.

## Layout

```
example.go               the flagship app (container + self-test)
conf/app.properties      kafka, resilience, actuator config
check.sh                 docker-gated smoke test
docker-compose.yml       single-node KRaft Kafka
```

## Manual Testing

Requires a local Kafka (`docker compose up -d`):

```bash
cd starter/experimental/starter-kafka/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (broker Ping probe)
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up a single-node KRaft Kafka via docker compose, then asserts
the actuator probes are UP, a produce → consume round-trip succeeds, a burst is
partly rejected with `ErrRateLimited`, and the dynamic-config label hot-reloads;
exit code 0 means pass. Skipped gracefully when docker is unavailable.

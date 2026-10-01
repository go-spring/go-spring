# starter-gorm-mysql Cloud-Native Example

A flagship for starter-gorm-mysql composing the cloud-native capability set
around a storage client — **discovery**, **resilience**, **health**,
**dynamic configuration** and **observability**.

## Capabilities

- **Service discovery**: the DB address is resolved through a registered
  discovery backend (`service-name`), not hardcoded config — a query only
  succeeds if resolve → dial → serve works.
- **Resilience**: with `resilience.enabled`, every query runs through the builtin
  `"default"` executor; a burst over `rate-limit` is rejected with
  `ErrRateLimited`.
- **Health**: the per-instance gorm `health.Indicator` is aggregated by
  `starter-actuator` on `:9370` — `/readyz` reflects the pool.
- **Dynamic configuration**: a `gs.Dync[string]` field is bound to a watched file
  (`file-watch`); editing it hot-reloads the value with no restart.
- **Observability**: the gorm observe plugin declares each operation; the resilience
  layer emits the span, metrics and access log through the OTel globals.

## Layout

```
example.go               the flagship app (container + self-test)
conf/app.properties      discovery, resilience, actuator, file-watch config
check.sh                 docker-gated smoke test
```

## Manual Testing

Requires a local MySQL (`docker compose up -d`):

```bash
cd starter/starter-gorm-mysql/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (db pool)
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up MySQL via docker compose, then asserts health probes are UP,
a discovery-resolved query succeeds, a burst is partly rejected with
`ErrRateLimited`, and a watched file hot-reloads; exit code 0 means pass. Skipped
gracefully when docker is unavailable.

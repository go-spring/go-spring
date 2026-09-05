# starter-memcached Cloud-Native Example

A flagship for starter-memcached composing the cloud-native capability set around
a cache client — **discovery**, **resilience**, **health** and **observability**.

## Capabilities

- **Service discovery**: the memcached address is resolved through a registered
  discovery backend (`service-name`), not hardcoded config — a round-trip only
  succeeds if resolve → dial → serve works.
- **Resilience**: with `resilience.enabled`, every command runs through the
  builtin `"default"` executor; a burst over `rate-limit` is rejected with
  `ErrRateLimited`. The resilience config is a `gs.Dync` field, so it is
  hot-reloadable.
- **Health**: the per-instance memcached `health.Indicator` is aggregated by
  `starter-actuator` on `:9370` — `/readyz` reflects the cluster.
- **Observability**: the starter's built-in observe layer rides the
  OTel globals.

## Layout

```
example.go               the flagship app (container + self-test)
conf/app.properties      discovery, resilience, actuator config
check.sh                 docker-gated smoke test
```

## Manual Testing

Requires a local memcached (`docker compose up -d`):

```bash
cd starter/starter-memcached/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (memcached cluster)
printf 'add cn:key 0 0 5\r\nhello\r\n' | nc 127.0.0.1 11211   # the instance resolved via discovery
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up memcached via docker compose, then asserts health probes are
UP, a discovery-resolved round-trip succeeds, and a burst is partly rejected with
`ErrRateLimited`; exit code 0 means pass. Skipped gracefully when docker is
unavailable.

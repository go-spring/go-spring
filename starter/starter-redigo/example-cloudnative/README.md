# starter-redigo Cloud-Native Example

A flagship for starter-redigo composing the cloud-native capability set around
a connection pool — **discovery**, **resilience**, **health** and
**observability**.

## Capabilities

- **Service discovery**: the redis address is resolved through a registered
  discovery backend (`service-name`), not hardcoded config — a round-trip only
  succeeds if resolve → dial → serve works.
- **Resilience**: with `resilience.enabled`, every command runs through the
  builtin `"default"` executor; a burst over `rate-limit` is rejected with
  `ErrRateLimited`. The resilience config is a `gs.Dync` field, so it is
  hot-reloadable.
- **Health**: the per-instance redigo `health.Indicator` is aggregated by
  `starter-actuator` on `:9370` — `/readyz` reflects the pool.
- **Observability**: the starter-built-in observe layer rides the OTel globals.
- **Dynamic config**: a `gs.Dync[string]` label is bound to a watched file; editing
  it hot-reloads the value with no restart.

## Layout

```
example.go               the flagship app (container + self-test)
conf/app.properties      discovery, resilience, actuator config
check.sh                 docker-gated smoke test
```

## Manual Testing

Requires a local Redis (`docker compose up -d`):

```bash
cd starter/starter-redigo/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (redis pool)
redis-cli set cn:key hello                        # the instance resolved via discovery
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up Redis via docker compose, then asserts health probes are UP,
a discovery-resolved round-trip succeeds, and a burst is partly rejected with
`ErrRateLimited`; exit code 0 means pass. Skipped gracefully when docker is
unavailable.

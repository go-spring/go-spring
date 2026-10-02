# starter-elasticsearch Cloud-Native Example

A flagship for starter-elasticsearch composing the cloud-native capability set
around an Elasticsearch client — **discovery**, **resilience**, **health** and
**dynamic config**.

## Capabilities

- **Service discovery**: the node addresses are resolved through a registered
  discovery backend (`service-name`), not hardcoded config — an index round-trip
  only succeeds if resolve → dial → serve works.
- **Resilience**: with `resilience.enabled`, every request runs through the
  builtin `"default"` executor; a burst over `rate-limit` is rejected with
  `ErrRateLimited`. The resilience config is a `gs.Dync` field, so it is
  hot-reloadable.
- **Health**: the per-instance elasticsearch `health.Indicator` is aggregated by
  `starter-actuator` on `:9370` — `/readyz` reflects the cluster.
- **Dynamic config**: a `gs.Dync[string]` label is bound to a watched file;
  editing it hot-reloads the value with no restart.

## Layout

```
main.go               the flagship app (container + self-test)
conf/app.properties      discovery, resilience, actuator config
check.sh                 docker-gated smoke test
```

## Manual Testing

Requires a local Elasticsearch (`docker compose up -d`):

```bash
cd starter/experimental/starter-elasticsearch/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz    # health (elasticsearch cluster)
curl 'http://127.0.0.1:9200'         # the cluster resolved via discovery
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up Elasticsearch via docker compose, then asserts health probes
are UP, a discovery-resolved index/get round-trip succeeds, and a burst is partly
rejected with `ErrRateLimited`; exit code 0 means pass. Skipped gracefully when
docker is unavailable.

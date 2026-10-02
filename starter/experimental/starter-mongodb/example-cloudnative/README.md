# starter-mongodb Cloud-Native Example

A flagship for starter-mongodb composing the cloud-native capability set around
a MongoDB client — **discovery**, **resilience**, **health** and **dynamic
config**.

## Capabilities

- **Service discovery**: the `disc` instance's address is resolved through a
  registered discovery backend (`service-name=mongo-cluster`), not hardcoded
  config — a round-trip only succeeds if resolve → dial → serve works.
- **Resilience**: with `resilience.enabled`, every connection dial flows through
  the builtin `"default"` executor; a burst of fresh connections over `rate-limit`
  is rejected with `ErrRateLimited`. Unlike go-redis (per-command hook), MongoDB
  exposes no per-command hook, so the seam is the **dial layer**: connection
  establishment is protected, not each already-open operation. The resilience
  config is a `gs.Dync` field, so it is hot-reloadable.
- **Health**: the per-instance mongodb `health.Indicator` is aggregated by
  `starter-actuator` on `:9370` — `/readyz` reflects the clients' pools.
- **Dynamic config**: a `gs.Dync[string]` label is bound to a watched file;
  editing it hot-reloads the value with no restart.

## Layout

```
main.go               the flagship app (container + self-test)
conf/app.properties      mongodb, discovery, resilience, actuator config
check.sh                 docker-gated smoke test
```

## Manual Testing

Requires a local MongoDB (`docker compose up -d`):

```bash
cd starter/experimental/starter-mongodb/example-cloudnative
docker compose up -d
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (mongodb clients)
# the "disc" instance was resolved via discovery (mongo-cluster)
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up MongoDB via docker compose, then asserts health probes are
UP, a direct CRUD round-trip succeeds, a discovery-resolved round-trip succeeds,
and a burst of fresh connections is partly rejected with `ErrRateLimited`; exit
code 0 means pass. Skipped gracefully when docker is unavailable.

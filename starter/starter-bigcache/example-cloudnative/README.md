# starter-bigcache Cloud-Native Example

A flagship for starter-bigcache composing the cloud-native capability set around
an **in-process** cache — **health**, **resilience**, **dynamic config** and
**observability**. Because bigcache has no external server, this example is fully
**self-contained**: no docker, no docker-compose, no service discovery of a remote
backend (the app dials nothing).

## Capabilities

- **Health**: the per-instance bigcache `health.Indicator` is auto-exported by the
  starter and aggregated by `starter-actuator` on `:9370` — `/readyz` + `/health`
  report UP (an in-process instance existing means it is ready).
- **Resilience**: with `resilience.enabled`, every `Get`/`Set`/`Delete` runs
  through the builtin `"default"` executor; a burst over `rate-limit` is rejected
  with `ErrRateLimited`. The resilience config is a `gs.Dync` field, so the policy
  is hot-reloadable.
- **Dynamic config**: a `gs.Dync[string]` field is bound to `demo.label` from a
  watched file; editing it hot-reloads the value with no restart.
- **Observability**: bigcache exposes no plugin hook, so the starter's
  `*`*Cache` wrapper delivers per-operation span+metric+log, plus
  cache-stat gauges (hits/misses/collisions) riding the OTel globals.
- **Discovery**: N/A — bigcache is in-process, so there is no external address to
  resolve.

## Layout

```
example.go               the flagship app (container + self-test)
conf/app.properties      bigcache instance, resilience, actuator, import config
check.sh                 self-contained smoke test (go run + 60s watchdog)
```

## Manual Testing

No external service is required — just run it:

```bash
cd starter/starter-bigcache/example-cloudnative
go run . -manual
```

In another terminal:

```bash
curl http://127.0.0.1:9370/readyz                 # health (bigcache indicator)
```

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example under a 60s watchdog; the app self-asserts the health
probes are UP, an in-process Set/Get/Delete round-trip succeeds, and a burst is
partly rejected with `ErrRateLimited`; exit code 0 means pass.

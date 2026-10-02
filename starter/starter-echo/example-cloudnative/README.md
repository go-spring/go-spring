# starter-echo Cloud-Native Example

A self-contained flagship for starter-echo that composes the full cloud-native
capability set in one app — **health**, **service discovery**, **resilience**,
**observability** and **dynamic configuration** — with no external services or
docker required.

## Capabilities

- **Health**: a `health.Indicator` bean is aggregated by `starter-actuator` on
  its management port (`:9370`). Toggling it flips `/readyz` between `200` and
  `503` while `/health` stays up.
- **Service discovery**: a `discovery.NewStaticDiscovery` backend hands out the
  app's own echo address (`:8082`); the app resolves it client-side and dials it
  end-to-end.
- **Resilience**: the builtin `"default"` driver rate-limits an arbitrary
  function (`Executor.Execute` → `ErrRateLimited`) and an inbound route
  (`/limited` → `429`).
- **Dynamic configuration**: a `gs.Dync[string]` field is bound to a watched
  file (`file-watch`). Editing it the way the kubelet swaps a ConfigMap
  hot-reloads the value into `/greeting`.
- **Observability**: starter-echo's Tracing/Metrics/AccessLog middleware are on
  by default, riding the OTel globals.

## Layout

```
main.go               the flagship app (container + self-test)
conf/app.properties      ports, actuator, file-watch imports
check.sh                 zero-dependency smoke test
```

| Port | Owner |
|------|-------|
| `:8082` | starter-echo business routes (`/`, `/greeting`, `/limited`) |
| `:9370` | starter-actuator probes (`/health`, `/readyz`, `/startup`, `/info`) |

## Manual Testing

```bash
cd starter/starter-echo/example-cloudnative
go run . -manual
```

In another terminal, exercise each capability:

```bash
curl http://127.0.0.1:9370/readyz                 # health
curl http://127.0.0.1:8082/greeting               # dynamic config (edit ./mount to hot-reload)
for i in $(seq 1 15); do curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8082/limited; done
curl http://127.0.0.1:8082/                       # discovery resolves the app itself
```

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the app and waits for the self-test to assert all capabilities;
exit code 0 means pass.

# Cloud-Native Flagship Example

A single self-contained application that composes the four cross-cutting
cloud-native capabilities go-spring ships: **health**, **service discovery**,
**resilience** and **dynamic configuration**. No external services or docker are
required — the app resolves a static discovery backend pointing at itself and
hot-reloads a watched local file.

## Capabilities

- **Health**: a `health.Indicator` bean is aggregated by `starter-actuator` on
  its management port (`:9370`). Toggling the indicator flips `/readyz` between
  `200` and `503` while `/health` stays up (a degraded dependency never trips
  liveness).
- **Service discovery**: a `discovery.NewStaticDiscovery` backend hands out the
  app's own gin address (`:8081`); the app resolves it client-side and dials the
  returned endpoint end-to-end.
- **Resilience**: the builtin `"default"` driver rate-limits both an arbitrary
  function (`Executor.Execute` → `ErrRateLimited`) and an inbound HTTP route
  (`/limited` → `429`).
- **Dynamic configuration**: a `gs.Dync[string]` field is bound to a watched
  file (`file-watch` provider). Editing the file the way the kubelet swaps a
  ConfigMap (atomic `..data` symlink) hot-reloads the value into the `/greeting`
  handler with no restart.

## Layout

```
main.go               the flagship app (container + self-test)
conf/app.properties   ports, actuator, file-watch imports
check.sh              zero-dependency smoke test
```

| Port | Owner |
|------|-------|
| `:8081` | starter-gin business routes (`/`, `/greeting`, `/limited`) |
| `:9370` | starter-actuator probes (`/health`, `/readyz`, `/startup`, `/info`) |

## Manual Testing

Terminal 1 — start the app:

```bash
cd examples/cloudnative
go run . -manual
```

Terminal 2 — exercise each capability:

```bash
# Health: readiness aggregate
curl http://127.0.0.1:9370/readyz

# Dynamic config: current greeting
curl http://127.0.0.1:8081/greeting
# edit ./mount/application.properties -> demo.greeting=hello-2
# curl again; the value changed with no restart

# Resilience: burst the rate-limited route -> some 429s
for i in $(seq 1 15); do curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8081/limited; done

# Discovery: the app resolves its own gin address client-side
curl http://127.0.0.1:8081/
```

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the app and waits for the self-test to assert all four
capabilities; exit code 0 means pass.

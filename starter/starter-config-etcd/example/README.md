# starter-config-etcd Example

etcd KV config management with starter-config-etcd.

## Features

- **Config loading**: Read config items from etcd KV
- **Config hot reload**: Modify KV via etcd API; the app picks up changes in real time
- **Dync dynamic binding**: Bind config via `Dync[T]`; refreshes automatically

> Requires a running etcd service. `check.sh` starts etcd via docker compose.

## Manual Testing

```bash
cd starter-config-etcd/example
go run . -manual
```

etcd must be running first:
```bash
# Start etcd
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, publish a new value and watch it print:

```bash
docker exec starter-etcd-config etcdctl put gs-config-demo 'demo.message=manual-1'
# prints: demo.message: "..." -> "manual-1"
```

Without `-manual` the example publishes a new value itself, waits for the bound
field to hot-reload, prints `hot-reload observed: hello-<hhmmss>`, and exits:

```bash
go run .
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts etcd via docker compose, runs the example and verifies config refresh, exit code 0 means pass.
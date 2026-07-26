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

Expected output:
```
initial value
updated value
```

Start etcd first:
```bash
# Start etcd
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts etcd via docker compose, runs the example and verifies config refresh, exit code 0 means pass.
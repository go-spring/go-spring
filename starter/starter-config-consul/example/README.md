# starter-config-consul Example

Consul KV config management with starter-config-consul.

## Features

- **Config loading**: Read config items from Consul KV
- **Config hot reload**: Modify KV via Consul API; the app picks up changes in real time
- **Dync dynamic binding**: Bind config via `Dync[T]`; refreshes automatically

> Requires a running Consul service. `check.sh` starts Consul via docker compose.

## Manual Testing

```bash
cd starter-config-consul/example
go run . -manual
```

Expected output:
```
initial value
updated value
```

Start Consul first:
```bash
# Start Consul
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Consul via docker compose, runs the example and verifies config refresh, exit code 0 means pass.

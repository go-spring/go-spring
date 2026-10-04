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

Consul must be running first:
```bash
# Start Consul
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, publish a new value and watch it print:

```bash
curl -fsS -X PUT -d 'demo.message=manual-1' http://127.0.0.1:8500/v1/kv/gs-config-demo
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

`check.sh` starts Consul via docker compose, runs the example and verifies config refresh, exit code 0 means pass.

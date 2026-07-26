# starter-config-nacos Example

Nacos config management with starter-config-nacos.

## Features

- **Config loading**: Read config from Nacos data-id
- **Config hot reload**: Publish new config via Nacos API; the app picks up changes in real time
- **Dync dynamic binding**: Bind config via `Dync[T]`; refreshes automatically

> Requires a running Nacos service. `check.sh` starts Nacos via docker compose.

## Manual Testing

```bash
cd starter-config-nacos/example
go run . -manual
```

Expected output:
```
initial value
updated value
```

Start Nacos first:
```bash
# Start Nacos
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Nacos via docker compose, runs the example and verifies config refresh, exit code 0 means pass.

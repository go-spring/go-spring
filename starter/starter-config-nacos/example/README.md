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

Nacos must be running first:
```bash
# Start Nacos
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

In another terminal, publish a new value and watch it print:

```bash
curl -fsS -X POST 'http://127.0.0.1:8848/nacos/v1/cs/configs' \
  -d 'dataId=gs-config-demo&group=DEFAULT_GROUP&content=demo.message=manual-1'
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

`check.sh` starts Nacos via docker compose, runs the example and verifies config refresh, exit code 0 means pass.

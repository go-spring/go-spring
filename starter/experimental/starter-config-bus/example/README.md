# starter-config-bus Example

Config bus dynamic refresh with starter-config-bus.

## Features

- **Config hot reload**: Publish new values via config bus; the app picks up changes in real time
- **Dync dynamic binding**: Bind config items via `Dync[T]`; values refresh automatically

> Requires Redis (config bus uses Redis as the message channel). `check.sh` starts Redis via docker compose.

## Manual Testing

```bash
cd starter-config-bus/example
go run . -manual
```

Expected output:
```
initial value
updated value
```

Start Redis first:
```bash
# Start Redis
docker compose up -d

# Run example (manual mode, keeps running)
go run . -manual
```

The service keeps running. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Redis via docker compose, runs the example and verifies config refresh, exit code 0 means pass.

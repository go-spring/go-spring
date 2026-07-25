# starter-lock-consul Example

Run this example to test Consul Session distributed lock with `starter-lock-consul`.

## Features

- **Lock Acquisition**: Acquire a distributed lock via Consul Session
- **Lock Contention**: Multiple instances compete for the same lock, only one succeeds
- **Lock Release**: Release the lock after the task completes

> Requires Consul service running. `check.sh` starts Consul via docker compose.

## Manual Testing

```bash
cd starter-lock-consul/example
go run . -manual
```

Expected output:
```
lock acquired
task completed
lock released
```

Start Consul first:
```bash
# Start Consul
docker compose up -d

# Run the example (manual mode, keeps running)
go run . -manual
```

Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Consul via docker compose, runs the example and verifies locking, exit code 0 means pass.

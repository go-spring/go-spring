# starter-lock-etcd Example

Demonstrates etcd distributed lock with starter-lock-etcd.

## Features

- **Lock Acquisition**: Acquire a distributed lock via etcd Lease
- **Lock Contention**: Multiple instances compete for the same lock, only one succeeds
- **Lock Release**: Release the lock after the task completes, or auto-release on TTL expiry

> Requires etcd service running. `check.sh` starts etcd via docker compose.

## Manual Testing

```bash
cd starter-lock-etcd/example
go run . -manual
```

Expected output:
```
lock acquired
task completed
lock released
```

Start etcd first:
```bash
# Start etcd
docker compose up -d

# Run the example (manual mode, keeps running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts etcd via docker compose, runs the example and verifies locking, exit code 0 means pass.
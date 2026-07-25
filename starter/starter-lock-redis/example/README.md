# starter-lock-redis Example

Demonstrates Redis distributed lock with starter-lock-redis.

## Features

- **TryAcquire**: Successfully acquires a lock on an available key
- **Lock Mutual Exclusion**: A held lock cannot be acquired by another instance
- **Lock Release**: Release the lock after the task completes

> Requires Redis service running. `check.sh` starts Redis via docker compose.

## Manual Testing

```bash
cd starter-lock-redis/example
go run . -manual
```

Expected output:
```
lock acquired on free key
lock rejected on held key
lock released
```

Start Redis first:
```bash
# Start Redis
docker compose up -d

# Run the example (manual mode, keeps running)
go run . -manual
```

The service keeps running. You can verify with corresponding CLI tools. Press `Ctrl+C` to stop.

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Redis via docker compose, runs the example and verifies locking, exit code 0 means pass.
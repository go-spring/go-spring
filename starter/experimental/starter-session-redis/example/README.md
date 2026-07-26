# starter-session-redis Example

Demonstrates Redis session management.

## Features

- **Cross-Replica Sharing**: Replica A writes session, Replica B reads (shared Redis storage)
- **Session Persistence**: Session data stored in Redis, survives process restarts
- **Cookie Passing**: Session ID passed via Cookie

> Requires a running Redis service. `check.sh` starts Redis via docker compose.

## Manual Testing

```bash
cd starter-session-redis/example
go run . -manual
```

Expected output:
```
session set: ok
session get: ok
cross-replica sharing: OK
```

Start Redis first:
```bash
# Start Redis
docker compose up -d

# Run the example
go run . -manual
```

Manual curl verification:
```bash
# Terminal 1: Start the service
go run . -manual

# Terminal 2: A writes session
curl -c /tmp/cookie http://127.0.0.1:9090/a/set?user=alice
# -> ok

# Terminal 2: B reads session (shared Redis)
curl -b /tmp/cookie http://127.0.0.1:9090/b/get
# -> alice
```

## Smoke Test

```bash
./check.sh
```

`check.sh` starts Redis via docker compose, runs the example and verifies sessions, exit code 0 means pass.
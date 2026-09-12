# starter-ratelimit-redis example

Two HTTP handlers (`/a/`, `/b/`) model two replicas of a service. They share
no in-process state; both draw from the same Redis token bucket
(burst 5, 2/s refill) registered under the driver name `redis`.

## Run

```bash
docker compose up -d     # or point spring.go-redis.instances.cache.addr at any redis
go run .
```

## Smoke test

```bash
./check.sh
```

Asserts that ten alternating requests split exactly into 5×200 + 5×429 (one
shared budget across "replicas"), then — after ~2s — that the refill grants
new tokens. Manual mode: `go run . -manual`, then
`curl http://127.0.0.1:9090/a/` and `curl http://127.0.0.1:9090/b/` and watch
them share the budget.

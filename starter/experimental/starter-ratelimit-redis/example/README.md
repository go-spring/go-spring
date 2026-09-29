# starter-ratelimit-redis example

Two HTTP handlers (`/a/`, `/b/`) model two replicas of a service. They share
no in-process state; both run under the service label `ratelimit-redis:api`, and
their budget (burst 5, 2/s) is spent from the one Redis-backed counter store the
starter contributed — one budget per scope for the whole process, and for every
replica configured the same way.

The budget itself is a governance rule (`conf/govern.yaml`), not starter config:
`spring.ratelimit.redis.client=cache` only says which Redis client the counters
count through.

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

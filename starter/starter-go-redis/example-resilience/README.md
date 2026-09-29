# starter-go-redis Resilience Example

Demonstrates starter-go-redis's **resilience protection**: with
`resilience.enabled`, every Redis command runs through the selected driver's
`ClientExecutor`, so a burst over the rate limit is rejected with `ErrRateLimited`
before reaching Redis.

## Features

- **Backend-neutral config**: the same `${resilience.*}` keys drive every client
  starter (rate-limit, burst, error-threshold, max-concurrent, ...).
- **Hot-reloadable**: the wrapper field-injects `Resilience` via `gs.Dync`, so
  a refresh swaps the executor live.
- **Driver pluggability**: `driver=default` (builtin, zero-dep) or `sentinel`.
- **Observability**: the executor is wrapped by `observe/resilience`, so breaker
  trips and rejects emit span + counter + histogram.

## Manual Testing

Requires a local Redis (`docker compose up -d`):

```bash
cd starter/starter-go-redis/example-resilience
docker compose up -d
go run . -manual
```

## Smoke Test

```bash
./check.sh
```

`check.sh` brings up Redis via docker compose, fires a burst of `Set` calls and
asserts a non-empty head is admitted and a non-empty tail is rejected with
`ErrRateLimited`; exit code 0 means pass. Skipped gracefully when docker is
unavailable.

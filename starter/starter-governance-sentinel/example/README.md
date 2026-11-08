# starter-governance-sentinel Example

Demonstrates the fused path: sentinel-golang is selected as the **rate-limit
algorithm** of the resilience engine.

## What it shows

- One `ratelimit.Factory` collects the built-in provider and the sentinel
  provider; a policy names the algorithm with `limiter: sentinel`.
- The call still runs through the resilience engine, so the rejection is the
  engine's neutral `ErrRateLimited` — not a sentinel error leaking out.
- A budget of 2/s: three immediate requests admit two and reject the third.

## Run

```bash
cd starter-governance-sentinel/example
go run .
```

Expected output:

```
sentinel limiter: 2 admitted, 1 rate-limited (budget 2/s)
resilience seams smoke: OK
```

## Smoke test

```bash
./check.sh
```

Exit code 0 means pass.

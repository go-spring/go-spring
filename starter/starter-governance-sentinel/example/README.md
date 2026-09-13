# starter-governance-sentinel Example

Demonstrates Sentinel circuit breaking and rate limiting.

## Features

- **Circuit breaking on the dialer seam**: against a dead address, the breaker opens after 3 refused dials and the 4th is short-circuited with `ErrCircuitOpen` before touching the network
- **Composed policy**: rate limit + circuit breaker + retry in one policy — a flaky upstream that 503s twice before succeeding is recovered within the retry budget
- **Sentinel driven**: both run on the `sentinel` driver built by `NewSentinelDriver()` (the container-free entry point)

## Manual Testing

```bash
cd starter-governance-sentinel/example
go run .
```

Expected output:
```
resilience seams smoke: OK
```

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
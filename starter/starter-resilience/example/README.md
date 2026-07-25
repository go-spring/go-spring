# starter-resilience Example

Demonstrates Sentinel circuit breaking and rate limiting.

## Features

- **Rate Limiting (Server Side)**: 5 QPS rate limit, excess requests return 429
- **Circuit Breaking (Client Side)**: Circuit breaker opens after 3 consecutive rejected dials
- **Sentinel Driven**: Resilience semantics implemented via Sentinel

## Manual Testing

```bash
cd starter-resilience/example
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
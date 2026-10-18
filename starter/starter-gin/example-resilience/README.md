# starter-gin Resilience Example

Demonstrates starter-gin's **inbound resilience admission**: with
`spring.gin.server.resilience.enabled`, every request runs through the selected
resilience driver's `Executor`, so a burst over the configured rate limit is
shed with HTTP 429 (circuit-open → 503) before the business handler runs.

## Features

- **Config-driven admission**: no middleware code — the starter applies the
  resilience admission middleware from `spring.gin.server.resilience.*`.
- **Rate limiting**: a burst over `rate-limit` is rejected with `429`.
- **Driver pluggability**: `driver=default` (builtin, zero-dep) or `sentinel`.

## Manual Testing

```bash
cd starter/starter-gin/example-resilience
go run . -manual
```

In another terminal, burst the route:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null -w "%{http_code}\n" http://127.0.0.1:8081/; done
```

You should see a mix of `200` and `429`.

## Smoke Test

```bash
./check.sh
```

`check.sh` fires a burst and asserts the server both serves (`200`) and sheds
(`429`); exit code 0 means pass.

# starter-pprof Example

[English](README.md) | [中文](README_CN.md)

Demonstrates pprof endpoint token authentication.

## Features

- **Token authentication**: Requests without a token return 401 Unauthorized
- **Bearer Token**: Requests with `Authorization: Bearer s3cr3t` access normally
- **pprof endpoints**: `/debug/pprof/`, `/debug/pprof/heap`, `/debug/pprof/cmdline`

## Manual Testing

Terminal 1, start the service:
```bash
cd starter-pprof/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
# No token -> 401
curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:9981/debug/pprof/
# -> 401

# With token -> 200
curl -s -o /dev/null -w '%{http_code}' -H 'Authorization: Bearer s3cr3t' \
  http://127.0.0.1:9981/debug/pprof/
# -> 200

curl -s -o /dev/null -w '%{http_code}' \
  'http://127.0.0.1:9981/debug/pprof/heap?token=s3cr3t'
# -> 200
```

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.

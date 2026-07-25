# starter-security-jwt Example

Demonstrates JWT authentication.

## Features

- **JWT Authentication**: Verifies request identity via JWT token
- **Missing Token Rejection**: Requests without a token return 401
- **Valid Token Allowance**: Requests carrying a valid token reach the business handler

## Manual Testing

Terminal 1, start the service:
```bash
cd starter-security-jwt/example
go run . -manual
```

Terminal 2, run verification commands:
```bash
# No Token -> 401
curl -i http://127.0.0.1:9090/me
# -> HTTP/1.1 401 Unauthorized

# With Token -> 200
curl -i -H 'Authorization: Bearer <token>' http://127.0.0.1:9090/me
# -> HTTP/1.1 200 OK
```

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.
# starter-oauth2-resource-server Example

Demonstrates an OAuth2 resource server protecting endpoints with JWT bearer
authentication. Verification uses a shared HMAC secret, so no external identity
provider is needed — the example mints its own tokens.

## Features

- **Token Authentication**: requests without a bearer token return 401
- **Invalid Token Rejection**: forged tokens (wrong secret) return 401
- **Subject Propagation**: `/me` echoes the authenticated subject
- **Authority Check**: `/orders` requires the `orders:read` scope, else 403

## Manual Testing

Terminal 1, start the service:

```bash
cd starter/experimental/starter-oauth2-resource-server/example
go run . -manual
```

Terminal 2, run verification commands:

```bash
# No token -> 401
curl -i http://127.0.0.1:9090/me

# With token -> 200
curl -i -H 'Authorization: Bearer <token>' http://127.0.0.1:9090/me
```

Press Ctrl+C to stop the service.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for the self-test to complete; exit code
0 means pass.

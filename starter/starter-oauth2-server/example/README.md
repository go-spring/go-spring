# starter-oauth2-server Example

Demonstrates OAuth2 authorization server for starter-oauth2-server.

## Features

- **Authorization endpoint**: `/oauth2/authorize` handles authorization requests
- **Token endpoint**: `/oauth2/token` issues access tokens
- **Token verification**: Resource server verifies tokens via HMAC
- **Security filters**: CORS + authentication + authorization unified filter chain

## Manual Testing

```bash
cd starter-oauth2-server/example
go run . -manual
```

The program keeps running, press Ctrl+C to exit. Without -manual, runTest() executes automatically and exits.

## Smoke Test

```bash
./check.sh
```

`check.sh` runs the example and waits for self-test to complete, exit code 0 means pass.